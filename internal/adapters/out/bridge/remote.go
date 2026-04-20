package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// RemoteBrowserPort implementa ports.BrowserPort atuando como um servidor que
// aguarda uma conexão de uma bridge remota (processo Native Messaging).
type RemoteBrowserPort struct {
	commands  chan domain.BridgeMessage
	events    chan domain.BridgeMessage
	pending   map[string]chan domain.BridgeMessage
	mu        sync.Mutex
	onConnect func()
	idCount   atomic.Uint64
}

var _ ports.BrowserPort = (*RemoteBrowserPort)(nil)

func NewRemoteBrowserPort() *RemoteBrowserPort {
	return &RemoteBrowserPort{
		commands: make(chan domain.BridgeMessage, 100),
		events:   make(chan domain.BridgeMessage, 100),
		pending:  make(map[string]chan domain.BridgeMessage),
	}
}

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

	select {
	case p.commands <- msg:
	case <-ctx.Done():
		return domain.BridgeMessage{}, ctx.Err()
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return domain.BridgeMessage{}, ctx.Err()
	}
}

func (p *RemoteBrowserPort) ReadEvents(ctx context.Context) (<-chan domain.BridgeMessage, error) {
	return p.events, nil
}

// CaptureNode solicita que o navegador capture um elemento específico do DOM.
func (p *RemoteBrowserPort) CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
	id := fmt.Sprintf("rcn-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "capture_node", req)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}

	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}

	if resp.Error != "" {
		return domain.CaptureNodeResponse{Success: false, Message: resp.Error}, nil
	}

	var result domain.CaptureNodeResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CaptureNodeResponse{}, err
	}
	// Se não houve erro e temos conteúdo, garantimos que Success seja true
	if result.Content != "" {
		result.Success = true
	}
	return result, nil
}

// TypeText simula a digitação de texto em um elemento do navegador.
func (p *RemoteBrowserPort) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("rtt-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "type_text", req)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	if resp.Error != "" {
		return domain.CommonResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, err
	}
	return result, nil
}

// WaitForElement aguarda até que um elemento apareça no DOM ou ocorra timeout.
func (p *RemoteBrowserPort) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("rwf-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "wait_for_element", req)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	if resp.Error != "" {
		return domain.CommonResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, err
	}
	return result, nil
}

// GetValue recupera o valor atual (value) de um elemento.
func (p *RemoteBrowserPort) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	id := fmt.Sprintf("rgv-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "get_value", req)
	if err != nil {
		return domain.GetValueResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.GetValueResponse{}, err
	}
	if resp.Error != "" {
		return domain.GetValueResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.GetValueResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.GetValueResponse{}, err
	}
	return result, nil
}

// GetContent recupera o conteúdo de texto de um elemento.
func (p *RemoteBrowserPort) GetContent(ctx context.Context, req domain.GetContentRequest) (domain.GetContentResponse, error) {
	id := fmt.Sprintf("rgc-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "get_content", req)
	if err != nil {
		return domain.GetContentResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.GetContentResponse{}, err
	}
	if resp.Error != "" {
		return domain.GetContentResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.GetContentResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.GetContentResponse{}, err
	}
	// Garante Success true se temos conteúdo
	if result.Content != "" {
		result.Success = true
	}
	return result, nil
}

// SelectOption seleciona uma opção em um elemento <select>.
func (p *RemoteBrowserPort) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("rso-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "select_option", req)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	if resp.Error != "" {
		return domain.CommonResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, err
	}
	return result, nil
}

// UploadFile realiza o upload de um arquivo.
func (p *RemoteBrowserPort) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	id := fmt.Sprintf("ruf-%d", p.idCount.Add(1))
	params := map[string]any{
		"selector": req.Selector,
		"path":     req.Path,
		"content":  base64.StdEncoding.EncodeToString(content),
	}
	msg, err := domain.NewBridgeRequest(id, "upload_file", params)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	if resp.Error != "" {
		return domain.CommonResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, err
	}
	return result, nil
}

// Scroll rola a página ou um elemento específico.
func (p *RemoteBrowserPort) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("rsc-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "scroll", req)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	if resp.Error != "" {
		return domain.CommonResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, err
	}
	return result, nil
}

// Hover simula o mouse hover em um elemento.
func (p *RemoteBrowserPort) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("rhv-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "hover", req)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	if resp.Error != "" {
		return domain.CommonResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, err
	}
	return result, nil
}

// SwitchTab muda para a aba especificada.
func (p *RemoteBrowserPort) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("rst-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "switch_tab", req)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	resp, err := p.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	if resp.Error != "" {
		return domain.CommonResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, err
	}
	return result, nil
}

// Screenshot captura um screenshot PNG da aba ativa.
func (p *RemoteBrowserPort) Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error) {
	id := fmt.Sprintf("rss-%d", p.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "screenshot", nil)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
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

	// Fallback para o formato direto do navegador {base64: ...}
	var data struct {
		Base64 string `json:"base64"`
	}
	if err := json.Unmarshal(resp.Result, &data); err == nil && data.Base64 != "" {
		return domain.CaptureNodeResponse{
			Success: true,
			Content: data.Base64,
			Format:  "png",
		}, nil
	}

	return domain.CaptureNodeResponse{}, fmt.Errorf("remote: falha ao decodificar resposta de screenshot")
}

// GetCommands retorna o canal de comandos para serem enviados à bridge remota.
func (p *RemoteBrowserPort) GetCommands() <-chan domain.BridgeMessage {
	return p.commands
}

// HandleResponse processa uma resposta vinda da bridge remota.
func (p *RemoteBrowserPort) HandleResponse(msg domain.BridgeMessage) {
	p.mu.Lock()
	ch, ok := p.pending[msg.ID]
	p.mu.Unlock()

	if ok {
		select {
		case ch <- msg:
		default:
		}
	}
}

// HandleEvent processa um evento vindo da bridge remota.
func (p *RemoteBrowserPort) HandleEvent(msg domain.BridgeMessage) {
	select {
	case p.events <- msg:
	default:
	}
}
