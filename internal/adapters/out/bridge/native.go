package bridge

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"sync"
	"ultra-browser/internal/core/domain"
)

// Reader abstracts the Chrome Native Messaging input protocol.
// It expects a 4-byte little-endian length prefix followed by a JSON payload.
type Reader struct {
	source    io.Reader
	events    chan domain.BridgeMessage
	mu        sync.Mutex
	onMessage func(domain.BridgeMessage) bool
}

// NewReader creates a new Reader from an io.Reader (usually os.Stdin).
func NewReader(r io.Reader) *Reader {
	return &Reader{
		source: r,
		events: make(chan domain.BridgeMessage, 100),
	}
}

// SetMessageHandler define um callback para processar mensagens antes de serem enviadas ao canal de eventos.
// Se o callback retornar true, a mensagem é considerada processada e não é enviada para ReadEvents.
func (r *Reader) SetMessageHandler(h func(domain.BridgeMessage) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onMessage = h
}

// ReadMessage reads a single message from the source.
// Returns the raw JSON bytes or an error.
func (r *Reader) ReadMessage() ([]byte, error) {
	var length uint32
	if err := binary.Read(r.source, binary.LittleEndian, &length); err != nil {
		return nil, err
	}

	if length == 0 {
		return []byte{}, nil
	}

	// Chrome Native Messaging has a limit of 1MB for messages.
	// We use a slightly larger buffer for safety, but enforce a reasonable limit.
	const maxMessageSize = 10 * 1024 * 1024 // 10MB
	if length > maxMessageSize {
		return nil, io.ErrUnexpectedEOF
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r.source, buf); err != nil {
		return nil, err
	}

	return buf, nil
}

// ReadEvents fornece um canal para leitura de eventos não solicitados enviados pelo navegador.
// Implementa a parte de leitura da interface BrowserPort.
func (r *Reader) ReadEvents(ctx context.Context) (<-chan domain.BridgeMessage, error) {
	return r.events, nil
}

// ReadLoop processa continuamente as mensagens vindas do navegador.
// O loop termina se o contexto for cancelado ou se ocorrer um erro de leitura (ex: EOF).
func (r *Reader) ReadLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			data, err := r.ReadMessage()
			if err != nil {
				if err == io.EOF || err == io.ErrClosedPipe {
					return nil
				}
				return err
			}

			if len(data) == 0 {
				continue
			}

			var msg domain.BridgeMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				// Falha no parse de uma mensagem individual não deve interromper o loop.
				// Em uma implementação real, usaríamos slog para registrar o erro.
				continue
			}

			// Se houver um handler registrado e ele processar a mensagem (ex: resposta correlacionada),
			// não enviamos para o canal de eventos gerais.
			r.mu.Lock()
			handler := r.onMessage
			r.mu.Unlock()

			if handler != nil && handler(msg) {
				continue
			}

			select {
			case r.events <- msg:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// Writer abstracts the Chrome Native Messaging output protocol.
// It sends a 4-byte little-endian length prefix followed by a JSON payload.
type Writer struct {
	sink io.Writer
	mu   sync.Mutex

	// Para correlação de mensagens (RequestId -> Response Channel)
	pending map[string]chan domain.BridgeMessage
	pmu     sync.Mutex
}

// NewWriter creates a new Writer targeting an io.Writer (usually os.Stdout).
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		sink:    w,
		pending: make(map[string]chan domain.BridgeMessage),
	}
}

// WriteMessage serializes and writes a single message to the sink with framing.
func (w *Writer) WriteMessage(msg domain.BridgeMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Escreve o prefixo de 4 bytes (Little Endian)
	length := uint32(len(data))
	if err := binary.Write(w.sink, binary.LittleEndian, length); err != nil {
		return err
	}

	// Escreve o payload JSON
	_, err = w.sink.Write(data)
	return err
}

// HandleResponse correlates a message from the reader to a pending request.
// Returns true if the message was handled (found in pending map).
func (w *Writer) HandleResponse(msg domain.BridgeMessage) bool {
	w.pmu.Lock()
	ch, ok := w.pending[msg.ID]
	if ok {
		delete(w.pending, msg.ID)
	}
	w.pmu.Unlock()

	if ok {
		ch <- msg
		close(ch)
	}
	return ok
}

// Call sends a request and blocks until a response is received or context is canceled.
func (w *Writer) Call(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	ch := make(chan domain.BridgeMessage, 1)

	w.pmu.Lock()
	w.pending[msg.ID] = ch
	w.pmu.Unlock()

	if err := w.WriteMessage(msg); err != nil {
		w.pmu.Lock()
		delete(w.pending, msg.ID)
		w.pmu.Unlock()
		return domain.BridgeMessage{}, err
	}

	select {
	case res := <-ch:
		return res, nil
	case <-ctx.Done():
		w.pmu.Lock()
		delete(w.pending, msg.ID)
		w.pmu.Unlock()
		return domain.BridgeMessage{}, ctx.Err()
	}
}

// ExecuteCommand envia uma mensagem de comando para a ponte do navegador
// e retorna a resposta correspondente. Implementa parte da interface BrowserPort.
func (w *Writer) ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	return w.Call(ctx, msg)
}
