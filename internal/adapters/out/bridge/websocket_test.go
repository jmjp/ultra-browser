package bridge

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"ultra-browser/internal/core/domain"

	"golang.org/x/net/websocket"
)

func TestWebSocketAdapter_ExecuteCommand(t *testing.T) {
	// 1. Mock WebSocket Server
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		for {
			var msg domain.BridgeMessage
			if err := websocket.JSON.Receive(ws, &msg); err != nil {
				return
			}

			// Simular processamento e resposta
			response := domain.BridgeMessage{
				ID:     msg.ID,
				Result: json.RawMessage(`{"success": true}`),
			}
			websocket.JSON.Send(ws, response)
		}
	}))
	defer server.Close()

	// 2. Initialize Adapter
	wsURL := "ws" + server.URL[len("http"):]
	adapter, err := NewWebSocketAdapter(wsURL)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	// 3. Test ExecuteCommand
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := domain.BridgeMessage{
		ID:   "test-id",
		Tool: "test-tool",
	}

	resp, err := adapter.ExecuteCommand(ctx, req)
	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}

	if resp.ID != req.ID {
		t.Errorf("Expected ID %s, got %s", req.ID, resp.ID)
	}

	var result map[string]bool
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !result["success"] {
		t.Errorf("Expected success true, got false")
	}
}

func TestWebSocketAdapter_ReadEvents(t *testing.T) {
	// 1. Mock WebSocket Server
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		// Enviar um evento espontâneo
		event := domain.BridgeMessage{
			ID:   "",
			Tool: "on_event",
			Params: json.RawMessage(`{"data": "hello"}`),
		}
		websocket.JSON.Send(ws, event)
		
		// Esperar um pouco para não fechar a conexão imediatamente
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// 2. Initialize Adapter
	wsURL := "ws" + server.URL[len("http"):]
	adapter, err := NewWebSocketAdapter(wsURL)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	// 3. Test ReadEvents
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, err := adapter.ReadEvents(ctx)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	select {
	case event := <-events:
		if event.Tool != "on_event" {
			t.Errorf("Expected tool on_event, got %s", event.Tool)
		}
	case <-ctx.Done():
		t.Errorf("Timeout waiting for event")
	}
}
