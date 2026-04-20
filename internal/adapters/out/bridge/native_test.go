package bridge

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
	"ultra-browser/internal/core/domain"
)

// TestReader_ReadMessage validates basic reading functionality and 4-byte framing.
func TestReader_ReadMessage(t *testing.T) {
	t.Run("Valid message reading", func(t *testing.T) {
		payload := `{"id":"1","tool":"test"}`
		buf := new(bytes.Buffer)
		length := uint32(len(payload))
		binary.Write(buf, binary.LittleEndian, length)
		buf.WriteString(payload)

		r := NewReader(buf)
		msg, err := ReadMessageWithTimeout(r, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if string(msg) != payload {
			t.Errorf("Expected %s, got %s", payload, string(msg))
		}
	})

	t.Run("Empty message", func(t *testing.T) {
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, uint32(0))

		r := NewReader(buf)
		msg, err := ReadMessageWithTimeout(r, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(msg) != 0 {
			t.Errorf("Expected empty message, got %s", string(msg))
		}
	})

	t.Run("Large message", func(t *testing.T) {
		size := 1 * 1024 * 1024 // 1MB
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = 'a'
		}

		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, uint32(size))
		buf.Write(payload)

		r := NewReader(buf)
		msg, err := ReadMessageWithTimeout(r, 1*time.Second)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(msg) != size {
			t.Errorf("Expected size %d, got %d", size, len(msg))
		}
	})

	t.Run("Premature EOF in header", func(t *testing.T) {
		buf := bytes.NewReader([]byte{0x01, 0x00}) // Only 2 bytes of header
		r := NewReader(buf)
		_, err := r.ReadMessage()
		if err == nil {
			t.Fatal("Expected error due to premature EOF in header, got nil")
		}
		if err != io.ErrUnexpectedEOF {
			t.Errorf("Expected io.ErrUnexpectedEOF, got %v", err)
		}
	})

	t.Run("Premature EOF in payload", func(t *testing.T) {
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, uint32(10))
		buf.WriteString("hello") // Only 5 bytes

		r := NewReader(buf)
		_, err := r.ReadMessage()
		if err == nil {
			t.Fatal("Expected error due to premature EOF in payload, got nil")
		}
		if err != io.ErrUnexpectedEOF {
			t.Errorf("Expected io.ErrUnexpectedEOF, got %v", err)
		}
	})

	t.Run("Invalid JSON payload", func(t *testing.T) {
		payload := `{"invalid": json` // Malformed JSON
		buf := new(bytes.Buffer)
		length := uint32(len(payload))
		binary.Write(buf, binary.LittleEndian, length)
		buf.WriteString(payload)

		r := NewReader(buf)
		msg, err := ReadMessageWithTimeout(r, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if string(msg) != payload {
			t.Errorf("Expected raw bytes %s, got %s", payload, string(msg))
		}
	})

	t.Run("EOF at start", func(t *testing.T) {
		buf := bytes.NewReader([]byte{})
		r := NewReader(buf)
		_, err := r.ReadMessage()
		if err != io.EOF {
			t.Errorf("Expected io.EOF, got %v", err)
		}
	})
}

// TestReader_Concurrency ensures the reader can be used in goroutines with channels.
func TestReader_Concurrency(t *testing.T) {
	t.Run("Non-blocking behavior simulation", func(t *testing.T) {
		pr, pw := io.Pipe()
		r := NewReader(pr)
		msgChan := make(chan []byte)
		errChan := make(chan error)

		go func() {
			for {
				msg, err := r.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				msgChan <- msg
			}
		}()

		payload := `{"msg":"hello"}`
		go func() {
			binary.Write(pw, binary.LittleEndian, uint32(len(payload)))
			pw.Write([]byte(payload))
			pw.Close()
		}()

		select {
		case msg := <-msgChan:
			if string(msg) != payload {
				t.Errorf("Expected %s, got %s", payload, string(msg))
			}
		case err := <-errChan:
			if err != io.EOF {
				t.Errorf("Unexpected error: %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for message")
		}

		select {
		case err := <-errChan:
			if err != io.EOF {
				t.Errorf("Expected EOF, got %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for EOF")
		}
	})
}

// Helper to read with timeout for testing
func ReadMessageWithTimeout(r *Reader, timeout time.Duration) ([]byte, error) {
	type result struct {
		msg []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		msg, err := r.ReadMessage()
		ch <- result{msg, err}
	}()

	select {
	case res := <-ch:
		return res.msg, res.err
	case <-time.After(timeout):
		return nil, io.ErrUnexpectedEOF // or custom timeout error
	}
}

// TestReader_ReadLoop validates the continuous reading and delivery of messages.
func TestReader_ReadLoop(t *testing.T) {
	t.Run("Continuous message delivery", func(t *testing.T) {
		pr, pw := io.Pipe()
		r := NewReader(pr)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		events, err := r.ReadEvents(ctx)
		if err != nil {
			t.Fatalf("Expected no error from ReadEvents, got %v", err)
		}

		errChan := make(chan error, 1)
		go func() {
			errChan <- r.ReadLoop(ctx)
		}()

		messages := []string{
			`{"id":"1","tool":"t1"}`,
			`{"id":"2","tool":"t2"}`,
		}

		go func() {
			for _, m := range messages {
				binary.Write(pw, binary.LittleEndian, uint32(len(m)))
				pw.Write([]byte(m))
			}
			pw.Close()
		}()

		for i := range messages {
			select {
			case msg := <-events:
				expectedID := fmt.Sprintf("%d", i+1)
				if msg.ID != expectedID {
					t.Errorf("Expected ID %s, got %s", expectedID, msg.ID)
				}
			case <-time.After(1 * time.Second):
				t.Fatalf("Timeout waiting for message %d", i+1)
			}
		}

		// Loop should terminate normally on EOF (from pw.Close())
		select {
		case err := <-errChan:
			if err != nil {
				t.Errorf("Expected nil error on EOF, got %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("ReadLoop did not terminate after EOF")
		}
	})

	t.Run("Message handler interception", func(t *testing.T) {
		pr, pw := io.Pipe()
		r := NewReader(pr)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		intercepted := make(chan domain.BridgeMessage, 1)
		r.SetMessageHandler(func(msg domain.BridgeMessage) bool {
			if msg.ID == "intercept-me" {
				intercepted <- msg
				return true
			}
			return false
		})

		events, _ := r.ReadEvents(ctx)

		go func() {
			_ = r.ReadLoop(ctx)
		}()

		msg1 := `{"id":"intercept-me"}`
		msg2 := `{"id":"normal"}`

		binary.Write(pw, binary.LittleEndian, uint32(len(msg1)))
		pw.Write([]byte(msg1))
		binary.Write(pw, binary.LittleEndian, uint32(len(msg2)))
		pw.Write([]byte(msg2))
		pw.Close()

		select {
		case msg := <-intercepted:
			if msg.ID != "intercept-me" {
				t.Errorf("Expected intercepted ID 'intercept-me', got %s", msg.ID)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for intercepted message")
		}

		select {
		case msg := <-events:
			if msg.ID != "normal" {
				t.Errorf("Expected normal ID 'normal', got %s", msg.ID)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for normal message")
		}
	})

	t.Run("Loop terminates on context cancel", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pw.Close()
		r := NewReader(pr)
		ctx, cancel := context.WithCancel(context.Background())
		
		errChan := make(chan error, 1)
		go func() {
			errChan <- r.ReadLoop(ctx)
		}()

		cancel()

		select {
		case err := <-errChan:
			if err != context.Canceled {
				t.Errorf("Expected context.Canceled, got %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("ReadLoop did not terminate on context cancel")
		}
	})
}

// TestWriter_WriteMessage validates framing and serialization.
func TestWriter_WriteMessage(t *testing.T) {
	t.Run("Validate 4-byte framing (Little Endian)", func(t *testing.T) {
		buf := new(bytes.Buffer)
		w := NewWriter(buf)

		msg := domain.BridgeMessage{ID: "1", Tool: "test"}
		payload, _ := json.Marshal(msg)
		expectedLength := uint32(len(payload))

		err := w.WriteMessage(msg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Note: Tests will fail until @go-engineer implements the logic.
		if buf.Len() < 4 {
			t.Fatalf("Expected at least 4 bytes for header, got %d", buf.Len())
		}

		var length uint32
		binary.Read(buf, binary.LittleEndian, &length)
		if length != expectedLength {
			t.Errorf("Expected length %d, got %d", expectedLength, length)
		}

		if buf.String() != string(payload) {
			t.Errorf("Expected payload %s, got %s", string(payload), buf.String())
		}
	})

	t.Run("Serialization under concurrency", func(t *testing.T) {
		buf := new(bytes.Buffer)
		w := NewWriter(buf)
		count := 100
		var wg sync.WaitGroup

		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msg := domain.BridgeMessage{ID: fmt.Sprintf("%d", id)}
				_ = w.WriteMessage(msg)
			}(i)
		}
		wg.Wait()

		// Verify we have 'count' messages and they are not interleaved.
		// Since we don't know the order, we check total size and 
		// if we can read 'count' valid length/payload pairs.
		for i := 0; i < count; i++ {
			var length uint32
			if err := binary.Read(buf, binary.LittleEndian, &length); err != nil {
				t.Fatalf("Failed to read length at message %d: %v", i, err)
			}
			data := make([]byte, length)
			if _, err := io.ReadFull(buf, data); err != nil {
				t.Fatalf("Failed to read payload at message %d: %v", i, err)
			}
			var msg domain.BridgeMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				t.Fatalf("Message %d is corrupted: %v", i, err)
			}
		}
	})
}

// TestWriter_CallCorrelation validates the async RequestId -> Response Channel mapping.
func TestWriter_CallCorrelation(t *testing.T) {
	t.Run("Successful correlation", func(t *testing.T) {
		// We use a discard writer since we don't need to check the output here.
		w := NewWriter(io.Discard)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		reqID := "req-123"
		msg := domain.BridgeMessage{ID: reqID, Tool: "test"}

		// Simulate response delivery in a goroutine
		go func() {
			time.Sleep(10 * time.Millisecond)
			response := domain.BridgeMessage{ID: reqID, Result: json.RawMessage(`{"status":"ok"}`)}
			w.HandleResponse(response)
		}()

		res, err := w.Call(ctx, msg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if res.ID != reqID {
			t.Errorf("Expected response ID %s, got %s", reqID, res.ID)
		}
	})

	t.Run("Timeout when no response arrives", func(t *testing.T) {
		w := NewWriter(io.Discard)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		msg := domain.BridgeMessage{ID: "timeout-id", Tool: "test"}
		_, err := w.Call(ctx, msg)

		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}
	})

	t.Run("Context cancel cleanup", func(t *testing.T) {
		w := NewWriter(io.Discard)
		ctx, cancel := context.WithCancel(context.Background())

		msg := domain.BridgeMessage{ID: "cancel-id", Tool: "test"}
		
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		_, err := w.Call(ctx, msg)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}

		// Verify that the pending channel was removed
		w.pmu.Lock()
		_, exists := w.pending[msg.ID]
		w.pmu.Unlock()
		if exists {
			t.Error("Pending request was not cleaned up after cancellation")
		}
	})
}

// TestWriter_CaptureNodeFormatting validates the JSON structure for the capture_node tool.
func TestWriter_CaptureNodeFormatting(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewWriter(buf)

	req := domain.CaptureNodeRequest{
		Selector: "#content",
		Format:   "png",
		Path:     "/tmp/capture.png",
	}

	msg, err := domain.NewBridgeRequest("cap-123", "capture_node", req)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	err = w.WriteMessage(msg)
	if err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}

	// Read framing
	var length uint32
	if err := binary.Read(buf, binary.LittleEndian, &length); err != nil {
		t.Fatalf("Failed to read length prefix: %v", err)
	}

	if uint32(buf.Len()) != length {
		t.Errorf("Message length prefix %d does not match actual length %d", length, buf.Len())
	}

	// Validate JSON structure
	var sent domain.BridgeMessage
	if err := json.Unmarshal(buf.Bytes(), &sent); err != nil {
		t.Fatalf("Sent data is not valid JSON: %v", err)
	}

	if sent.ID != "cap-123" {
		t.Errorf("Expected ID 'cap-123', got %s", sent.ID)
	}
	if sent.Tool != "capture_node" {
		t.Errorf("Expected tool 'capture_node', got %s", sent.Tool)
	}

	// Validate params
	var sentParams domain.CaptureNodeRequest
	if err := json.Unmarshal(sent.Params, &sentParams); err != nil {
		t.Fatalf("Params are not valid CaptureNodeRequest: %v", err)
	}

	if sentParams.Selector != req.Selector {
		t.Errorf("Expected selector %s, got %s", req.Selector, sentParams.Selector)
	}
	if sentParams.Format != req.Format {
		t.Errorf("Expected format %s, got %s", req.Format, sentParams.Format)
	}
}
