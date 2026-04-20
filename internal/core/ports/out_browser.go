package ports

import (
	"context"

	"ultra-browser/internal/core/domain"
)

// BrowserPort define a interface outbound (secundária) para a comunicação com o navegador.
// Esta porta abstrai o Native Messaging Bridge que conecta o núcleo ao Chrome.
type BrowserPort interface {
	// ExecuteCommand envia uma mensagem de comando para a ponte do navegador
	// e retorna a resposta correspondente de forma síncrona ou correlacionada.
	ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error)

	// ReadEvents fornece um canal para leitura de eventos não solicitados
	// enviados pelo navegador (ex: notificações de mudança de estado).
	ReadEvents(ctx context.Context) (<-chan domain.BridgeMessage, error)

	// CaptureNode solicita que o navegador capture um elemento específico do DOM.
	CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error)

	// TypeText simula a digitação de texto em um elemento do navegador.
	TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error)

	// WaitForElement aguarda até que um elemento apareça no DOM ou ocorra timeout.
	WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error)

	// GetValue recupera o valor atual (value) de um elemento (ex: input).
	GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error)

	// SelectOption seleciona uma opção em um elemento <select>.
	SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error)

	// UploadFile realiza o upload de um arquivo enviando o conteúdo para o navegador.
	UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error)

	// Scroll rola a página ou um elemento específico.
	Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error)

	// Hover simula o evento de mouse hover (mouseover) em um elemento.
	Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error)

	// SwitchTab muda para a aba especificada.
	SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error)

	// Screenshot captura um screenshot PNG da aba ativa.
	Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error)
}
