package mcphttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// MCPServer representa o adaptador de entrada HTTP para o protocolo MCP.
type MCPServer struct {
	service ports.MCPService
}

// NewMCPServer cria uma nova instância do servidor MCP.
func NewMCPServer(service ports.MCPService) *MCPServer {
	return &MCPServer{service: service}
}

// ServeHTTP implementa a interface http.Handler para processar requisições MCP.
func (s *MCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS Headers básicos
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Suporte a SSE (Streaming): Detecta via Accept header
	if r.Header.Get("Accept") == "text/event-stream" {
		s.handleSSE(w, r)
		return
	}

	// Requisições normais de mensagens via POST
	if r.Method == http.MethodPost {
		s.handlePost(w, r)
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// handleSSE gerencia a conexão streaming Server-Sent Events.
func (s *MCPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// O servidor DEVE enviar um evento 'endpoint' com o URI para os POSTs subsequentes.
	// Usamos o path atual (ex: /mcp) como endpoint para as mensagens JSON-RPC.
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", r.URL.Path)
	flusher.Flush()

	// Mantém a conexão aberta para potenciais notificações.
	// O contexto é cancelado quando o cliente desconecta.
	<-r.Context().Done()
}

// handlePost processa requisições JSON-RPC 2.0 via POST.
func (s *MCPServer) handlePost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var msg domain.MCPMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		s.sendError(w, nil, -32700, "Parse error")
		return
	}

	// Resposta base
	response := domain.MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
	}

	ctx := r.Context()

	switch msg.Method {
	case "initialize":
		response.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{
					"listChanged": false,
				},
				"resources": map[string]any{
					"subscribe":   false,
					"listChanged": false,
				},
			},
			"serverInfo": map[string]string{
				"name":    "ultra-browser",
				"version": "0.1.0",
			},
		}

	case "tools/list":
		tools, err := s.service.ListTools(ctx)
		if err != nil {
			s.sendError(w, msg.ID, -32603, "Internal error: "+err.Error())
			return
		}
		response.Result = map[string]any{
			"tools": tools,
		}

	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			s.sendError(w, msg.ID, -32602, "Invalid params: failed to parse tool call arguments")
			return
		}

		result, err := s.service.CallTool(ctx, params.Name, params.Arguments)
		if err != nil {
			// Alguns erros de ferramenta podem ser retornados como sucesso com flag isError no MCP,
			// mas aqui estamos seguindo o padrão de erro JSON-RPC para falhas de execução.
			s.sendError(w, msg.ID, -32603, "Tool execution error: "+err.Error())
			return
		}
		response.Result = result

	case "notifications/initialized":
		// Confirmação de inicialização do cliente. Não requer resposta.
		w.WriteHeader(http.StatusNoContent)
		return

	case "ping":
		response.Result = map[string]any{}

	default:
		s.sendError(w, msg.ID, -32601, "Method not found: "+msg.Method)
		return
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Se falhar ao encodar a resposta, enviamos erro genérico se possível
		s.sendError(w, msg.ID, -32603, "Internal error: failed to encode response")
	}
}

// sendError envia uma resposta de erro formatada conforme JSON-RPC 2.0.
func (s *MCPServer) sendError(w http.ResponseWriter, id any, code int, message string) {
	resp := domain.MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &domain.MCPError{
			Code:    code,
			Message: message,
		},
	}

	// Para Parse Error (-32700), o status code 400 é comum em transportes HTTP.
	if code == -32700 {
		w.WriteHeader(http.StatusBadRequest)
	}

	json.NewEncoder(w).Encode(resp)
}
