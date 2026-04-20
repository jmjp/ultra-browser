package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// toolHandler é a assinatura de um handler de ferramenta.
type toolHandler func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)

// MCPService implementa a lógica de negócio do protocolo MCP.
type MCPService struct {
	browser  ports.BrowserPort
	fs       ports.FileSystemPort
	tools    map[string]domain.Tool
	handlers map[string]toolHandler
	idCount  atomic.Uint64
}

// NewMCPService cria uma nova instância do serviço MCP.
func NewMCPService(browser ports.BrowserPort, fs ports.FileSystemPort) *MCPService {
	s := &MCPService{
		browser:  browser,
		fs:       fs,
		tools:    make(map[string]domain.Tool),
		handlers: make(map[string]toolHandler),
	}
	s.registerTools()
	s.registerHandlers()
	return s
}

// ListTools retorna a lista de ferramentas disponíveis em ordem alfabética.
func (s *MCPService) ListTools(ctx context.Context) ([]domain.Tool, error) {
	tools := make([]domain.Tool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools, nil
}

// CallTool executa uma ferramenta pelo nome usando o registry de handlers.
func (s *MCPService) CallTool(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
	handler, ok := s.handlers[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return handler(ctx, params)
}

// registerHandlers associa cada nome de ferramenta ao seu handler.
func (s *MCPService) registerHandlers() {
	// Ferramentas orquestradas localmente (precisam de fs ou lógica especial)
	s.handlers["capture_node"] = s.handleCaptureNode
	s.handlers["screenshot"] = s.handleScreenshot
	s.handlers["upload_file"] = s.handleUploadFile

	// Ferramentas com validação tipada + dispatch direto para o browser
	s.handlers["type_text"] = makeTypedHandler(s, func(ctx context.Context, req domain.TypeTextRequest) (any, error) {
		return s.browser.TypeText(ctx, req)
	})
	s.handlers["wait_for_element"] = makeTypedHandler(s, func(ctx context.Context, req domain.WaitForElementRequest) (any, error) {
		return s.browser.WaitForElement(ctx, req)
	})
	s.handlers["get_value"] = makeTypedHandler(s, func(ctx context.Context, req domain.GetValueRequest) (any, error) {
		return s.browser.GetValue(ctx, req)
	})
	s.handlers["select_option"] = makeTypedHandler(s, func(ctx context.Context, req domain.SelectOptionRequest) (any, error) {
		return s.browser.SelectOption(ctx, req)
	})
	s.handlers["scroll"] = makeTypedHandler(s, func(ctx context.Context, req domain.ScrollRequest) (any, error) {
		return s.browser.Scroll(ctx, req)
	})
	s.handlers["hover"] = makeTypedHandler(s, func(ctx context.Context, req domain.HoverRequest) (any, error) {
		return s.browser.Hover(ctx, req)
	})
	s.handlers["switch_tab"] = makeTypedHandler(s, func(ctx context.Context, req domain.SwitchTabRequest) (any, error) {
		return s.browser.SwitchTab(ctx, req)
	})

	// Ferramentas genéricas: delegadas à bridge via ExecuteCommand com retry
	for name := range s.tools {
		if _, already := s.handlers[name]; !already {
			n := name // captura para closure
			s.handlers[n] = func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
				return s.executeBridgeCommand(ctx, n, params)
			}
		}
	}
}

// makeTypedHandler cria um toolHandler genérico que faz unmarshal, valida e delega.
func makeTypedHandler[T interface{ Validate() error }](s *MCPService, fn func(context.Context, T) (any, error)) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		var req T
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		resp, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult("generic", result)
	}
}

// executeBridgeCommand envia um comando à bridge com retry exponencial.
// A lógica de retry pertence à infraestrutura — fica aqui como camada de resiliência de aplicação.
func (s *MCPService) executeBridgeCommand(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
	id := fmt.Sprintf("%d", s.idCount.Add(1))
	req := domain.BridgeMessage{
		ID:     id,
		Tool:   name,
		Params: params,
	}

	const maxRetries = 3
	var (
		resp domain.BridgeMessage
		err  error
	)

	for i := 0; i < maxRetries; i++ {
		tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		resp, err = s.browser.ExecuteCommand(tctx, req)
		cancel()

		if err == nil {
			break
		}
		if errors.Is(err, context.Canceled) || (errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil) {
			return nil, err
		}
		if i < maxRetries-1 {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("bridge execution failed after %d attempts: %w", maxRetries, err)
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	return s.formatMCPResult(name, resp.Result)
}

// --- Handlers orquestrados ---

func (s *MCPService) handleCaptureNode(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var req domain.CaptureNodeRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params for capture_node: %w", err)
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	resp, err := s.browser.CaptureNode(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("browser capture failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("browser failed to capture node: %s", resp.Message)
	}

	if resp.Content != "" {
		data, err := s.decodeContent(resp.Content, req.Format)
		if err != nil {
			return nil, err
		}
		if err := s.fs.WriteFile(ctx, req.Path, data); err != nil {
			return nil, fmt.Errorf("failed to save file to %s: %w", req.Path, err)
		}
		resp.FilePath = req.Path
		resp.Message = fmt.Sprintf("Successfully saved %s to %s", req.Format, req.Path)
	}

	result, _ := json.Marshal(resp)
	return s.formatMCPResult("capture_node", result)
}

func (s *MCPService) handleScreenshot(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var req domain.ScreenshotRequest
	if len(params) > 0 && string(params) != "{}" {
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for screenshot: %w", err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}

	resp, err := s.browser.Screenshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("browser screenshot failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("browser failed to capture screenshot: %s", resp.Message)
	}

	if req.Path != "" && resp.Content != "" {
		decoded, err := base64.StdEncoding.DecodeString(resp.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 screenshot: %w", err)
		}
		if err := s.fs.WriteFile(ctx, req.Path, decoded); err != nil {
			return nil, fmt.Errorf("failed to save screenshot to %s: %w", req.Path, err)
		}
		resp.FilePath = req.Path
		resp.Message = fmt.Sprintf("Screenshot saved to %s", req.Path)
	}

	result, _ := json.Marshal(resp)
	return s.formatMCPResult("screenshot", result)
}

func (s *MCPService) handleUploadFile(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var req domain.UploadFileRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params for upload_file: %w", err)
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	content, err := s.fs.ReadFile(ctx, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file for upload: %w", err)
	}

	resp, err := s.browser.UploadFile(ctx, req, content)
	if err != nil {
		return nil, err
	}

	result, _ := json.Marshal(resp)
	return s.formatMCPResult("generic", result)
}

// --- Helpers ---

// decodeContent converte o conteúdo retornado pelo browser para bytes.
func (s *MCPService) decodeContent(content, format string) ([]byte, error) {
	if format == "png" {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 content: %w", err)
		}
		return decoded, nil
	}
	return []byte(content), nil
}

// formatMCPResult converte o resultado bruto para o padrão de resposta MCP.
func (s *MCPService) formatMCPResult(toolName string, result json.RawMessage) (json.RawMessage, error) {
	type mcpContent struct {
		Type     string `json:"type"`
		Text     string `json:"text,omitempty"`
		Data     string `json:"data,omitempty"`
		MimeType string `json:"mimeType,omitempty"`
	}
	type mcpResponse struct {
		Content []mcpContent `json:"content"`
		IsError bool         `json:"isError,omitempty"`
	}

	response := mcpResponse{Content: []mcpContent{}}

	switch toolName {
	case "capture_node":
		var resp domain.CaptureNodeResponse
		if err := json.Unmarshal(result, &resp); err == nil {
			msg := fmt.Sprintf("Arquivo salvo em: %s", resp.FilePath)
			if resp.Message != "" {
				msg = resp.Message
			}
			response.Content = append(response.Content, mcpContent{Type: "text", Text: msg})
		} else {
			response.Content = append(response.Content, mcpContent{Type: "text", Text: string(result)})
		}

	case "screenshot":
		var resp domain.CaptureNodeResponse
		if err := json.Unmarshal(result, &resp); err == nil && resp.Content != "" && resp.Format == "png" {
			if resp.FilePath == "" {
				response.Content = append(response.Content, mcpContent{
					Type:     "image",
					Data:     resp.Content,
					MimeType: "image/png",
				})
			} else {
				msg := fmt.Sprintf("Screenshot salvo em: %s", resp.FilePath)
				if resp.Message != "" {
					msg = resp.Message
				}
				response.Content = append(response.Content, mcpContent{Type: "text", Text: msg})
			}
			break
		}
		// Fallback: objeto { "base64": "..." }
		var data struct {
			Base64 string `json:"base64"`
		}
		if err := json.Unmarshal(result, &data); err == nil && data.Base64 != "" {
			response.Content = append(response.Content, mcpContent{
				Type:     "image",
				Data:     data.Base64,
				MimeType: "image/png",
			})
		} else {
			response.Content = append(response.Content, mcpContent{Type: "text", Text: string(result)})
		}

	default:
		response.Content = append(response.Content, mcpContent{Type: "text", Text: string(result)})
	}

	return json.Marshal(response)
}

// registerTools define as ferramentas suportadas conforme o PRD.md e spec.yaml.
func (s *MCPService) registerTools() {
	s.tools["list_tabs"] = domain.Tool{
		Name:        "list_tabs",
		Description: "Lista todas as janelas e abas abertas no navegador.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}
	s.tools["navigate"] = domain.Tool{
		Name:        "navigate",
		Description: "Navega para uma URL específica na aba ativa ou cria uma nova.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "URL para navegar"},
			},
			"required": []string{"url"},
		},
	}
	s.tools["screenshot"] = domain.Tool{
		Name:        "screenshot",
		Description: "Captura um screenshot (PNG) da aba ativa no momento.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Caminho absoluto para salvar o screenshot (.png)"},
			},
		},
	}
	s.tools["get_content"] = domain.Tool{
		Name:        "get_content",
		Description: "Extrai o conteúdo de texto (inner text) da página na aba ativa.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}
	s.tools["click"] = domain.Tool{
		Name:        "click",
		Description: "Executa um clique em um elemento da página identificado por um seletor CSS.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento (ex: #submit-btn)"},
			},
			"required": []string{"selector"},
		},
	}
	s.tools["execute_script"] = domain.Tool{
		Name:        "execute_script",
		Description: "Executa código JavaScript arbitrário no contexto da página da aba ativa.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"script": map[string]any{"type": "string", "description": "Código JS a ser executado"},
			},
			"required": []string{"script"},
		},
	}
	s.tools["capture_node"] = domain.Tool{
		Name:        "capture_node",
		Description: "Captura um elemento específico do DOM e o salva em disco (PNG ou HTML).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento"},
				"format":   map[string]any{"type": "string", "enum": []string{"png", "html"}, "description": "Formato de saída", "default": "html"},
				"path":     map[string]any{"type": "string", "description": "Caminho absoluto para salvar o arquivo"},
			},
			"required": []string{"selector", "path"},
		},
	}
	s.tools["type_text"] = domain.Tool{
		Name:        "type_text",
		Description: "Simula a digitação de texto em um elemento do navegador.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento"},
				"text":     map[string]any{"type": "string", "description": "Texto para digitar"},
			},
			"required": []string{"selector", "text"},
		},
	}
	s.tools["wait_for_element"] = domain.Tool{
		Name:        "wait_for_element",
		Description: "Aguarda até que um elemento apareça no DOM ou ocorra timeout.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento"},
				"timeout":  map[string]any{"type": "integer", "description": "Timeout em milissegundos", "default": 5000},
			},
			"required": []string{"selector"},
		},
	}
	s.tools["get_value"] = domain.Tool{
		Name:        "get_value",
		Description: "Recupera o valor atual (value) de um elemento (ex: input).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento"},
			},
			"required": []string{"selector"},
		},
	}
	s.tools["select_option"] = domain.Tool{
		Name:        "select_option",
		Description: "Seleciona uma opção em um elemento <select>.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento"},
				"value":    map[string]any{"type": "string", "description": "Valor da opção para selecionar"},
			},
			"required": []string{"selector", "value"},
		},
	}
	s.tools["upload_file"] = domain.Tool{
		Name:        "upload_file",
		Description: "Realiza o upload de um arquivo para um elemento de input.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento input file"},
				"path":     map[string]any{"type": "string", "description": "Caminho absoluto do arquivo local"},
			},
			"required": []string{"selector", "path"},
		},
	}
	s.tools["scroll"] = domain.Tool{
		Name:        "scroll",
		Description: "Rola a página ou um elemento específico.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento (opcional)"},
				"x":        map[string]any{"type": "integer", "description": "Posição horizontal (opcional)"},
				"y":        map[string]any{"type": "integer", "description": "Posição vertical (opcional)"},
			},
		},
	}
	s.tools["hover"] = domain.Tool{
		Name:        "hover",
		Description: "Simula o evento de mouse hover (mouseover) em um elemento.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector": map[string]any{"type": "string", "description": "Seletor CSS do elemento"},
			},
			"required": []string{"selector"},
		},
	}
	s.tools["switch_tab"] = domain.Tool{
		Name:        "switch_tab",
		Description: "Muda o foco para a aba especificada pelo seu ID.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tab_id": map[string]any{"type": "integer", "description": "ID da aba para ativar"},
			},
			"required": []string{"tab_id"},
		},
	}
}
