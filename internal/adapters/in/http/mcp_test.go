package mcphttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mcphttp "ultra-browser/internal/adapters/in/http"
	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

func TestMCPServer_ServeHTTP(t *testing.T) {
	mockService := &ports.MockMCPService{}
	server := mcphttp.NewMCPServer(mockService)

	t.Run("Initialize", func(t *testing.T) {
		reqBody := `{"jsonrpc": "2.0", "id": 1, "method": "initialize"}`
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

		if resp.ID.(float64) != 1 {
			t.Errorf("expected id 1, got %v", resp.ID)
		}

		result := resp.Result.(map[string]any)
		if result["protocolVersion"] != "2024-11-05" {
			t.Errorf("expected protocolVersion 2024-11-05, got %v", result["protocolVersion"])
		}
	})

	t.Run("ToolsList", func(t *testing.T) {
		mockService.ListToolsFunc = func(ctx context.Context) ([]domain.Tool, error) {
			return []domain.Tool{
				{Name: "list-tabs", Description: "Lists all tabs"},
			}, nil
		}

		reqBody := `{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}`
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

		result := resp.Result.(map[string]any)
		tools := result["tools"].([]any)
		if len(tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(tools))
		}
		tool := tools[0].(map[string]any)
		if tool["name"] != "list-tabs" {
			t.Errorf("expected tool name list-tabs, got %v", tool["name"])
		}
	})

	t.Run("ToolsCall", func(t *testing.T) {
		mockService.CallToolFunc = func(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
			if name != "list-tabs" {
				return nil, errors.New("tool not found")
			}
			return json.RawMessage(`{"tabs": []}`), nil
		}

		reqBody := `{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "list-tabs", "arguments": {}}}`
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

		if resp.Error != nil {
			t.Errorf("expected no error, got %v", resp.Error)
		}

		result := resp.Result.(map[string]any)
		if _, ok := result["tabs"]; !ok {
			t.Errorf("expected result to contain 'tabs', got %v", result)
		}
	})

	t.Run("SSEHeader", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
		req.Header.Set("Accept", "text/event-stream")
		w := httptest.NewRecorder()

		// Cancela o contexto após um breve momento para permitir que o handler retorne
		cancel()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if contentType := w.Header().Get("Content-Type"); contentType != "text/event-stream" {
			t.Errorf("expected content-type text/event-stream, got %s", contentType)
		}

		body, _ := io.ReadAll(w.Body)
		expectedPrefix := "event: endpoint\ndata: /mcp\n\n"
		if string(body) != expectedPrefix {
			t.Errorf("expected body %q, got %q", expectedPrefix, string(body))
		}
	})

	t.Run("MethodNotAllowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		reqBody := `{"invalid": "json"`
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(reqBody))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}
