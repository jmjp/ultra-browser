package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// NativeMessagingAdapter implementa a interface ports.BrowserPort.
type NativeMessagingAdapter struct {
	reader  *Reader
	writer  *Writer
	idCount atomic.Uint64
}

var _ ports.BrowserPort = (*NativeMessagingAdapter)(nil)

// NewNativeMessagingAdapter cria uma nova instância do adaptador.
func NewNativeMessagingAdapter(r *Reader, w *Writer) *NativeMessagingAdapter {
	a := &NativeMessagingAdapter{reader: r, writer: w}
	r.SetMessageHandler(w.HandleResponse)
	return a
}

// execute é o helper genérico que elimina o boilerplate de cada método:
// gera ID, cria BridgeRequest, executa, verifica erro e faz unmarshal.
func execute[T any](ctx context.Context, a *NativeMessagingAdapter, prefix, tool string, req any) (T, error) {
	var zero T
	id := fmt.Sprintf("%s-%d", prefix, a.idCount.Add(1))
	msg, err := domain.NewBridgeRequest(id, tool, req)
	if err != nil {
		return zero, fmt.Errorf("failed to create bridge request: %w", err)
	}

	resp, err := a.writer.ExecuteCommand(ctx, msg)
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
	return execute[domain.CaptureNodeResponse](ctx, a, "cn", "capture_node", req)
}

// TypeText simula a digitação de texto em um elemento do navegador.
func (a *NativeMessagingAdapter) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	return execute[domain.CommonResponse](ctx, a, "tt", "type_text", req)
}

// WaitForElement aguarda até que um elemento apareça no DOM ou ocorra timeout.
func (a *NativeMessagingAdapter) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	return execute[domain.CommonResponse](ctx, a, "wf", "wait_for_element", req)
}

// GetValue recupera o valor atual (value) de um elemento.
func (a *NativeMessagingAdapter) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	return execute[domain.GetValueResponse](ctx, a, "gv", "get_value", req)
}

// SelectOption seleciona uma opção em um elemento <select>.
func (a *NativeMessagingAdapter) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	return execute[domain.CommonResponse](ctx, a, "so", "select_option", req)
}

// UploadFile realiza o upload de um arquivo enviando o conteúdo para o navegador em Base64.
func (a *NativeMessagingAdapter) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	params := map[string]any{
		"selector": req.Selector,
		"path":     req.Path,
		"content":  base64.StdEncoding.EncodeToString(content),
	}
	return execute[domain.CommonResponse](ctx, a, "uf", "upload_file", params)
}

// Scroll rola a página ou um elemento específico.
func (a *NativeMessagingAdapter) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	return execute[domain.CommonResponse](ctx, a, "sc", "scroll", req)
}

// Hover simula o mouse hover em um elemento.
func (a *NativeMessagingAdapter) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	return execute[domain.CommonResponse](ctx, a, "hv", "hover", req)
}

// SwitchTab muda para a aba especificada.
func (a *NativeMessagingAdapter) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	return execute[domain.CommonResponse](ctx, a, "st", "switch_tab", req)
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
		return domain.CaptureNodeResponse{Success: false, Message: resp.Error}, nil
	}

	// Tenta desempacotar como { "base64": "..." }
	var data struct {
		Base64 string `json:"base64"`
	}
	if err := json.Unmarshal(resp.Result, &data); err == nil && data.Base64 != "" {
		return domain.CaptureNodeResponse{Success: true, Content: data.Base64, Format: "png"}, nil
	}

	// Fallback: string base64 pura
	var base64Str string
	if err := json.Unmarshal(resp.Result, &base64Str); err == nil && base64Str != "" {
		return domain.CaptureNodeResponse{Success: true, Content: base64Str, Format: "png"}, nil
	}

	return domain.CaptureNodeResponse{}, fmt.Errorf("failed to unmarshal screenshot response: invalid format")
}

// Run inicia o loop de leitura e processamento de mensagens.
func (a *NativeMessagingAdapter) Run(ctx context.Context) error {
	return a.reader.ReadLoop(ctx)
}
