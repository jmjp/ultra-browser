package bridge

import (
	"context"
	"fmt"
	"sync"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// BridgeHub gerencia o adaptador ativo para comunicação com o navegador.
// O servidor MCP usa este hub como seu BrowserPort, permitindo que diferentes
// implementações de transporte (SSE, WebSocket, etc.) sejam registradas de forma transparente.
type BridgeHub struct {
	mu     sync.RWMutex
	active ports.BrowserPort
}

// Assegura que BridgeHub implementa ports.BrowserPort.
var _ ports.BrowserPort = (*BridgeHub)(nil)

// NewBridgeHub cria um novo hub sem adaptador ativo.
func NewBridgeHub() *BridgeHub {
	return &BridgeHub{}
}

// Register marca uma instância BrowserPort como a conexão ativa.
// Se já houver um adaptador registrado, ele será fechado antes da substituição.
func (h *BridgeHub) Register(a ports.BrowserPort) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.active != nil && h.active != a {
		_ = h.active.Close()
	}
	h.active = a
}

// Unregister remove a conexão ativa se for a mesma passada como argumento.
func (h *BridgeHub) UnregisterSpecific(a ports.BrowserPort) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.active == a {
		h.active = nil
	}
}

// Unregister fecha e remove a conexão ativa incondicionalmente.
func (h *BridgeHub) Unregister() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.active != nil {
		_ = h.active.Close()
		h.active = nil
	}
}

// Close fecha a conexão ativa e limpa o hub.
func (h *BridgeHub) Close() error {
	h.Unregister()
	return nil
}

// ExecuteCommand encaminha o comando ao adaptador ativo.
// Retorna erro se não houver conexão estabelecida.
func (h *BridgeHub) ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.BridgeMessage{}, fmt.Errorf("browser: nenhuma conexão ativa com o navegador; aguardando conexão da extensão")
	}

	return a.ExecuteCommand(ctx, msg)
}

// ReadEvents retorna o canal de eventos do adaptador ativo, ou um canal vazio se inativo.
func (h *BridgeHub) ReadEvents(ctx context.Context) (<-chan domain.BridgeMessage, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		ch := make(chan domain.BridgeMessage)
		return ch, nil
	}

	return a.ReadEvents(ctx)
}

// CaptureNode encaminha a solicitação de captura para o adaptador ativo.
func (h *BridgeHub) CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.CaptureNode(ctx, req)
}

// TypeText encaminha a solicitação de digitação para o adaptador ativo.
func (h *BridgeHub) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.TypeText(ctx, req)
}

// WaitForElement encaminha a solicitação de espera para o adaptador ativo.
func (h *BridgeHub) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.WaitForElement(ctx, req)
}

// GetValue encaminha a solicitação de valor para o adaptador ativo.
func (h *BridgeHub) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.GetValueResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.GetValue(ctx, req)
}

// SelectOption encaminha a solicitação de seleção para o adaptador ativo.
func (h *BridgeHub) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.SelectOption(ctx, req)
}

// UploadFile encaminha a solicitação de upload para o adaptador ativo.
func (h *BridgeHub) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.UploadFile(ctx, req, content)
}

// Scroll encaminha a solicitação de rolagem para o adaptador ativo.
func (h *BridgeHub) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.Scroll(ctx, req)
}

// Hover encaminha a solicitação de hover para o adaptador ativo.
func (h *BridgeHub) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.Hover(ctx, req)
}

// SwitchTab encaminha a solicitação de mudança de aba para o adaptador ativo.
func (h *BridgeHub) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.SwitchTab(ctx, req)
}

// Screenshot captura um screenshot PNG da aba ativa via adaptador.
func (h *BridgeHub) Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("browser: nenhuma conexão ativa")
	}

	return a.Screenshot(ctx)
}
