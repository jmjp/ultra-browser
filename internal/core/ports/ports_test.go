package ports_test

import (
	"context"
	"encoding/json"
	"testing"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

// TestInterfaceCompliance garante que os mocks satisfaçam as interfaces.
func TestInterfaceCompliance(t *testing.T) {
	var _ ports.MCPService = (*ports.MockMCPService)(nil)
	var _ ports.BrowserPort = (*ports.MockBrowserPort)(nil)
}

// TestMockFunctionality valida o comportamento básico do mock.
func TestMockFunctionality(t *testing.T) {
	ctx := context.Background()

	t.Run("MockMCPService", func(t *testing.T) {
		mcp := &ports.MockMCPService{
			ListToolsFunc: func(ctx context.Context) ([]domain.Tool, error) {
				return []domain.Tool{{Name: "test-tool"}}, nil
			},
		}

		tools, err := mcp.ListTools(ctx)
		if err != nil {
			t.Fatalf("ListTools falhou: %v", err)
		}
		if len(tools) != 1 || tools[0].Name != "test-tool" {
			t.Errorf("Ferramenta inesperada retornada")
		}
	})

	t.Run("MockBrowserPort", func(t *testing.T) {
		bp := &ports.MockBrowserPort{
			ExecuteCommandFunc: func(ctx context.Context, msg domain.BridgeMessage) (domain.BridgeMessage, error) {
				return domain.BridgeMessage{ID: msg.ID, Result: json.RawMessage(`"success"`)}, nil
			},
		}

		msg := domain.BridgeMessage{ID: "1", Tool: "navigate"}
		resp, err := bp.ExecuteCommand(ctx, msg)
		if err != nil {
			t.Fatalf("ExecuteCommand falhou: %v", err)
		}
		if resp.ID != "1" {
			t.Errorf("ID da mensagem de resposta incorreto")
		}
	})
}
