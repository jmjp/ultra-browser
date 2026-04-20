package domain_test

import (
	"encoding/json"
	"testing"
	"ultra-browser/internal/core/domain"
)

func TestNewBridgeRequest(t *testing.T) {
	params := map[string]string{"url": "https://google.com"}
	msg, err := domain.NewBridgeRequest("1", "chrome_navigate", params)
	if err != nil {
		t.Fatalf("Erro inesperado: %v", err)
	}

	if msg.ID != "1" {
		t.Errorf("ID esperado 1, obteve %s", msg.ID)
	}
	if msg.Tool != "chrome_navigate" {
		t.Errorf("Tool esperada chrome_navigate, obteve %s", msg.Tool)
	}

	var decodedParams map[string]string
	if err := json.Unmarshal(msg.Params, &decodedParams); err != nil {
		t.Fatalf("Erro ao decodificar params: %v", err)
	}
	if decodedParams["url"] != "https://google.com" {
		t.Errorf("URL esperada https://google.com, obteve %s", decodedParams["url"])
	}
}

func TestNewBridgeResponse(t *testing.T) {
	result := map[string]int{"tabId": 123}
	msg, err := domain.NewBridgeResponse("1", result)
	if err != nil {
		t.Fatalf("Erro inesperado: %v", err)
	}

	if msg.ID != "1" {
		t.Errorf("ID esperado 1, obteve %s", msg.ID)
	}

	var decodedResult map[string]int
	if err := json.Unmarshal(msg.Result, &decodedResult); err != nil {
		t.Fatalf("Erro ao decodificar result: %v", err)
	}
	if decodedResult["tabId"] != 123 {
		t.Errorf("tabId esperado 123, obteve %d", decodedResult["tabId"])
	}
}

func TestMCPMessage_JSON(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	var msg domain.MCPMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("Erro ao unmarshal MCPMessage: %v", err)
	}

	if msg.JSONRPC != "2.0" {
		t.Errorf("JSONRPC esperado 2.0, obteve %s", msg.JSONRPC)
	}
	if msg.Method != "tools/list" {
		t.Errorf("Method esperado tools/list, obteve %s", msg.Method)
	}
}

func TestCaptureNodeRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.CaptureNodeRequest
		wantErr bool
	}{
		{
			name: "Valid PNG request",
			req: domain.CaptureNodeRequest{
				Selector: "#main-content",
				Format:   "png",
				Path:     "C:\\tmp\\node.png",
			},
			wantErr: false,
		},
		{
			name: "Valid HTML request",
			req: domain.CaptureNodeRequest{
				Selector: ".article",
				Format:   "html",
				Path:     "/tmp/node.html",
			},
			wantErr: false,
		},
		{
			name: "Missing selector",
			req: domain.CaptureNodeRequest{
				Format: "png",
				Path:   "/tmp/node.png",
			},
			wantErr: true,
		},
		{
			name: "Invalid format",
			req: domain.CaptureNodeRequest{
				Selector: "div",
				Format:   "pdf",
				Path:     "/tmp/node.pdf",
			},
			wantErr: true,
		},
		{
			name: "Relative path (Windows)",
			req: domain.CaptureNodeRequest{
				Selector: "div",
				Format:   "png",
				Path:     "tmp\\node.png",
			},
			wantErr: false,
		},
		{
			name: "Relative path (Unix)",
			req: domain.CaptureNodeRequest{
				Selector: "div",
				Format:   "png",
				Path:     "tmp/node.png",
			},
			wantErr: false,
		},
		{
			name: "Inferred PNG request",
			req: domain.CaptureNodeRequest{
				Selector: "div",
				Path:     "/tmp/image.png",
			},
			wantErr: false,
		},
		{
			name: "Inferred HTML request",
			req: domain.CaptureNodeRequest{
				Selector: "div",
				Path:     "/tmp/page.html",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("CaptureNodeRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCaptureNodeResponse_JSON(t *testing.T) {
	resp := domain.CaptureNodeResponse{
		Success:  true,
		FilePath: "/tmp/captured.png",
		Message:  "Captured successfully",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Erro ao serializar CaptureNodeResponse: %v", err)
	}

	var decoded domain.CaptureNodeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Erro ao desserializar CaptureNodeResponse: %v", err)
	}

	if decoded.Success != resp.Success || decoded.FilePath != resp.FilePath || decoded.Message != resp.Message {
		t.Errorf("CaptureNodeResponse decodificado difere do original. Original: %+v, Decodificado: %+v", resp, decoded)
	}
}

func TestTypeTextRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.TypeTextRequest
		wantErr bool
	}{
		{"Valid request", domain.TypeTextRequest{Selector: "#input", Text: "hello"}, false},
		{"Missing selector", domain.TypeTextRequest{Text: "hello"}, true},
		{"Missing text", domain.TypeTextRequest{Selector: "#input"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWaitForElementRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.WaitForElementRequest
		wantErr bool
	}{
		{"Valid request", domain.WaitForElementRequest{Selector: "#btn", Timeout: 5000}, false},
		{"Missing selector", domain.WaitForElementRequest{Timeout: 5000}, true},
		{"Negative timeout", domain.WaitForElementRequest{Selector: "#btn", Timeout: -1}, true},
		{"Zero timeout is valid", domain.WaitForElementRequest{Selector: "#btn", Timeout: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetValueRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.GetValueRequest
		wantErr bool
	}{
		{"Valid request", domain.GetValueRequest{Selector: "#input"}, false},
		{"Missing selector", domain.GetValueRequest{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSelectOptionRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.SelectOptionRequest
		wantErr bool
	}{
		{"Valid request", domain.SelectOptionRequest{Selector: "#select", Value: "opt1"}, false},
		{"Missing selector", domain.SelectOptionRequest{Value: "opt1"}, true},
		{"Missing value", domain.SelectOptionRequest{Selector: "#select"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUploadFileRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.UploadFileRequest
		wantErr bool
	}{
		{"Valid absolute path (Unix)", domain.UploadFileRequest{Selector: "#file", Path: "/tmp/test.txt"}, false},
		{"Valid absolute path (Windows)", domain.UploadFileRequest{Selector: "#file", Path: "C:\\test.txt"}, false},
		{"Missing selector", domain.UploadFileRequest{Path: "/tmp/test.txt"}, true},
		{"Missing path", domain.UploadFileRequest{Selector: "#file"}, true},
		{"Relative path", domain.UploadFileRequest{Selector: "#file", Path: "test.txt"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestScrollRequest_Validation(t *testing.T) {
	intPtr := func(i int) *int { return &i }
	tests := []struct {
		name    string
		req     domain.ScrollRequest
		wantErr bool
	}{
		{"Valid selector", domain.ScrollRequest{Selector: "#footer"}, false},
		{"Valid X and Y", domain.ScrollRequest{X: intPtr(0), Y: intPtr(100)}, false},
		{"Missing all", domain.ScrollRequest{}, true},
		{"Missing Y", domain.ScrollRequest{X: intPtr(10)}, true},
		{"Missing X", domain.ScrollRequest{Y: intPtr(10)}, true},
		{"X, Y and Selector is valid", domain.ScrollRequest{X: intPtr(0), Y: intPtr(0), Selector: "#id"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHoverRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.HoverRequest
		wantErr bool
	}{
		{"Valid request", domain.HoverRequest{Selector: "#menu"}, false},
		{"Missing selector", domain.HoverRequest{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
