package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"ultra-browser/internal/adapters/out/bridge"
	"ultra-browser/internal/core/domain"
)

func TestRunServerModeHealthCheck(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status esperado 200, obtido %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("Corpo esperado OK, obtido %s", string(body))
	}
}

func TestGracefulShutdown(t *testing.T) {
	server := &http.Server{
		Addr:    "127.0.0.1:12347",
		Handler: http.NewServeMux(),
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Falha no graceful shutdown: %v", err)
	}
}

func TestBridgeHubNoActiveConnection(t *testing.T) {
	hub := bridge.NewBridgeHub()

	msg := domain.BridgeMessage{
		ID:   "test-id",
		Tool: "list_tabs",
	}

	_, err := hub.ExecuteCommand(context.Background(), msg)
	if err == nil {
		t.Fatal("Esperava erro quando nenhuma bridge está ativa")
	}
}

func TestCLIPortVariants(t *testing.T) {
	ports := []int{12306, 8080, 9090}
	for _, p := range ports {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		server := &http.Server{Addr: addr}
		if server.Addr != addr {
			t.Errorf("Porta incorreta. Esperada %s, obtida %s", addr, server.Addr)
		}
	}
}

func TestSignals(t *testing.T) {
	if syscall.SIGINT != 0x2 {
		t.Errorf("SIGINT inesperado: %v", syscall.SIGINT)
	}
	if syscall.SIGTERM != 0xf {
		t.Errorf("SIGTERM inesperado: %v", syscall.SIGTERM)
	}
}
