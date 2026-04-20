package filesystem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"ultra-browser/internal/core/ports"
)

// LocalFileSystemAdapter implementa a interface ports.FileSystemPort.
type LocalFileSystemAdapter struct {
	rootDir string
}

// Assegura que LocalFileSystemAdapter implementa ports.FileSystemPort.
var _ ports.FileSystemPort = (*LocalFileSystemAdapter)(nil)

// NewLocalFileSystemAdapter cria uma nova instância do adaptador com um diretório raiz restrito.
func NewLocalFileSystemAdapter(rootDir string) *LocalFileSystemAdapter {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		// Se não conseguir resolver o absoluto, usa o original
		absRoot = rootDir
	}
	return &LocalFileSystemAdapter{
		rootDir: absRoot,
	}
}

// WriteFile grava dados em um arquivo dentro do diretório raiz.
func (a *LocalFileSystemAdapter) WriteFile(ctx context.Context, path string, data []byte) error {
	fullPath, err := a.validatePath(path)
	if err != nil {
		return err
	}

	// Garante que o diretório pai existe
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(fullPath, data, 0644)
}

// Stat retorna informações sobre o arquivo.
func (a *LocalFileSystemAdapter) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	fullPath, err := a.validatePath(path)
	if err != nil {
		return nil, err
	}

	return os.Stat(fullPath)
}

// ReadFile lê o conteúdo de um arquivo do sistema de arquivos local.
func (a *LocalFileSystemAdapter) ReadFile(ctx context.Context, path string) ([]byte, error) {
	fullPath, err := a.validatePath(path)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(fullPath)
}

// validatePath limpa o caminho e garante que ele esteja dentro do rootDir.
func (a *LocalFileSystemAdapter) validatePath(path string) (string, error) {
	// 1. Limpa o caminho para resolver .. e caminhos relativos
	cleanPath := filepath.Clean(path)

	var fullPath string
	if filepath.IsAbs(cleanPath) {
		fullPath = cleanPath
	} else {
		fullPath = filepath.Join(a.rootDir, cleanPath)
	}

	// 2. Resolve o caminho absoluto final para comparação segura
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// 3. Verifica se o caminho absoluto começa com o rootDir absoluto
	// Usamos strings.HasPrefix após garantir que ambos são caminhos absolutos
	if !strings.HasPrefix(absPath, a.rootDir) {
		return "", errors.New("security error: path is outside allowed root directory")
	}

	return absPath, nil
}
