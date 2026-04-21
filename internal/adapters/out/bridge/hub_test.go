package bridge

import (
	"context"
	"strings"
	"sync"
	"testing"

	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/ports"
)

func TestBridgeHub_Register_ClosesPrevious(t *testing.T) {
	hub := NewBridgeHub()

	closed1 := false
	mock1 := &ports.MockBrowserPort{
		CloseFunc: func() error {
			closed1 = true
			return nil
		},
	}

	closed2 := false
	mock2 := &ports.MockBrowserPort{
		CloseFunc: func() error {
			closed2 = true
			return nil
		},
	}

	// Registra o primeiro
	hub.Register(mock1)
	if closed1 {
		t.Error("mock1 should not be closed yet")
	}

	// Registra o segundo - deve fechar o primeiro
	hub.Register(mock2)
	if !closed1 {
		t.Error("mock1 should have been closed")
	}
	if closed2 {
		t.Error("mock2 should not be closed yet")
	}

	// Unregister - deve fechar o segundo
	hub.Unregister()
	if !closed2 {
		t.Error("mock2 should have been closed")
	}
}

func TestBridgeHub_ExecuteCommand_ErrorWhenNoActive(t *testing.T) {
	hub := NewBridgeHub()
	ctx := context.Background()
	msg := domain.BridgeMessage{ID: "1"}

	_, err := hub.ExecuteCommand(ctx, msg)
	if err == nil {
		t.Fatal("Expected error when no active adapter, got nil")
	}

	expectedMsg := "browser: nenhuma conexão ativa com o navegador"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing %q, got %q", expectedMsg, err.Error())
	}
}

func TestBridgeHub_ThreadSafety(t *testing.T) {
	hub := NewBridgeHub()
	ctx := context.Background()
	msg := domain.BridgeMessage{ID: "test"}

	const workers = 50
	const iterations = 100
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	// Goroutines registrando adaptadores
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				mock := &ports.MockBrowserPort{
					ExecuteCommandFunc: func(ctx context.Context, m domain.BridgeMessage) (domain.BridgeMessage, error) {
						return domain.BridgeMessage{ID: m.ID, Result: []byte(`"ok"`)}, nil
					},
					CloseFunc: func() error { return nil },
				}
				hub.Register(mock)
			}
		}(i)
	}

	// Goroutines executando comandos
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = hub.ExecuteCommand(ctx, msg)
			}
		}(i)
	}

	wg.Wait()
}

func TestBridgeHub_Unregister_ResetsToNil(t *testing.T) {
	hub := NewBridgeHub()
	mock := &ports.MockBrowserPort{
		CloseFunc: func() error { return nil },
	}

	hub.Register(mock)
	hub.Unregister()

	_, err := hub.ExecuteCommand(context.Background(), domain.BridgeMessage{})
	if err == nil {
		t.Error("Expected error after unregistering")
	}
}

func TestBridgeHub_Close(t *testing.T) {
	hub := NewBridgeHub()
	closed := false
	mock := &ports.MockBrowserPort{
		CloseFunc: func() error {
			closed = true
			return nil
		},
	}

	hub.Register(mock)
	err := hub.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !closed {
		t.Error("Adapter was not closed")
	}
}
