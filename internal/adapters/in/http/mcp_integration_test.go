package mcphttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
	mcphttp "ultra-browser/internal/adapters/in/http"
	"ultra-browser/internal/adapters/out/bridge"
	"ultra-browser/internal/adapters/out/filesystem"
	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
	"ultra-browser/internal/core/services"
)

// TestMCPServer_CaptureNodeIntegration valida a integração final da ferramenta capture_node no servidor MCP.
// Este teste garante que a injeção de dependência e o roteamento de ferramentas no servidor HTTP
// estão funcionando corretamente para a nova funcionalidade.
func TestMCPServer_CaptureNodeIntegration(t *testing.T) {
	mockBrowser := &ports.MockBrowserPort{}
	mockFS := &ports.MockFileSystemPort{}
	service := services.NewMCPService(mockBrowser, mockFS)
	server := mcphttp.NewMCPServer(service)

	t.Run("Success_PNG", func(t *testing.T) {
		absPath := "/tmp/capture.png"
		if runtime.GOOS == "windows" {
			absPath = "C:\\tmp\\capture.png"
		}

		mockBrowser.CaptureNodeFunc = func(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
			if req.Selector != "#main" || req.Format != "png" || req.Path != absPath {
				return domain.CaptureNodeResponse{}, fmt.Errorf("unexpected params: %+v", req)
			}
			return domain.CaptureNodeResponse{
				Success: true,
				Content: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==", // 1x1 base64 png
				Format:  "png",
			}, nil
		}

		mockFS.WriteFileFunc = func(ctx context.Context, path string, data []byte) error {
			if path != absPath {
				return fmt.Errorf("unexpected path: %s", path)
			}
			if len(data) == 0 {
				return errors.New("data should not be empty")
			}
			return nil
		}

		reqBody := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "capture_node", "arguments": {"selector": "#main", "format": "png", "path": %q}}}`, absPath)
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(reqBody))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp domain.MCPMessage
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		if resp.Error != nil {
			t.Errorf("expected no error, got %v", resp.Error)
		}

		result := resp.Result.(map[string]any)
		content := result["content"].([]any)
		if len(content) != 1 {
			t.Fatalf("expected 1 content element, got %d", len(content))
		}
		item := content[0].(map[string]any)
		if item["type"] != "text" {
			t.Errorf("expected type text, got %v", item["type"])
		}
		expectedText := fmt.Sprintf("Successfully saved png to %s", absPath)
		if item["text"] != expectedText {
			t.Errorf("expected text %q, got %q", expectedText, item["text"])
		}
	})

	t.Run("Failure_Validation", func(t *testing.T) {
		reqBody := `{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "capture_node", "arguments": {"selector": "", "format": "invalid", "path": "relative/path"}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(reqBody))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp domain.MCPMessage
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		// Erros de ferramenta no MCP devem vir no Result com isError: true
		if resp.Error != nil {
			t.Errorf("expected no JSON-RPC error, got %v", resp.Error)
		}

		result, ok := resp.Result.(map[string]any)
		if !ok {
			t.Fatal("expected result to be a map")
		}

		if result["isError"] != true {
			t.Error("expected isError to be true in result")
		}

		content := result["content"].([]any)
		item := content[0].(map[string]any)
		if !strings.Contains(strings.ToLower(item["text"].(string)), "validation") {
			t.Errorf("expected validation error message in content, got %q", item["text"])
		}
	})

}

// TestMCPServer_ToolsIntegration valida a integração das novas ferramentas no servidor MCP.
func TestMCPServer_ToolsIntegration(t *testing.T) {
	mockBrowser := &ports.MockBrowserPort{}
	mockFS := &ports.MockFileSystemPort{}
	service := services.NewMCPService(mockBrowser, mockFS)
	server := mcphttp.NewMCPServer(service)

	t.Run("type_text", func(t *testing.T) {
		mockBrowser.TypeTextFunc = func(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
			if req.Selector != "#input" || req.Text != "hello world" {
				return domain.CommonResponse{}, fmt.Errorf("unexpected params: %+v", req)
			}
			return domain.CommonResponse{Success: true, Message: "Typed hello world"}, nil
		}

		reqBody := `{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "type_text", "arguments": {"selector": "#input", "text": "hello world"}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		var resp domain.MCPMessage
		json.NewDecoder(w.Body).Decode(&resp)
		result := resp.Result.(map[string]any)
		content := result["content"].([]any)[0].(map[string]any)
		if !strings.Contains(content["text"].(string), "Typed hello world") {
			t.Errorf("expected text to contain success message, got %q", content["text"])
		}
	})

	t.Run("wait_for_element", func(t *testing.T) {
		mockBrowser.WaitForElementFunc = func(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
			if req.Selector != ".loaded" {
				return domain.CommonResponse{}, fmt.Errorf("unexpected selector: %s", req.Selector)
			}
			return domain.CommonResponse{Success: true, Message: "Element found"}, nil
		}

		reqBody := `{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "wait_for_element", "arguments": {"selector": ".loaded"}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		var resp domain.MCPMessage
		json.NewDecoder(w.Body).Decode(&resp)
		result := resp.Result.(map[string]any)
		content := result["content"].([]any)[0].(map[string]any)
		if !strings.Contains(content["text"].(string), "Element found") {
			t.Errorf("expected text to contain success message, got %q", content["text"])
		}
	})

	t.Run("get_value", func(t *testing.T) {
		mockBrowser.GetValueFunc = func(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
			if req.Selector != "#email" {
				return domain.GetValueResponse{}, fmt.Errorf("unexpected selector: %s", req.Selector)
			}
			return domain.GetValueResponse{Value: "test@example.com", Success: true}, nil
		}

		reqBody := `{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "get_value", "arguments": {"selector": "#email"}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		var resp domain.MCPMessage
		json.NewDecoder(w.Body).Decode(&resp)
		result := resp.Result.(map[string]any)
		content := result["content"].([]any)[0].(map[string]any)
		if !strings.Contains(content["text"].(string), "test@example.com") {
			t.Errorf("expected value in response, got %q", content["text"])
		}
	})

	t.Run("select_option", func(t *testing.T) {
		mockBrowser.SelectOptionFunc = func(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
			if req.Selector != "#country" || req.Value != "BR" {
				return domain.CommonResponse{}, fmt.Errorf("unexpected params: %+v", req)
			}
			return domain.CommonResponse{Success: true, Message: "Option BR selected"}, nil
		}

		reqBody := `{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": "select_option", "arguments": {"selector": "#country", "value": "BR"}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		var resp domain.MCPMessage
		json.NewDecoder(w.Body).Decode(&resp)
		result := resp.Result.(map[string]any)
		content := result["content"].([]any)[0].(map[string]any)
		if !strings.Contains(content["text"].(string), "Option BR selected") {
			t.Errorf("expected success message, got %q", content["text"])
		}
	})

	t.Run("upload_file", func(t *testing.T) {
		testFilePath := "/path/to/test.txt"
		if runtime.GOOS == "windows" {
			testFilePath = "C:\\path\\to\\test.txt"
		}
		testContent := []byte("file content")

		mockFS.ReadFileFunc = func(ctx context.Context, path string) ([]byte, error) {
			if path != testFilePath {
				return nil, fmt.Errorf("unexpected path: %s", path)
			}
			return testContent, nil
		}

		mockBrowser.UploadFileFunc = func(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
			if req.Selector != "#upload" || req.Path != testFilePath {
				return domain.CommonResponse{}, fmt.Errorf("unexpected params: %+v", req)
			}
			if string(content) != string(testContent) {
				return domain.CommonResponse{}, fmt.Errorf("unexpected content: %s", string(content))
			}
			return domain.CommonResponse{Success: true, Message: "File uploaded"}, nil
		}

		reqBody := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": {"name": "upload_file", "arguments": {"selector": "#upload", "path": %q}}}`, testFilePath)
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		var resp domain.MCPMessage
		json.NewDecoder(w.Body).Decode(&resp)
		result := resp.Result.(map[string]any)
		content := result["content"].([]any)[0].(map[string]any)
		if !strings.Contains(content["text"].(string), "File uploaded") {
			t.Errorf("expected success message, got %q", content["text"])
		}
	})

	t.Run("scroll", func(t *testing.T) {
		mockBrowser.ScrollFunc = func(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
			if req.Selector != "#footer" || *req.Y != 100 {
				return domain.CommonResponse{}, fmt.Errorf("unexpected params: %+v", req)
			}
			return domain.CommonResponse{Success: true, Message: "Scrolled to footer"}, nil
		}

		reqBody := `{"jsonrpc": "2.0", "id": 6, "method": "tools/call", "params": {"name": "scroll", "arguments": {"selector": "#footer", "y": 100}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		var resp domain.MCPMessage
		json.NewDecoder(w.Body).Decode(&resp)
		result := resp.Result.(map[string]any)
		content := result["content"].([]any)[0].(map[string]any)
		if !strings.Contains(content["text"].(string), "Scrolled to footer") {
			t.Errorf("expected success message, got %q", content["text"])
		}
	})

	t.Run("hover", func(t *testing.T) {
		mockBrowser.HoverFunc = func(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
			if req.Selector != "#menu" {
				return domain.CommonResponse{}, fmt.Errorf("unexpected selector: %s", req.Selector)
			}
			return domain.CommonResponse{Success: true, Message: "Hovered over menu"}, nil
		}

		reqBody := `{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": {"name": "hover", "arguments": {"selector": "#menu"}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		var resp domain.MCPMessage
		json.NewDecoder(w.Body).Decode(&resp)
		result := resp.Result.(map[string]any)
		content := result["content"].([]any)[0].(map[string]any)
		if !strings.Contains(content["text"].(string), "Hovered over menu") {
			t.Errorf("expected success message, got %q", content["text"])
		}
	})
}

// TestMCPServer_WebSocketIntegration valida o fluxo completo via WebSocket.
// 1. Inicia o servidor HTTP.
// 2. Conecta um cliente WebSocket simulando a extensão Chrome.
// 3. Faz uma requisição MCP que exige uma ferramenta de browser.
// 4. Verifica se o comando chegou ao cliente WebSocket.
// 5. Envia uma resposta fake do cliente WebSocket para o Go.
// 6. Verifica se a resposta MCP final contém os dados corretos.
func TestMCPServer_WebSocketIntegration(t *testing.T) {
	// 1. Configura infraestrutura com Hub real para testar a ponte WebSocket
	hub := bridge.NewBridgeHub()
	fsAdapter := filesystem.NewLocalFileSystemAdapter(t.TempDir())
	service := services.NewMCPService(hub, fsAdapter)
	serverMCP := mcphttp.NewMCPServer(service)

	mux := http.NewServeMux()
	mux.Handle("/mcp", serverMCP)

	// Handler WebSocket que registra o adaptador no hub (simula o cmd/api/main.go)
	mux.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		adapter := bridge.NewWebSocketAdapterFromConn(ws)
		hub.Register(adapter)
		<-adapter.Done()
		hub.UnregisterSpecific(adapter)
	}))

	// 2. Inicia o servidor de teste
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 3. Conecta um cliente WebSocket (simulando a extensão Chrome)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("falha ao conectar no websocket: %v", err)
	}
	defer ws.Close()

	// 4. Goroutine para responder aos comandos que chegarem via WebSocket (simula o browser)
	go func() {
		for {
			var msg domain.BridgeMessage
			if err := websocket.JSON.Receive(ws, &msg); err != nil {
				return
			}

			// Se o comando for 'hover', envia uma resposta fake de sucesso
			if msg.Tool == "hover" {
				resp := domain.BridgeMessage{
					ID:     msg.ID,
					Tool:   msg.Tool,
					Result: json.RawMessage(`{"success": true, "message": "Hovered from WebSocket client"}`),
				}
				_ = websocket.JSON.Send(ws, resp)
			}
		}
	}()

	// 5. Faz uma requisição MCP via POST que exige o browser (hover)
	// Adicionamos um retry loop ou sleep curto para garantir que o hub registrou a conexão
	var resp *http.Response
	for i := 0; i < 5; i++ {
		reqBody := `{"jsonrpc": "2.0", "id": 101, "method": "tools/call", "params": {"name": "hover", "arguments": {"selector": "#submit-btn"}}}`
		resp, err = http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("falha no POST MCP: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			break
		}
		resp.Body.Close()
		time.Sleep(100 * time.Millisecond)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code inesperado: %d", resp.StatusCode)
	}

	// 6. Valida a resposta final do MCP
	var mcpResp domain.MCPMessage
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		t.Fatalf("falha ao decodificar resposta MCP: %v", err)
	}

	if mcpResp.Error != nil {
		t.Fatalf("erro no MCP: %v", mcpResp.Error)
	}

	resultMap, ok := mcpResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("resultado MCP não é um map: %T", mcpResp.Result)
	}

	content := resultMap["content"].([]any)
	item := content[0].(map[string]any)
	text := item["text"].(string)

	if !strings.Contains(text, "Hovered from WebSocket client") {
		t.Errorf("texto da resposta inesperado: %q", text)
	}
}
