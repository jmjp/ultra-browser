package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

func TestMCPService_ListTools(t *testing.T) {
	mockBrowser := &ports.MockBrowserPort{}
	mockFS := &ports.MockFileSystemPort{}
	service := NewMCPService(mockBrowser, mockFS)

	tools, err := service.ListTools(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedTools := []string{
		"list_tabs", "navigate", "screenshot", "get_content", "click", "execute_script", "capture_node",
		"type_text", "wait_for_element", "get_value", "select_option", "upload_file", "scroll", "hover", "switch_tab",
	}
	if len(tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(tools))
	}

	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	for _, name := range expectedTools {
		if !toolMap[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}
}

func TestMCPService_CallTool(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		params         json.RawMessage
		mockResponse   domain.BridgeMessage
		mockError      error
		expectedResult string
		expectedError  string
	}{
		{
			name:     "Success - list_tabs (Text Format)",
			toolName: "list_tabs",
			params:   json.RawMessage(`{}`),
			mockResponse: domain.BridgeMessage{
				Result: json.RawMessage(`{"tabs": []}`),
			},
			expectedResult: `{"content":[{"type":"text","text":"{\"tabs\": []}"}]}`,
		},
		{
			name:     "Success - screenshot (Image Format)",
			toolName: "screenshot",
			params:   json.RawMessage(`{}`),
			mockResponse: domain.BridgeMessage{
				Result: json.RawMessage(`{"base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKma1gAAAABJRU5ErkJggg=="}`),
			},
			expectedResult: `{"content":[{"type":"image","data":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKma1gAAAABJRU5ErkJggg==","mimeType":"image/png"}]}`,
		},
		{
			name:          "Error - Tool Not Found",
			toolName:      "unknown_tool",
			params:        json.RawMessage(`{}`),
			expectedError: "tool not found: unknown_tool",
		},
		{
			name:     "Error - Bridge returns error",
			toolName: "navigate",
			params:   json.RawMessage(`{"url": "https://example.com"}`),
			mockResponse: domain.BridgeMessage{
				Error: "navigation failed",
			},
			expectedError: "navigation failed",
		},
		{
			name:          "Error - Bridge execution error (Retries)",
			toolName:      "navigate", // Mudar de screenshot para navigate pois screenshot agora é caso especial
			params:        json.RawMessage(`{"url": "https://example.com"}`),
			mockError:     errors.New("connection lost"),
			expectedError: "bridge execution failed after 3 attempts: connection lost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mockFS := &ports.MockFileSystemPort{}
			mockBrowser := &ports.MockBrowserPort{
				ExecuteCommandFunc: func(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
					callCount++
					if tt.mockError != nil {
						return domain.BridgeMessage{}, tt.mockError
					}
					// Verify if tool name is correct in request
					if msg.Tool != tt.toolName {
						t.Errorf("expected tool %s, got %s", tt.toolName, msg.Tool)
					}
					// Ensure ID is generated
					if msg.ID == "" {
						t.Error("expected ID to be generated, got empty")
					}
					return tt.mockResponse, nil
				},
				ScreenshotFunc: func(ctx context.Context) (domain.CaptureNodeResponse, error) {
					callCount++
					if tt.mockError != nil {
						return domain.CaptureNodeResponse{}, tt.mockError
					}
					// Desempacota o base64 do mockResponse se existir
					var data struct {
						Base64 string `json:"base64"`
					}
					if len(tt.mockResponse.Result) > 0 {
						json.Unmarshal(tt.mockResponse.Result, &data)
					}

					return domain.CaptureNodeResponse{
						Success: true,
						Content: data.Base64,
						Format:  "png",
					}, nil
				},
			}

			service := NewMCPService(mockBrowser, mockFS)
			result, err := service.CallTool(context.Background(), tt.toolName, tt.params)

			if tt.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedError)
				}
				if err.Error() != tt.expectedError {
					t.Errorf("expected error %q, got %q", tt.expectedError, err.Error())
				}
				if tt.mockError != nil && callCount != 3 {
					t.Errorf("expected 3 retries, got %d", callCount)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if string(result) != tt.expectedResult {
				t.Errorf("expected result %q, got %q", tt.expectedResult, string(result))
			}
		})
	}
}

func TestMCPService_CallTool_CaptureNode(t *testing.T) {
	mockBrowser := &ports.MockBrowserPort{}
	mockFS := &ports.MockFileSystemPort{}
	service := NewMCPService(mockBrowser, mockFS)

	ctx := context.Background()
	params := json.RawMessage(`{"selector": "#main", "format": "png", "path": "/tmp/test.png"}`)

	t.Run("Success", func(t *testing.T) {
		calledFS := false
		mockBrowser.CaptureNodeFunc = func(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
			// Simula a resposta do browser com os dados capturados
			return domain.CaptureNodeResponse{
				Success: true,
				Content: "YmFzZTY0ZGF0YQ==", // "base64data" em base64
				Format:  "png",
			}, nil
		}

		mockFS.WriteFileFunc = func(ctx context.Context, path string, data []byte) error {
			calledFS = true
			if path != "/tmp/test.png" {
				t.Errorf("expected path /tmp/test.png, got %s", path)
			}
			// Verifica se os dados recebidos do browser foram passados para o filesystem (decodificados)
			if string(data) != "base64data" {
				t.Errorf("expected data base64data, got %s", string(data))
			}
			return nil
		}

		// A orquestração deve chamar o browser e depois o filesystem
		_, err := service.CallTool(ctx, "capture_node", params)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !calledFS {
			t.Error("expected FileSystem.WriteFile to be called, but it was not")
		}
	})

	t.Run("BrowserFailure", func(t *testing.T) {
		mockBrowser.CaptureNodeFunc = func(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
			return domain.CaptureNodeResponse{}, errors.New("browser capture failed")
		}

		_, err := service.CallTool(ctx, "capture_node", params)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("FilesystemFailure", func(t *testing.T) {
		mockBrowser.CaptureNodeFunc = func(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
			return domain.CaptureNodeResponse{
				Success: true,
				Content: "ZGF0YQ==", // "data"
				Format:  "png",
			}, nil
		}

		mockFS.WriteFileFunc = func(ctx context.Context, path string, data []byte) error {
			return errors.New("disk full")
		}

		_, err := service.CallTool(ctx, "capture_node", params)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestMCPService_CallTool_NewTools(t *testing.T) {
	mockBrowser := &ports.MockBrowserPort{}
	mockFS := &ports.MockFileSystemPort{}
	service := NewMCPService(mockBrowser, mockFS)
	ctx := context.Background()

	t.Run("TypeText_Success", func(t *testing.T) {
		mockBrowser.TypeTextFunc = func(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
			if req.Selector != "#input" || req.Text != "hello" {
				return domain.CommonResponse{Success: false}, nil
			}
			return domain.CommonResponse{Success: true, Message: "Typed hello"}, nil
		}
		params := json.RawMessage(`{"selector": "#input", "text": "hello"}`)
		resp, err := service.CallTool(ctx, "type_text", params)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(string(resp), "Typed hello") {
			t.Errorf("expected response to contain 'Typed hello', got %s", string(resp))
		}
	})

	t.Run("WaitForElement_Success", func(t *testing.T) {
		mockBrowser.WaitForElementFunc = func(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
			return domain.CommonResponse{Success: true, Message: "Element appeared"}, nil
		}
		params := json.RawMessage(`{"selector": "#ready", "timeout": 1000}`)
		resp, err := service.CallTool(ctx, "wait_for_element", params)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(string(resp), "Element appeared") {
			t.Errorf("expected response to contain 'Element appeared', got %s", string(resp))
		}
	})

	t.Run("GetValue_Success", func(t *testing.T) {
		mockBrowser.GetValueFunc = func(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
			return domain.GetValueResponse{Success: true, Value: "secret"}, nil
		}
		params := json.RawMessage(`{"selector": "#pass"}`)
		resp, err := service.CallTool(ctx, "get_value", params)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(string(resp), "secret") {
			t.Errorf("expected response to contain 'secret', got %s", string(resp))
		}
	})

	t.Run("SelectOption_Success", func(t *testing.T) {
		mockBrowser.SelectOptionFunc = func(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
			return domain.CommonResponse{Success: true, Message: "Selected option"}, nil
		}
		params := json.RawMessage(`{"selector": "#select", "value": "opt1"}`)
		resp, err := service.CallTool(ctx, "select_option", params)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(string(resp), "Selected option") {
			t.Errorf("expected response to contain 'Selected option', got %s", string(resp))
		}
	})

	t.Run("UploadFile_Success", func(t *testing.T) {
		mockFS.ReadFileFunc = func(ctx context.Context, path string) ([]byte, error) {
			return []byte("file content"), nil
		}
		mockBrowser.UploadFileFunc = func(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
			if string(content) != "file content" {
				return domain.CommonResponse{Success: false}, nil
			}
			return domain.CommonResponse{Success: true, Message: "File uploaded"}, nil
		}
		params := json.RawMessage(`{"selector": "#file", "path": "/tmp/test.txt"}`)
		resp, err := service.CallTool(ctx, "upload_file", params)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(string(resp), "File uploaded") {
			t.Errorf("expected response to contain 'File uploaded', got %s", string(resp))
		}
	})

	t.Run("UploadFile_FileSystemError", func(t *testing.T) {
		mockFS.ReadFileFunc = func(ctx context.Context, path string) ([]byte, error) {
			return nil, errors.New("file not found")
		}
		params := json.RawMessage(`{"selector": "#file", "path": "/tmp/test.txt"}`)
		_, err := service.CallTool(ctx, "upload_file", params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("expected error to contain 'file not found', got %v", err)
		}
	})

	t.Run("Scroll_Success", func(t *testing.T) {
		mockBrowser.ScrollFunc = func(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
			return domain.CommonResponse{Success: true, Message: "Scrolled"}, nil
		}
		params := json.RawMessage(`{"x": 0, "y": 100}`)
		resp, err := service.CallTool(ctx, "scroll", params)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(string(resp), "Scrolled") {
			t.Errorf("expected response to contain 'Scrolled', got %s", string(resp))
		}
	})

	t.Run("Hover_Success", func(t *testing.T) {
		mockBrowser.HoverFunc = func(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
			return domain.CommonResponse{Success: true, Message: "Hovered"}, nil
		}
		params := json.RawMessage(`{"selector": "#btn"}`)
		resp, err := service.CallTool(ctx, "hover", params)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(string(resp), "Hovered") {
			t.Errorf("expected response to contain 'Hovered', got %s", string(resp))
		}
	})

	t.Run("BrowserCommunicationError", func(t *testing.T) {
		mockBrowser.TypeTextFunc = func(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
			return domain.CommonResponse{}, errors.New("bridge error")
		}
		params := json.RawMessage(`{"selector": "#input", "text": "hello"}`)
		_, err := service.CallTool(ctx, "type_text", params)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "bridge error") {
			t.Errorf("expected error to contain 'bridge error', got %v", err)
		}
	})
}
