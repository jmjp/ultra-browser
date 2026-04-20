package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// NativeMessagingAdapter implementa a interface ports.BrowserPort.
// Ele une o Reader e o Writer para fornecer uma comunicação bidirecional completa com o Chrome.
type NativeMessagingAdapter struct {
	reader  *Reader
	writer  *Writer
	idCount atomic.Uint64
}

// Assegura que NativeMessagingAdapter implementa ports.BrowserPort.
var _ ports.BrowserPort = (*NativeMessagingAdapter)(nil)

// NewNativeMessagingAdapter cria uma nova instância do adaptador.
func NewNativeMessagingAdapter(r *Reader, w *Writer) *NativeMessagingAdapter {
	a := &NativeMessagingAdapter{
		reader: r,
		writer: w,
	}
	// Configura o correlacionador de mensagens no leitor.
	// Isso permite que respostas a comandos sejam interceptadas antes de irem para o canal de eventos gerais.
	r.SetMessageHandler(w.HandleResponse)
	return a
}

// ExecuteCommand envia um comando para o navegador e aguarda a resposta correlacionada.
func (a *NativeMessagingAdapter) ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	return a.writer.ExecuteCommand(ctx, msg)
}

// ReadEvents retorna o canal de eventos não solicitados vindos do navegador.
func (a *NativeMessagingAdapter) ReadEvents(ctx context.Context) (<-chan domain.BridgeMessage, error) {
	return a.reader.ReadEvents(ctx)
}

// CaptureNode solicita que o navegador capture um elemento específico do DOM.
func (a *NativeMessagingAdapter) CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
	id := fmt.Sprintf("cn-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "capture_node", req)
	if err != nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}

	if resp.Error != "" {
		return domain.CaptureNodeResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.CaptureNodeResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("failed to unmarshal capture_node response: %w", err)
	}

	return result, nil
}

// TypeText simula a digitação de texto em um elemento do navegador.
func (a *NativeMessagingAdapter) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("tt-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "type_text", req)
	if err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}

	if resp.Error != "" {
		return domain.CommonResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to unmarshal type_text response: %w", err)
	}

	return result, nil
}

// WaitForElement aguarda até que um elemento apareça no DOM ou ocorra timeout.
func (a *NativeMessagingAdapter) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("wf-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "wait_for_element", req)
	if err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}

	if resp.Error != "" {
		return domain.CommonResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to unmarshal wait_for_element response: %w", err)
	}

	return result, nil
}

// GetValue recupera o valor atual (value) de um elemento.
func (a *NativeMessagingAdapter) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	id := fmt.Sprintf("gv-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "get_value", req)
	if err != nil {
		return domain.GetValueResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.GetValueResponse{}, err
	}

	if resp.Error != "" {
		return domain.GetValueResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.GetValueResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.GetValueResponse{}, fmt.Errorf("failed to unmarshal get_value response: %w", err)
	}

	return result, nil
}

// SelectOption seleciona uma opção em um elemento <select>.
func (a *NativeMessagingAdapter) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("so-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "select_option", req)
	if err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}

	if resp.Error != "" {
		return domain.CommonResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to unmarshal select_option response: %w", err)
	}

	return result, nil
}

// UploadFile realiza o upload de um arquivo enviando o conteúdo para o navegador em Base64.
func (a *NativeMessagingAdapter) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	id := fmt.Sprintf("uf-%d", a.idCount.Add(1))

	// Combina os parâmetros da requisição com o conteúdo em Base64
	params := map[string]any{
		"selector": req.Selector,
		"path":     req.Path,
		"content":  base64.StdEncoding.EncodeToString(content),
	}

	msg, err := domain.NewBridgeRequest(id, "upload_file", params)
	if err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}

	if resp.Error != "" {
		return domain.CommonResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to unmarshal upload_file response: %w", err)
	}

	return result, nil
}

// Scroll rola a página ou um elemento específico.
func (a *NativeMessagingAdapter) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("sc-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "scroll", req)
	if err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}

	if resp.Error != "" {
		return domain.CommonResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to unmarshal scroll response: %w", err)
	}

	return result, nil
}

// Hover simula o mouse hover em um elemento.
func (a *NativeMessagingAdapter) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("hv-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "hover", req)
	if err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}

	if resp.Error != "" {
		return domain.CommonResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to unmarshal hover response: %w", err)
	}

	return result, nil
}

// SwitchTab muda para a aba especificada.
func (a *NativeMessagingAdapter) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	id := fmt.Sprintf("st-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "switch_tab", req)
	if err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CommonResponse{}, err
	}

	if resp.Error != "" {
		return domain.CommonResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	var result domain.CommonResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return domain.CommonResponse{}, fmt.Errorf("failed to unmarshal switch_tab response: %w", err)
	}

	return result, nil
}

// Screenshot captura um screenshot PNG da aba ativa.
func (a *NativeMessagingAdapter) Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error) {
	id := fmt.Sprintf("ss-%d", a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, "screenshot", nil)
	if err != nil {
		return domain.CaptureNodeResponse{}, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
	if err != nil {
		return domain.CaptureNodeResponse{}, err
	}

	if resp.Error != "" {
		return domain.CaptureNodeResponse{
			Success: false,
			Message: resp.Error,
		}, nil
	}

	// Tenta desempacotar o resultado. A extensão pode retornar { "base64": "..." } ou o valor base64 direto.
	var result struct {
		Base64 string `json:"base64"`
	}
	if err := json.Unmarshal(resp.Result, &result); err == nil && result.Base64 != "" {
		return domain.CaptureNodeResponse{
			Success: true,
			Content: result.Base64,
			Format:  "png",
		}, nil
	}

	// Fallback: tenta desempacotar como string pura se não for objeto
	var base64Str string
	if err := json.Unmarshal(resp.Result, &base64Str); err == nil && base64Str != "" {
		return domain.CaptureNodeResponse{
			Success: true,
			Content: base64Str,
			Format:  "png",
		}, nil
	}

	return domain.CaptureNodeResponse{}, fmt.Errorf("failed to unmarshal screenshot response: invalid format")
}

// Run inicia o loop de leitura e processamento de mensagens.
// Bloqueia até que o contexto seja cancelado ou ocorra um erro fatal.
func (a *NativeMessagingAdapter) Run(ctx context.Context) error {
	return a.reader.ReadLoop(ctx)
}
