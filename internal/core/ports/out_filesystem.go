package ports

import (
	"context"
	"os"
)

// FileSystemPort define a interface outbound para operações de sistema de arquivos.
type FileSystemPort interface {
	// WriteFile grava dados em um arquivo no caminho especificado.
	WriteFile(ctx context.Context, path string, data []byte) error
	// Stat retorna informações sobre o arquivo no caminho especificado.
	Stat(ctx context.Context, path string) (os.FileInfo, error)
	// ReadFile lê o conteúdo de um arquivo do sistema de arquivos local.
	ReadFile(ctx context.Context, path string) ([]byte, error)
}
