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

	mcphttp "ultra-browser/internal/adapters/in/http"
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

		if resp.Error == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(strings.ToLower(resp.Error.Message), "validation") {
			t.Errorf("expected validation error message, got %q", resp.Error.Message)
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
