package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// RemoteBrowserPort implementa ports.BrowserPort atuando como servidor que
// aguarda uma conexão de uma bridge remota (processo Native Messaging).
type RemoteBrowserPort struct {
	commands chan domain.BridgeMessage
	events   chan domain.BridgeMessage
	pending  map[string]chan domain.BridgeMessage
	mu       sync.Mutex
	idCount  atomic.Uint64
}

var _ ports.BrowserPort = (*RemoteBrowserPort)(nil)

// NewRemoteBrowserPort cria uma nova instância da porta remota.
func NewRemoteBrowserPort() *RemoteBrowserPort {
	return &RemoteBrowserPort{
		commands: make(chan domain.BridgeMessage, 100),
		events:   make(chan domain.BridgeMessage, 100),
		pending:  make(map[string]chan domain.BridgeMessage),
	}
}

// ExecuteCommand envia um comando para a bridge remota e aguarda a resposta correlacionada.
func (p *RemoteBrowserPort) ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	ch := make(chan domain.BridgeMessage, 1)

	p.mu.Lock()
	p.pending[msg.ID] = ch
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.pending, msg.ID)
		p.mu.Unlock()
	}()

	// Envia o comando para a bridge remota via SSE
	select {
	case p.commands <- msg:
	case <-ctx.Done():
		return domain.BridgeMessage{}, ctx.Err()
	}

	// Aguarda a resposta correlacionada pelo ID
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return domain.BridgeMessage{}, ctx.Err()
	}
}

// ReadEvents retorna o canal de eventos não solicitados vindos da bridge remota.
func (p *RemoteBrowserPort) ReadEvents(ctx context.Context) (<-chan domain.BridgeMessage, error) {
	return p.events, nil
}

// executeRemote é o helper genérico que elimina o boilerplate de cada método:
// gera ID, cria BridgeRequest, executa e faz unmarshal tipado.
func executeRemote[T any](ctx context.Context, p *RemoteBrowserPort, prefix, tool string, req any) (T, error) {
	var zero T
	id := fmt.Sprintf("%s-%d", prefix, p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, tool, req)
	if err != nil {
		return zero, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return zero, err
	}
	if resp.Error != "" {
		return zero, errors.New(resp.Error)
	}

	var result T
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return zero, fmt.Errorf("failed to unmarshal %s response: %w", tool, err)
	}
	return result, nil
}

// CaptureNode solicita que o navegador capture um elemento específico do DOM.
func (p *RemoteBrowserPort) CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
	res, err := executeRemote[domain.CaptureNodeResponse](ctx, p, "rcn", "capture_node", req)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}
	if res.Content != "" {
		res.Success = true
	}
	return res, nil
}

// TypeText simula a digitação de texto em um elemento do navegador.
func (p *RemoteBrowserPort) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	return executeRemote[domain.CommonResponse](ctx, p, "rtt", "type_text", req)
}

// WaitForElement aguarda até que um elemento apareça no DOM ou ocorra timeout.
func (p *RemoteBrowserPort) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	return executeRemote[domain.CommonResponse](ctx, p, "rwf", "wait_for_element", req)
}

// GetValue recupera o valor atual (value) de um elemento.
func (p *RemoteBrowserPort) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	return executeRemote[domain.GetValueResponse](ctx, p, "rgv", "get_value", req)
}

// GetContent recupera o conteúdo de texto de um elemento.
func (p *RemoteBrowserPort) GetContent(ctx context.Context, req domain.GetContentRequest) (domain.GetContentResponse, error) {
	res, err := executeRemote[domain.GetContentResponse](ctx, p, "rgc", "get_content", req)
	if err != nil {
		return domain.GetContentResponse{}, err
	}
	if res.Content != "" {
		res.Success = true
	}
	return res, nil
}

// SelectOption seleciona uma opção em um elemento <select>.
func (p *RemoteBrowserPort) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	return executeRemote[domain.CommonResponse](ctx, p, "rso", "select_option", req)
}

// UploadFile realiza o upload de um arquivo enviando o conteúdo em Base64.
func (p *RemoteBrowserPort) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	params := map[string]any{
		"selector": req.Selector,
		"path":     req.Path,
		"content":  base64.StdEncoding.EncodeToString(content),
	}
	return executeRemote[domain.CommonResponse](ctx, p, "ruf", "upload_file", params)
}

// Scroll rola a página ou um elemento específico.
func (p *RemoteBrowserPort) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	return executeRemote[domain.CommonResponse](ctx, p, "rsc", "scroll", req)
}

// Hover simula o mouse hover em um elemento.
func (p *RemoteBrowserPort) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	return executeRemote[domain.CommonResponse](ctx, p, "rhv", "hover", req)
}

// SwitchTab muda para a aba especificada.
func (p *RemoteBrowserPort) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	return executeRemote[domain.CommonResponse](ctx, p, "rst", "switch_tab", req)
}

// Screenshot captura um screenshot PNG da aba ativa.
func (p *RemoteBrowserPort) Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error) {
	id := fmt.Sprintf("rss-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "screenshot", nil)
	if err != nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := p.ExecuteCommand(ctx, msg)
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

	return domain.CaptureNodeResponse{}, fmt.Errorf("remote: failed to decode screenshot response")
}

// GetCommands retorna o canal de comandos para serem enviados à bridge remota via SSE.
func (p *RemoteBrowserPort) GetCommands() <-chan domain.BridgeMessage {
	return p.commands
}

// HandleResponse processa uma resposta vinda da bridge remota e a correlaciona pelo ID.
func (p *RemoteBrowserPort) HandleResponse(msg domain.BridgeMessage) {
	p.mu.Lock()
	ch, ok := p.pending[msg.ID]
	p.mu.Unlock()

	if ok {
		select {
		case ch <- msg:
		default:
			// Canal já foi fechado ou preenchido (timeout ou cancelamento)
		}
	}
}

// HandleEvent processa um evento assíncrono vindo da bridge remota.
func (p *RemoteBrowserPort) HandleEvent(msg domain.BridgeMessage) {
	select {
	case p.events <- msg:
	default:
		// Canal cheio: descarta evento para não bloquear o handler HTTP
	}
}
