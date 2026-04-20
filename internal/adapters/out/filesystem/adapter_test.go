package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFileSystemAdapter_WriteFile(t *testing.T) {
	// Cria um diretório temporário para os testes
	tmpDir, err := os.MkdirTemp("", "ultra-browser-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve o caminho absoluto para evitar problemas com symlinks em alguns SOs
	absTmpDir, _ := filepath.Abs(tmpDir)
	adapter := NewLocalFileSystemAdapter(absTmpDir)
	ctx := context.Background()

	t.Run("Escrita básica em diretório temporário", func(t *testing.T) {
		filename := "test.txt"
		content := []byte("hello world")
		err := adapter.WriteFile(ctx, filename, content)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verifica se o arquivo existe e tem o conteúdo correto
		fullPath := filepath.Join(absTmpDir, filename)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("Failed to read created file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("Expected %s, got %s", string(content), string(data))
		}
	})

	t.Run("Proteção contra Path Traversal", func(t *testing.T) {
		// Tentativa de gravar fora do rootDir usando ../
		traversalPath := "../traversal_attack.txt"
		content := []byte("malicious content")
		err := adapter.WriteFile(ctx, traversalPath, content)
		
		if err == nil {
			t.Error("Expected error for path traversal, got nil")
		}

		// Verifica se o arquivo NÃO foi criado fora do diretório permitido
		// Tentamos prever onde o arquivo seria criado se a falha existisse
		parentDir := filepath.Dir(absTmpDir)
		outsidePath := filepath.Join(parentDir, "traversal_attack.txt")
		if _, err := os.Stat(outsidePath); err == nil {
			t.Errorf("Security breach: file created outside root directory at %s", outsidePath)
			os.Remove(outsidePath) // Limpa se criado
		}
	})

	t.Run("Criação automática de diretórios pais", func(t *testing.T) {
		nestedPath := "subdir/nested/deep/file.txt"
		content := []byte("nested content")
		err := adapter.WriteFile(ctx, nestedPath, content)
		if err != nil {
			t.Fatalf("Expected no error when creating nested path, got %v", err)
		}

		fullPath := filepath.Join(absTmpDir, nestedPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Nested file not created: %s", fullPath)
		}
		
		data, _ := os.ReadFile(fullPath)
		if string(data) != string(content) {
			t.Errorf("Content mismatch in nested file")
		}
	})

	t.Run("Caminho absoluto fora do root deve falhar", func(t *testing.T) {
		// No Windows, um caminho como C:\Windows\System32\...
		// No Linux/Mac, /etc/passwd
		// Vamos usar um caminho absoluto arbitrário que sabemos estar fora do tmpDir
		absPath := "/tmp/outside_test.txt"
		if filepath.Separator == '\\' {
			absPath = `C:\outside_test.txt`
		}

		err := adapter.WriteFile(ctx, absPath, []byte("data"))
		if err == nil {
			t.Error("Expected error for absolute path outside root, got nil")
		}
	})
}

func TestLocalFileSystemAdapter_ReadFile(t *testing.T) {
	// Cria um diretório temporário para os testes
	tmpDir, err := os.MkdirTemp("", "ultra-browser-read-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	absTmpDir, _ := filepath.Abs(tmpDir)
	adapter := NewLocalFileSystemAdapter(absTmpDir)
	ctx := context.Background()

	t.Run("Leitura bem-sucedida", func(t *testing.T) {
		filename := "read_test.txt"
		content := []byte("content to read")
		
		// Prepara o arquivo
		fullPath := filepath.Join(absTmpDir, filename)
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			t.Fatalf("Failed to prepare test file: %v", err)
		}

		data, err := adapter.ReadFile(ctx, filename)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("Expected %s, got %s", string(content), string(data))
		}
	})

	t.Run("Arquivo inexistente", func(t *testing.T) {
		_, err := adapter.ReadFile(ctx, "non_existent.txt")
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
			t.Errorf("Expected not exist error, got %v", err)
		}
	})

	t.Run("Proteção contra Path Traversal", func(t *testing.T) {
		// Tenta ler fora do rootDir
		_, err := adapter.ReadFile(ctx, "../any_file.txt")
		if err == nil {
			t.Error("Expected security error for path traversal, got nil")
		}
	})
}

func TestLocalFileSystemAdapter_Stat(t *testing.T) {
	// Cria um diretório temporário para os testes
	tmpDir, err := os.MkdirTemp("", "ultra-browser-stat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	absTmpDir, _ := filepath.Abs(tmpDir)
	adapter := NewLocalFileSystemAdapter(absTmpDir)
	ctx := context.Background()

	t.Run("Stat bem-sucedido", func(t *testing.T) {
		filename := "stat_test.txt"
		fullPath := filepath.Join(absTmpDir, filename)
		if err := os.WriteFile(fullPath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to prepare test file: %v", err)
		}

		info, err := adapter.Stat(ctx, filename)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if info.Name() != filename {
			t.Errorf("Expected name %s, got %s", filename, info.Name())
		}
	})

	t.Run("Stat em arquivo inexistente", func(t *testing.T) {
		_, err := adapter.Stat(ctx, "non_existent.txt")
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
		if !os.IsNotExist(err) {
			t.Errorf("Expected not exist error, got %v", err)
		}
	})

	t.Run("Stat com Path Traversal", func(t *testing.T) {
		_, err := adapter.Stat(ctx, "../../forbidden.txt")
		if err == nil {
			t.Error("Expected security error for path traversal, got nil")
		}
	})
}

func TestNewLocalFileSystemAdapter(t *testing.T) {
	t.Run("Caminho absoluto", func(t *testing.T) {
		dir := "test-dir"
		adapter := NewLocalFileSystemAdapter(dir)
		absDir, _ := filepath.Abs(dir)
		if adapter.rootDir != absDir {
			t.Errorf("Expected rootDir to be absolute: %s", adapter.rootDir)
		}
	})
}
