package bridge

import (
	"context"
	"fmt"
	"sync"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// BridgeHub gerencia a conexão ativa da bridge Native Messaging.
// O servidor daemon usa este hub como seu BrowserPort. Quando um processo bridge
// (lançado pelo Chrome) encaminha um comando via /tool-proxy, o hub o despacha
// para a bridge nativa ativa (se houver).
type BridgeHub struct {
	mu     sync.RWMutex
	active ports.BrowserPort
}

// Assegura que BridgeHub implementa ports.BrowserPort.
var _ ports.BrowserPort = (*BridgeHub)(nil)

// NewBridgeHub cria um novo hub sem bridge ativa.
func NewBridgeHub() *BridgeHub {
	return &BridgeHub{}
}

// Register marca uma instância BrowserPort como a bridge ativa.
func (h *BridgeHub) Register(a ports.BrowserPort) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = a
}

// Unregister remove a bridge ativa.
func (h *BridgeHub) Unregister() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = nil
}

// ExecuteCommand encaminha o comando à bridge ativa.
// Retorna erro se nenhuma bridge estiver conectada.
func (h *BridgeHub) ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.BridgeMessage{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa; verifique se a extensão Chrome está conectada")
	}

	return a.ExecuteCommand(ctx, msg)
}

// ReadEvents retorna o canal de eventos da bridge ativa, ou um canal vazio.
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

// CaptureNode encaminha a solicitação de captura para a bridge ativa.
func (h *BridgeHub) CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.CaptureNode(ctx, req)
}

// TypeText encaminha a solicitação de digitação para a bridge ativa.
func (h *BridgeHub) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.TypeText(ctx, req)
}

// WaitForElement encaminha a solicitação de espera para a bridge ativa.
func (h *BridgeHub) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.WaitForElement(ctx, req)
}

// GetValue encaminha a solicitação de valor para a bridge ativa.
func (h *BridgeHub) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.GetValueResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.GetValue(ctx, req)
}

// SelectOption encaminha a solicitação de seleção para a bridge ativa.
func (h *BridgeHub) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.SelectOption(ctx, req)
}

// UploadFile encaminha a solicitação de upload para a bridge ativa.
func (h *BridgeHub) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.UploadFile(ctx, req, content)
}

// Scroll encaminha a solicitação de rolagem para a bridge ativa.
func (h *BridgeHub) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.Scroll(ctx, req)
}

// Hover encaminha a solicitação de hover para a bridge ativa.
func (h *BridgeHub) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.Hover(ctx, req)
}

// SwitchTab encaminha a solicitação de mudança de aba para a bridge ativa.
func (h *BridgeHub) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CommonResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.SwitchTab(ctx, req)
}

// Screenshot captura um screenshot PNG da aba ativa.
func (h *BridgeHub) Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error) {
	h.mu.RLock()
	a := h.active
	h.mu.RUnlock()

	if a == nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("bridge: nenhuma conexão Native Messaging ativa")
	}

	return a.Screenshot(ctx)
}
