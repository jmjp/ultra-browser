package bridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// HTTPProxyAdapter implementa ports.BrowserPort encaminhando comandos
// via HTTP para o servidor daemon MCP que está rodando em background.
// Usado quando o binário é lançado pelo Chrome via Native Messaging.
type HTTPProxyAdapter struct {
	serverURL string
	client    *http.Client
}

// Assegura que HTTPProxyAdapter implementa ports.BrowserPort.
var _ ports.BrowserPort = (*HTTPProxyAdapter)(nil)

// NewHTTPProxyAdapter cria um adaptador que encaminha comandos para o daemon HTTP.
func NewHTTPProxyAdapter(serverURL string) *HTTPProxyAdapter {
	return &HTTPProxyAdapter{
		serverURL: serverURL,
		client: &http.Client{
			Timeout: 35 * time.Second,
		},
	}
}

// ExecuteCommand serializa o comando como JSON-RPC e o envia ao daemon MCP via HTTP.
func (a *HTTPProxyAdapter) ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	// Converte BridgeMessage em uma requisição JSON-RPC para o endpoint /tool-proxy
	payload, err := json.Marshal(msg)
	if err != nil {
		return domain.BridgeMessage{}, fmt.Errorf("http_proxy: falha ao serializar requisição: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.serverURL+"/tool-proxy", bytes.NewReader(payload))
	if err != nil {
		return domain.BridgeMessage{}, fmt.Errorf("http_proxy: falha ao criar requisição HTTP: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return domain.BridgeMessage{}, fmt.Errorf("http_proxy: erro ao comunicar com daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.BridgeMessage{}, fmt.Errorf("http_proxy: daemon retornou status %d", resp.StatusCode)
	}

	var result domain.BridgeMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return domain.BridgeMessage{}, fmt.Errorf("http_proxy: falha ao decodificar resposta do daemon: %w", err)
	}

	return result, nil
}

// ReadEvents não é suportado no modo proxy — retorna canal vazio.
func (a *HTTPProxyAdapter) ReadEvents(_ context.Context) (<-chan domain.BridgeMessage, error) {
	ch := make(chan domain.BridgeMessage)
	return ch, nil
}

// CaptureNode solicita que o navegador capture um elemento específico do DOM.
func (a *HTTPProxyAdapter) CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
	msg, err := domain.NewBridgeRequest("cn-proxy", "capture_node", req)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}
	resp, err := a.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}
	if resp.Error != "" {
		return domain.CaptureNodeResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CaptureNodeResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("http_proxy: falha ao decodificar capture_node: %w", err)
	}
	return result, nil
}

// proxyCommon é um helper que encaminha um comando e desserializa a resposta em CommonResponse.
func (a *HTTPProxyAdapter) proxyCommon(ctx context.Context, tool string, req any) (domain.CommonResponse, error) {
	msg, err := domain.NewBridgeRequest(tool+"-proxy", tool, req)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	resp, err := a.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}
	if resp.Error != "" {
		return domain.CommonResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, fmt.Errorf("http_proxy: falha ao decodificar resposta de %s: %w", tool, err)
	}
	return result, nil
}

// TypeText simula a digitação de texto em um elemento do navegador.
func (a *HTTPProxyAdapter) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	return a.proxyCommon(ctx, "type_text", req)
}

// WaitForElement aguarda até que um elemento apareça no DOM ou ocorra timeout.
func (a *HTTPProxyAdapter) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	return a.proxyCommon(ctx, "wait_for_element", req)
}

// GetValue recupera o valor atual (value) de um elemento.
func (a *HTTPProxyAdapter) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	msg, err := domain.NewBridgeRequest("gv-proxy", "get_value", req)
	if err != nil {
		return domain.GetValueResponse{}, err
	}
	resp, err := a.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.GetValueResponse{}, err
	}
	if resp.Error != "" {
		return domain.GetValueResponse{Success: false, Message: resp.Error}, nil
	}
	var result domain.GetValueResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.GetValueResponse{}, fmt.Errorf("http_proxy: falha ao decodificar get_value: %w", err)
	}
	return result, nil
}

// SelectOption seleciona uma opção em um elemento <select>.
func (a *HTTPProxyAdapter) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	return a.proxyCommon(ctx, "select_option", req)
}

// UploadFile realiza o upload de um arquivo enviando o conteúdo em Base64 ao daemon.
func (a *HTTPProxyAdapter) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	params := map[string]any{
		"selector": req.Selector,
		"base64":   base64.StdEncoding.EncodeToString(content),
		"filename": req.Path,
	}
	return a.proxyCommon(ctx, "upload_file", params)
}

// Scroll rola a página ou um elemento específico.
func (a *HTTPProxyAdapter) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	return a.proxyCommon(ctx, "scroll", req)
}

// Hover simula o mouse hover em um elemento.
func (a *HTTPProxyAdapter) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	return a.proxyCommon(ctx, "hover", req)
}

// SwitchTab muda para a aba especificada.
func (a *HTTPProxyAdapter) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	return a.proxyCommon(ctx, "switch_tab", req)
}

// Screenshot captura um screenshot PNG da aba ativa.
func (a *HTTPProxyAdapter) Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error) {
	msg, err := domain.NewBridgeRequest("ss-proxy", "screenshot", nil)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
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

	return domain.CaptureNodeResponse{}, fmt.Errorf("http_proxy: falha ao decodificar resposta de screenshot")
}
