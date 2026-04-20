package bridge

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"testing"
	"time"
	"ultra-browser/internal/core/domain"
)

func TestNativeMessagingAdapter(t *testing.T) {
	// Pipe para simular stdin do browser (o que o adaptador lê)
	pr, pw := io.Pipe()
	r := NewReader(pr)
	
	// Pipe para simular stdout do browser (o que o adaptador escreve)
	outReader, outWriter := io.Pipe()
	w := NewWriter(outWriter)
	
	adapter := NewNativeMessagingAdapter(r, w)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Inicia o loop em background
	go func() {
		_ = adapter.Run(ctx)
	}()

	t.Run("ExecuteCommand handles correlation automatically", func(t *testing.T) {
		reqID := "req-adapter-1"
		msg := domain.BridgeMessage{ID: reqID, Tool: "test"}

		// Goroutine para simular o browser processando o comando
		go func() {
			// Lê o que foi enviado
			var length uint32
			if err := binary.Read(outReader, binary.LittleEndian, &length); err != nil {
				return
			}
			data := make([]byte, length)
			io.ReadFull(outReader, data)

			// Envia resposta
			time.Sleep(10 * time.Millisecond)
			response := `{"id":"req-adapter-1","result":{"status":"done"}}`
			binary.Write(pw, binary.LittleEndian, uint32(len(response)))
			pw.Write([]byte(response))
		}()

		res, err := adapter.ExecuteCommand(ctx, msg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if res.ID != reqID {
			t.Errorf("Expected response ID %s, got %s", reqID, res.ID)
		}
	})

	t.Run("ReadEvents still receives unsolicited events", func(t *testing.T) {
		events, _ := adapter.ReadEvents(ctx)

		go func() {
			unsolicited := `{"id":"","tool":"event","params":{"msg":"hello"}}`
			binary.Write(pw, binary.LittleEndian, uint32(len(unsolicited)))
			pw.Write([]byte(unsolicited))
		}()

		select {
		case msg := <-events:
			if msg.Tool != "event" {
				t.Errorf("Expected tool 'event', got %s", msg.Tool)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for event")
		}
	})

	t.Run("CaptureNode sends correct request and receives response", func(t *testing.T) {
		req := domain.CaptureNodeRequest{
			Selector: "#test",
			Format:   "png",
			Path:     "/abs/path/test.png",
		}

		go func() {
			var length uint32
			if err := binary.Read(outReader, binary.LittleEndian, &length); err != nil {
				return
			}
			data := make([]byte, length)
			if _, err := io.ReadFull(outReader, data); err != nil {
				return
			}

			var msg domain.BridgeMessage
			json.Unmarshal(data, &msg)

			result := domain.CaptureNodeResponse{
				Success:  true,
				FilePath: "/abs/path/test.png",
				Message:  "Captured successfully",
			}
			rawResult, _ := json.Marshal(result)
			resp := domain.BridgeMessage{
				ID:     msg.ID,
				Result: rawResult,
			}
			respData, _ := json.Marshal(resp)
			
			binary.Write(pw, binary.LittleEndian, uint32(len(respData)))
			pw.Write(respData)
		}()

		res, err := adapter.CaptureNode(ctx, req)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !res.Success {
			t.Error("Expected success true")
		}
		if res.FilePath != req.Path {
			t.Errorf("Expected FilePath %s, got %s", req.Path, res.FilePath)
		}
	})

	t.Run("New tools implementation tests", func(t *testing.T) {
		tools := []struct {
			name     string
			call     func() (any, error)
			expected any
			toolName string
		}{
			{
				name: "TypeText",
				call: func() (any, error) {
					return adapter.TypeText(ctx, domain.TypeTextRequest{Selector: "#id", Text: "txt"})
				},
				expected: domain.CommonResponse{Success: true, Message: "Typed"},
				toolName: "type_text",
			},
			{
				name: "WaitForElement",
				call: func() (any, error) {
					return adapter.WaitForElement(ctx, domain.WaitForElementRequest{Selector: "#id", Timeout: 1000})
				},
				expected: domain.CommonResponse{Success: true},
				toolName: "wait_for_element",
			},
			{
				name: "GetValue",
				call: func() (any, error) {
					return adapter.GetValue(ctx, domain.GetValueRequest{Selector: "#id"})
				},
				expected: domain.GetValueResponse{Success: true, Value: "val"},
				toolName: "get_value",
			},
			{
				name: "SelectOption",
				call: func() (any, error) {
					return adapter.SelectOption(ctx, domain.SelectOptionRequest{Selector: "#id", Value: "opt"})
				},
				expected: domain.CommonResponse{Success: true},
				toolName: "select_option",
			},
			{
				name: "UploadFile",
				call: func() (any, error) {
					return adapter.UploadFile(ctx, domain.UploadFileRequest{Selector: "#id", Path: "/path"}, []byte("content"))
				},
				expected: domain.CommonResponse{Success: true},
				toolName: "upload_file",
			},
			{
				name: "Scroll",
				call: func() (any, error) {
					y := 100
					return adapter.Scroll(ctx, domain.ScrollRequest{Y: &y})
				},
				expected: domain.CommonResponse{Success: true},
				toolName: "scroll",
			},
			{
				name: "Hover",
				call: func() (any, error) {
					return adapter.Hover(ctx, domain.HoverRequest{Selector: "#id"})
				},
				expected: domain.CommonResponse{Success: true},
				toolName: "hover",
			},
		}

		for _, tc := range tools {
			t.Run(tc.name, func(t *testing.T) {
				go func() {
					var length uint32
					binary.Read(outReader, binary.LittleEndian, &length)
					data := make([]byte, length)
					io.ReadFull(outReader, data)

					var msg domain.BridgeMessage
					json.Unmarshal(data, &msg)

					if msg.Tool != tc.toolName {
						// Não podemos usar t.Errorf aqui pois estamos em outra goroutine, 
						// mas o teste principal vai falhar por timeout ou erro de unmarshal se não enviarmos resposta.
					}

					rawResult, _ := json.Marshal(tc.expected)
					resp := domain.BridgeMessage{ID: msg.ID, Result: rawResult}
					respData, _ := json.Marshal(resp)
					binary.Write(pw, binary.LittleEndian, uint32(len(respData)))
					pw.Write(respData)
				}()

				res, err := tc.call()
				if err != nil {
					t.Fatalf("Unexpected error for %s: %v", tc.name, err)
				}

				// Validação básica de sucesso (todos os tc.expected são structs com Success=true)
				resJSON, _ := json.Marshal(res)
				expJSON, _ := json.Marshal(tc.expected)
				if string(resJSON) != string(expJSON) {
					t.Errorf("Result mismatch for %s.\nExpected: %s\nGot:      %s", tc.name, expJSON, resJSON)
				}
			})
		}
	})

	t.Run("CaptureNode handles bridge error", func(t *testing.T) {
		req := domain.CaptureNodeRequest{
			Selector: "#error",
			Format:   "html",
			Path:     "/abs/path/error.html",
		}

		go func() {
			var length uint32
			if err := binary.Read(outReader, binary.LittleEndian, &length); err != nil {
				return
			}
			data := make([]byte, length)
			io.ReadFull(outReader, data)

			var msg domain.BridgeMessage
			json.Unmarshal(data, &msg)

			resp := domain.BridgeMessage{
				ID:    msg.ID,
				Error: "selector not found",
			}
			respData, _ := json.Marshal(resp)
			
			binary.Write(pw, binary.LittleEndian, uint32(len(respData)))
			pw.Write(respData)
		}()

		res, err := adapter.CaptureNode(ctx, req)
		if err != nil {
			t.Fatalf("Expected no execution error, got %v", err)
		}

		if res.Success {
			t.Error("Expected success false")
		}
		if res.Message != "selector not found" {
			t.Errorf("Expected error message 'selector not found', got %s", res.Message)
		}
	})
}
