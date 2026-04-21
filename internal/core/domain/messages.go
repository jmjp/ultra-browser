package domain

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

// BridgeMessage representa o envelope de mensagem para a comunicação com o navegador.
type BridgeMessage struct {
	ID     string          `json:"id"`
	Tool   string          `json:"tool,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// MCPMessage representa a base para mensagens do protocolo MCP (JSON-RPC 2.0).
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError representa um erro no protocolo MCP.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// NewBridgeRequest cria uma nova mensagem de requisição para a bridge.
func NewBridgeRequest(id, tool string, params any) (BridgeMessage, error) {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return BridgeMessage{}, err
	}
	return BridgeMessage{
		ID:     id,
		Tool:   tool,
		Params: rawParams,
	}, nil
}

// NewBridgeResponse cria uma nova mensagem de resposta para a bridge.
func NewBridgeResponse(id string, result any) (BridgeMessage, error) {
	rawResult, err := json.Marshal(result)
	if err != nil {
		return BridgeMessage{}, err
	}
	return BridgeMessage{
		ID:     id,
		Result: rawResult,
	}, nil
}

// CaptureNodeRequest representa os parâmetros para a ferramenta capture_node.
type CaptureNodeRequest struct {
	Selector string `json:"selector"`
	Format   string `json:"format"`
	Path     string `json:"path"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r *CaptureNodeRequest) Validate() error {
	if r.Selector == "" {
		return errors.New("selector is required")
	}

	// Tenta inferir o formato a partir da extensão se estiver vazio
	if r.Format == "" && r.Path != "" {
		ext := strings.ToLower(filepath.Ext(r.Path))
		if ext == ".png" {
			r.Format = "png"
		} else if ext == ".html" || ext == ".htm" {
			r.Format = "html"
		}
	}

	if r.Format != "png" && r.Format != "html" {
		return errors.New("invalid format: must be png or html")
	}
	if r.Path == "" {
		return errors.New("path is required")
	}
	// Remove a restrição de caminho absoluto - o filesystem adapter já valida segurança
	// Aceita caminhos relativos e absolutos
	return nil
}

// CaptureNodeResponse representa a resposta da ferramenta capture_node.
type CaptureNodeResponse struct {
	Success  bool   `json:"success"`
	Content  string `json:"content,omitempty"`
	Format   string `json:"format,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Message  string `json:"message,omitempty"`
}

// CommonResponse representa uma resposta básica de sucesso/falha com mensagem.
type CommonResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// TypeTextRequest representa os parâmetros para a ferramenta type_text.
type TypeTextRequest struct {
	Selector string `json:"selector"`
	Text     string `json:"text"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r TypeTextRequest) Validate() error {
	if r.Selector == "" {
		return errors.New("selector is required")
	}
	if r.Text == "" {
		return errors.New("text is required")
	}
	return nil
}

// WaitForElementRequest representa os parâmetros para a ferramenta wait_for_element.
type WaitForElementRequest struct {
	Selector string `json:"selector"`
	Timeout  int    `json:"timeout"` // em milissegundos
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r WaitForElementRequest) Validate() error {
	if r.Selector == "" {
		return errors.New("selector is required")
	}
	if r.Timeout < 0 {
		return errors.New("timeout must be non-negative")
	}
	return nil
}

// GetValueRequest representa os parâmetros para a ferramenta get_value.
type GetValueRequest struct {
	Selector string `json:"selector"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r GetValueRequest) Validate() error {
	if r.Selector == "" {
		return errors.New("selector is required")
	}
	return nil
}

// GetValueResponse representa a resposta da ferramenta get_value.
type GetValueResponse struct {
	Success bool   `json:"success"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message,omitempty"`
}

// GetContentRequest representa os parâmetros para a ferramenta get_content.
type GetContentRequest struct {
	Selector string `json:"selector,omitempty"` // Opcional, se vazio pega do body
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r GetContentRequest) Validate() error {
	return nil
}

// GetContentResponse representa a resposta da ferramenta get_content.
type GetContentResponse struct {
	Success bool   `json:"success"`
	Content string `json:"content,omitempty"`
	Message string `json:"message,omitempty"`
}

// SelectOptionRequest representa os parâmetros para a ferramenta select_option.
type SelectOptionRequest struct {
	Selector string `json:"selector"`
	Value    string `json:"value"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r SelectOptionRequest) Validate() error {
	if r.Selector == "" {
		return errors.New("selector is required")
	}
	if r.Value == "" {
		return errors.New("value is required")
	}
	return nil
}

// UploadFileRequest representa os parâmetros para a ferramenta upload_file.
type UploadFileRequest struct {
	Selector string `json:"selector"`
	Path     string `json:"path"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r UploadFileRequest) Validate() error {
	if r.Selector == "" {
		return errors.New("selector is required")
	}
	if r.Path == "" {
		return errors.New("path is required")
	}
	// Remove a restrição de caminho absoluto - o filesystem adapter já valida segurança
	return nil
}

// ScrollRequest representa os parâmetros para a ferramenta scroll.
type ScrollRequest struct {
	X        *int   `json:"x,omitempty"`
	Y        *int   `json:"y,omitempty"`
	Selector string `json:"selector,omitempty"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r ScrollRequest) Validate() error {
	if r.Selector == "" && (r.X == nil || r.Y == nil) {
		return errors.New("either selector or both x and y are required")
	}
	return nil
}

// HoverRequest representa os parâmetros para a ferramenta hover.
type HoverRequest struct {
	Selector string `json:"selector"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r HoverRequest) Validate() error {
	if r.Selector == "" {
		return errors.New("selector is required")
	}
	return nil
}

// SwitchTabRequest representa os parâmetros para a ferramenta switch_tab.
type SwitchTabRequest struct {
	TabID int `json:"tab_id"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r SwitchTabRequest) Validate() error {
	if r.TabID <= 0 {
		return errors.New("tab_id is required and must be positive")
	}
	return nil
}

// ScreenshotRequest representa os parâmetros para a ferramenta screenshot.
type ScreenshotRequest struct {
	Path string `json:"path,omitempty"`
}

// Validate verifica se a requisição possui parâmetros válidos.
func (r ScreenshotRequest) Validate() error {
	// Remove a restrição de caminho absoluto - o filesystem adapter já valida segurança
	return nil
}
