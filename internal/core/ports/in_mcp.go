package ports

import (
	"context"
	"encoding/json"

	"ultra-browser/internal/core/domain"
)

// MCPService define a interface inbound (primária) para o servidor MCP.
// Esta porta é responsável por gerenciar o ciclo de vida do serviço MCP e
// despachar as chamadas de ferramentas para o executor adequado.
type MCPService interface {
	// ListTools retorna todas as ferramentas registradas no sistema.
	ListTools(ctx context.Context) ([]domain.Tool, error)

	// CallTool executa uma ferramenta específica com os parâmetros fornecidos.
	CallTool(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error)
}
