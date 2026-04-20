package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// MCPService implementa a lógica de negócio do protocolo MCP.
type MCPService struct {
	browser ports.BrowserPort
	fs      ports.FileSystemPort
	tools   map[string]domain.Tool
	idCount atomic.Uint64
}

// NewMCPService cria uma nova instância do serviço MCP.
func NewMCPService(browser ports.BrowserPort, fs ports.FileSystemPort) *MCPService {
	s := &MCPService{
		browser: browser,
		fs:      fs,
		tools:   make(map[string]domain.Tool),
	}
	s.registerTools()
	return s
}

// ListTools retorna a lista de ferramentas disponíveis.
func (s *MCPService) ListTools(ctx context.Context) ([]domain.Tool, error) {
	tools := make([]domain.Tool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}
	return tools, nil
}

// CallTool executa uma ferramenta no navegador.
func (s *MCPService) CallTool(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
	tool, ok := s.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	// Orquestração especial para capture_node (precisa de acesso ao FileSystem)
	if name == "capture_node" {
		var req domain.CaptureNodeRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for capture_node: %w", err)
		}

		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}

		// Chama o browser para capturar o nó
		resp, err := s.browser.CaptureNode(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("browser capture failed: %w", err)
		}

		if !resp.Success {
			return nil, fmt.Errorf("browser failed to capture node: %s", resp.Message)
		}

		// Se o browser retornou dados, salva no disco
		if resp.Content != "" {
			var data []byte
			if req.Format == "png" {
				// PNG vem em base64
				decoded, err := base64.StdEncoding.DecodeString(resp.Content)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 content: %w", err)
				}
				data = decoded
			} else {
				// HTML vem como string pura
				data = []byte(resp.Content)
			}

			if err := s.fs.WriteFile(ctx, req.Path, data); err != nil {
				return nil, fmt.Errorf("failed to save file to %s: %w", req.Path, err)
			}
			resp.FilePath = req.Path
			resp.Message = fmt.Sprintf("Successfully saved %s to %s", req.Format, req.Path)
		}

		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)
	}

	// Orquestração para novas ferramentas
	switch name {
	case "type_text":
		var req domain.TypeTextRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for %s: %w", name, err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		resp, err := s.browser.TypeText(ctx, req)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)

	case "wait_for_element":
		var req domain.WaitForElementRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for %s: %w", name, err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		resp, err := s.browser.WaitForElement(ctx, req)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)

	case "get_value":
		var req domain.GetValueRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for %s: %w", name, err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		resp, err := s.browser.GetValue(ctx, req)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)

	case "select_option":
		var req domain.SelectOptionRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for %s: %w", name, err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		resp, err := s.browser.SelectOption(ctx, req)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)

	case "upload_file":
		var req domain.UploadFileRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for %s: %w", name, err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		// Lê o arquivo do sistema de arquivos local
		content, err := s.fs.ReadFile(ctx, req.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file for upload: %w", err)
		}
		resp, err := s.browser.UploadFile(ctx, req, content)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)

	case "scroll":
		var req domain.ScrollRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for %s: %w", name, err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		resp, err := s.browser.Scroll(ctx, req)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)

	case "hover":
		var req domain.HoverRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for %s: %w", name, err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		resp, err := s.browser.Hover(ctx, req)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)

	case "switch_tab":
		var req domain.SwitchTabRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params for %s: %w", name, err)
		}
		if err := req.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		resp, err := s.browser.SwitchTab(ctx, req)
		if err != nil {
			return nil, err
		}
		result, _ := json.Marshal(resp)
		return s.formatMCPResult(name, result)

	case "screenshot":
		var req domain.ScreenshotRequest
		// params pode estar vazio para screenshot
		if len(params) > 0 && string(params) != "{}" {
			if err := json.Unmarshal(params, &req); err != nil {
				return nil, fmt.Errorf("invalid params for screenshot: %w", err)
			}
			if err := req.Validate(); err != nil {
				return nil, fmt.Errorf("validation failed: %w", err)
			}
		}

		// Chama o navegador para capturar a tela
		resp, err := s.browser.Screenshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("browser screenshot failed: %w", err)
		}

		if !resp.Success {
			return nil, fmt.Errorf("browser failed to capture screenshot: %s", resp.Message)
		}

		// Se um path foi fornecido, salva no disco
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
		return s.formatMCPResult(name, result)
	}

	// Gera ID dinâmico para correlação na bridge
	id := fmt.Sprintf("%d", s.idCount.Add(1))

	// Envia o comando para a bridge (BrowserPort)
	req := domain.BridgeMessage{
		ID:     id,
		Tool:   tool.Name,
		Params: params,
	}

	var resp domain.BridgeMessage
	var err error

	// Resiliência: Implementa retentativas simples com timeout
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		// Timeout de 30 segundos por tentativa (nível de aplicação)
		tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		resp, err = s.browser.ExecuteCommand(tctx, req)
		cancel()

		if err == nil {
			break
		}

		// Se o erro for cancelamento do contexto pai, não retenta
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
			return nil, err
		}

		// Espera exponencial simples ou fixa antes de retentar
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

	// Conversão para o formato de resposta MCP Tool Result
	return s.formatMCPResult(name, resp.Result)
}

// formatMCPResult converte o resultado bruto da bridge para o padrão de resposta MCP.
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

	response := mcpResponse{
		Content: []mcpContent{},
	}

	// Tratamento especializado por tipo de ferramenta
	switch toolName {
	case "capture_node":
		var resp domain.CaptureNodeResponse
		if err := json.Unmarshal(result, &resp); err == nil {
			msg := fmt.Sprintf("Arquivo salvo em: %s", resp.FilePath)
			if resp.Message != "" {
				msg = resp.Message
			}
			response.Content = append(response.Content, mcpContent{
				Type: "text",
				Text: msg,
			})
		} else {
			response.Content = append(response.Content, mcpContent{
				Type: "text",
				Text: string(result),
			})
		}

	case "screenshot":
		// Primeiro tenta como CaptureNodeResponse (novo formato interno devido à orquestração de salvamento)
		var resp domain.CaptureNodeResponse
		if err := json.Unmarshal(result, &resp); err == nil && resp.Content != "" && resp.Format == "png" {
			if resp.FilePath == "" {
				// Retorna a imagem via MCP se não foi salva em arquivo
				response.Content = append(response.Content, mcpContent{
					Type:     "image",
					Data:     resp.Content,
					MimeType: "image/png",
				})
			} else {
				// Retorna mensagem de confirmação se foi salva em arquivo
				msg := fmt.Sprintf("Screenshot salvo em: %s", resp.FilePath)
				if resp.Message != "" {
					msg = resp.Message
				}
				response.Content = append(response.Content, mcpContent{
					Type: "text",
					Text: msg,
				})
			}
			break
		}

		// Fallback: Espera um objeto { "base64": "..." } ou string base64 pura (origem direta da bridge)
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
			// Fallback para texto se não for um base64 válido
			response.Content = append(response.Content, mcpContent{
				Type: "text",
				Text: string(result),
			})
		}

	default:
		// Padrão: Retorna o JSON como texto
		response.Content = append(response.Content, mcpContent{
			Type: "text",
			Text: string(result),
		})
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
				"format":   map[string]any{"type": "string", "enum": []string{"png", "html"}, "description": "Formato de saída"},
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
