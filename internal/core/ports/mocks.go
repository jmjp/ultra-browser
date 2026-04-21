package ports

import (
	"context"
	"encoding/json"
	"os"

	"ultra-browser/internal/core/domain"
)

// MockMCPService implementa a interface MCPService para uso em testes de outros pacotes.
type MockMCPService struct {
	ListToolsFunc func(ctx context.Context) ([]domain.Tool, error)
	CallToolFunc  func(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error)
}

func (m *MockMCPService) ListTools(ctx context.Context) ([]domain.Tool, error) {
	if m.ListToolsFunc != nil {
		return m.ListToolsFunc(ctx)
	}
	return nil, nil
}

func (m *MockMCPService) CallTool(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
	if m.CallToolFunc != nil {
		return m.CallToolFunc(ctx, name, params)
	}
	return nil, nil
}

// MockBrowserPort implementa a interface BrowserPort para uso em testes de outros pacotes.
type MockBrowserPort struct {
	ExecuteCommandFunc func(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error)
	ReadEventsFunc     func(ctx context.Context) (<-chan domain.BridgeMessage, error)
	CaptureNodeFunc    func(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error)
	TypeTextFunc       func(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error)
	WaitForElementFunc func(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error)
	GetValueFunc       func(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error)
	SelectOptionFunc   func(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error)
	UploadFileFunc     func(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error)
	ScrollFunc         func(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error)
	HoverFunc          func(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error)
	SwitchTabFunc      func(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error)
	ScreenshotFunc     func(ctx context.Context) (domain.CaptureNodeResponse, error)
	CloseFunc          func() error
}

func (m *MockBrowserPort) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockBrowserPort) Screenshot(ctx context.Context) (domain.CaptureNodeResponse, error) {
	if m.ScreenshotFunc != nil {
		return m.ScreenshotFunc(ctx)
	}
	return domain.CaptureNodeResponse{}, nil
}

func (m *MockBrowserPort) ExecuteCommand(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
	if m.ExecuteCommandFunc != nil {
		return m.ExecuteCommandFunc(ctx, msg)
	}
	return domain.BridgeMessage{}, nil
}

func (m *MockBrowserPort) ReadEvents(ctx context.Context) (<-chan domain.BridgeMessage, error) {
	if m.ReadEventsFunc != nil {
		return m.ReadEventsFunc(ctx)
	}
	return nil, nil
}

func (m *MockBrowserPort) CaptureNode(ctx context.Context, req domain.CaptureNodeRequest) (domain.CaptureNodeResponse, error) {
	if m.CaptureNodeFunc != nil {
		return m.CaptureNodeFunc(ctx, req)
	}
	return domain.CaptureNodeResponse{}, nil
}

func (m *MockBrowserPort) TypeText(ctx context.Context, req domain.TypeTextRequest) (domain.CommonResponse, error) {
	if m.TypeTextFunc != nil {
		return m.TypeTextFunc(ctx, req)
	}
	return domain.CommonResponse{}, nil
}

func (m *MockBrowserPort) WaitForElement(ctx context.Context, req domain.WaitForElementRequest) (domain.CommonResponse, error) {
	if m.WaitForElementFunc != nil {
		return m.WaitForElementFunc(ctx, req)
	}
	return domain.CommonResponse{}, nil
}

func (m *MockBrowserPort) GetValue(ctx context.Context, req domain.GetValueRequest) (domain.GetValueResponse, error) {
	if m.GetValueFunc != nil {
		return m.GetValueFunc(ctx, req)
	}
	return domain.GetValueResponse{}, nil
}

func (m *MockBrowserPort) SelectOption(ctx context.Context, req domain.SelectOptionRequest) (domain.CommonResponse, error) {
	if m.SelectOptionFunc != nil {
		return m.SelectOptionFunc(ctx, req)
	}
	return domain.CommonResponse{}, nil
}

func (m *MockBrowserPort) UploadFile(ctx context.Context, req domain.UploadFileRequest, content []byte) (domain.CommonResponse, error) {
	if m.UploadFileFunc != nil {
		return m.UploadFileFunc(ctx, req, content)
	}
	return domain.CommonResponse{}, nil
}

func (m *MockBrowserPort) Scroll(ctx context.Context, req domain.ScrollRequest) (domain.CommonResponse, error) {
	if m.ScrollFunc != nil {
		return m.ScrollFunc(ctx, req)
	}
	return domain.CommonResponse{}, nil
}

func (m *MockBrowserPort) Hover(ctx context.Context, req domain.HoverRequest) (domain.CommonResponse, error) {
	if m.HoverFunc != nil {
		return m.HoverFunc(ctx, req)
	}
	return domain.CommonResponse{}, nil
}

func (m *MockBrowserPort) SwitchTab(ctx context.Context, req domain.SwitchTabRequest) (domain.CommonResponse, error) {
	if m.SwitchTabFunc != nil {
		return m.SwitchTabFunc(ctx, req)
	}
	return domain.CommonResponse{}, nil
}

// MockFileSystemPort implementa a interface FileSystemPort para uso em testes de outros pacotes.
type MockFileSystemPort struct {
	WriteFileFunc func(ctx context.Context, path string, data []byte) error
	StatFunc      func(ctx context.Context, path string) (os.FileInfo, error)
	ReadFileFunc  func(ctx context.Context, path string) ([]byte, error)
}

func (m *MockFileSystemPort) WriteFile(ctx context.Context, path string, data []byte) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(ctx, path, data)
	}
	return nil
}

func (m *MockFileSystemPort) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(ctx, path)
	}
	return nil, nil
}

func (m *MockFileSystemPort) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(ctx, path)
	}
	return nil, nil
}
