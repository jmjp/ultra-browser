package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"

	"golang.org/x/net/websocket"
)

// WebSocketAdapter implementa ports.BrowserPort usando WebSockets.
type WebSocketAdapter struct {
	conn      *websocket.Conn
	pending   map[string]chan domain.BridgeMessage
	mu        sync.RWMutex
	events    chan domain.BridgeMessage
	done      chan struct{}
	closeOnce sync.Once
	idCount   atomic.Uint64
}

var _ ports.BrowserPort = (*WebSocketAdapter)(nil)

// NewWebSocketAdapterFromConn cria um novo adaptador usando uma conexão WebSocket já estabelecida.
func NewWebSocketAdapterFromConn(conn *websocket.Conn) *WebSocketAdapter {
	a := &WebSocketAdapter{
		conn:    conn,
		pending: make(map[string]chan domain.BridgeMessage),
		events:  make(chan domain.BridgeMessage, 100),
		done:    make(chan struct{}),
	}

	go a.readLoop()

	return a
}

// NewWebSocketAdapter cria uma nova conexão com o browser via WebSocket (dialing).
func NewWebSocketAdapter(url string) (*WebSocketAdapter, error) {
	config, err := websocket.NewConfig(url, "http://localhost/")
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket config: %w", err)
	}

	conn, err := websocket.DialConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial websocket: %w", err)
	}

	return NewWebSocketAdapterFromConn(conn), nil
}

func (a *WebSocketAdapter) readLoop() {
	defer a.Close()
	for {
		var msg domain.BridgeMessage
		err := websocket.JSON.Receive(a.conn, &msg)
		if err != nil {
			slog.Debug("WebSocket read loop stopped", "error", err)
			return
		}

		if msg.ID != "" {
			a.mu.RLock()
			ch, ok := a.pending[msg.ID]
			a.mu.RUnlock()

			if ok {
				select {
				case ch <- msg:
				default:
					// Canal já cheio ou fechado
				}
				continue
			}
		}

		// Se não tiver ID ou não for uma resposta pendente, é um evento espontâneo
		select {
		case a.events <- msg:
		default:
			slog.Warn("Event buffer full, dropping message", "id", msg.ID, "tool", msg.Tool)
		}
	}
}

// Close fecha a conexão com o WebSocket.
func (a *WebSocketAdapter) Close() error {
	var err error
	a.closeOnce.Do(func() {
		err = a.conn.Close()
		close(a.events)
		close(a.done)

		a.mu.Lock()
		for id, ch := range a.pending {
			close(ch)
			delete(a.pending, id)
		}
		a.mu.Unlock()
	})
	return err
}

// Done retorna um canal que é fechado quando o adaptador é encerrado.
func (a *WebSocketAdapter) Done() <-chan struct{} {
	return a.done
}

func (a *WebSocketAdapter) ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	if msg.ID == "" {
		return domain.BridgeMessage{}, errors.New("message ID is required for ExecuteCommand")
	}

	resCh := make(chan domain.BridgeMessage, 1)

	a.mu.Lock()
	a.pending[msg.ID] = resCh
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.pending, msg.ID)
		a.mu.Unlock()
	}()

	err := websocket.JSON.Send(a.conn, msg)
	if err != nil {
		return domain.BridgeMessage{}, fmt.Errorf("failed to send command: %w", err)
	}

	select {
	case res := <-resCh:
		if res.Error != "" {
			return res, errors.New(res.Error)
		}
		return res, nil
	case <-ctx.Done():
		return domain.BridgeMessage{}, ctx.Err()
	}
}

func (a *WebSocketAdapter) ReadEvents(ctx context.Context) (<-chan domain.BridgeMessage, error) {
	return a.events, nil
}

// executeWS é um helper para os métodos da interface ports.BrowserPort.
func executeWS[T any](ctx context.Context, a *WebSocketAdapter, prefix, tool string, req any) (T, error) {
	var zero T
	id := fmt.Sprintf("%s-%d", prefix, a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, tool, req)
	if err != nil {
		return zero, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.ExecuteCommand(ctx, msg)
	if err != nil {
		return zero, err
	}

	var result T
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return zero, fmt.Errorf("failed to unmarshal %s response: %w", tool, err)
	}
	return result, nil
}

// CaptureNode solicita que o navegador capture um elemento específico do DOM.
func (a *WebSocketAdapter) CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
	res, err := executeWS[domain.CaptureNodeResponse](ctx, a, "wcn", "capture_node", req)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}
	if res.Content != "" {
		res.Success = true
	}
	return res, nil
}

// TypeText simula a digitação de texto em um elemento do navegador.
func (a *WebSocketAdapter) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	return executeWS[domain.CommonResponse](ctx, a, "wtt", "type_text", req)
}

// WaitForElement aguarda até que um elemento apareça no DOM ou ocorra timeout.
func (a *WebSocketAdapter) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	return executeWS[domain.CommonResponse](ctx, a, "wwf", "wait_for_element", req)
}

// GetValue recupera o valor atual (value) de um elemento.
func (a *WebSocketAdapter) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	return executeWS[domain.GetValueResponse](ctx, a, "wgv", "get_value", req)
}

// SelectOption seleciona uma opção em um elemento <select>.
func (a *WebSocketAdapter) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	return executeWS[domain.CommonResponse](ctx, a, "wso", "select_option", req)
}

// UploadFile realiza o upload de um arquivo enviando o conteúdo em Base64.
func (a *WebSocketAdapter) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	params := map[string]any{
		"selector": req.Selector,
		"path":     req.Path,
		"content":  base64.StdEncoding.EncodeToString(content),
	}
	return executeWS[domain.CommonResponse](ctx, a, "wuf", "upload_file", params)
}

// Scroll rola a página ou um elemento específico.
func (a *WebSocketAdapter) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	return executeWS[domain.CommonResponse](ctx, a, "wsc", "scroll", req)
}

// Hover simula o mouse hover em um elemento.
func (a *WebSocketAdapter) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	return executeWS[domain.CommonResponse](ctx, a, "whv", "hover", req)
}

// SwitchTab muda para a aba especificada.
func (a *WebSocketAdapter) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	return executeWS[domain.CommonResponse](ctx, a, "wst", "switch_tab", req)
}

// Screenshot captura um screenshot PNG da aba ativa.
func (a *WebSocketAdapter) Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error) {
	id := fmt.Sprintf("wss-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "screenshot", nil)
	if err != nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}
	if resp.Error != "" {
		return domain.CaptureNodeResponse{Success: false, Message: resp.Error}, nil
	}

	// Tenta desempacotar como CaptureNodeResponse
	var result domain.CaptureNodeResponse
	if err := json.Unmarshal(resp.Result, &result); err == nil && result.Content != "" {
		return result, nil
	}

	// Fallback: formato direto do navegador { "base64": "..." }
	var data struct {
		Base64 string `json:"base64"`
	}
	if err := json.Unmarshal(resp.Result, &data); err == nil && data.Base64 != "" {
		return domain.CaptureNodeResponse{Success: true, Content: data.Base64, Format: "png"}, nil
	}

	slog.Error("Failed to decode screenshot response", "result", string(resp.Result))
	return domain.CaptureNodeResponse{}, fmt.Errorf("websocket: failed to decode screenshot response")
}
