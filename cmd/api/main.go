package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"ultra-browser/internal/adapters/in/http"
	"ultra-browser/internal/adapters/out/bridge"
	"ultra-browser/internal/adapters/out/filesystem"
	"ultra-browser/internal/adapters/out/register"
	"ultra-browser/internal/core/domain"
	"ultra-browser/internal/core/services"
)

type Config struct {
	Port   int
	Stdin  io.ReadCloser
	Stdout io.WriteCloser
}

func main() {
	logFile, _ := os.OpenFile("ultra-browser.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if logFile != nil {
		multiWriter := io.MultiWriter(os.Stderr, logFile)
		slog.SetDefault(slog.New(slog.NewTextHandler(multiWriter, nil)))
	}

	flag.CommandLine.SetOutput(os.Stderr)
	port := flag.Int("port", 12306, "Porta do servidor MCP")
	serverMode := flag.Bool("server", false, "Rodar em modo servidor persistente (Singleton)")
	flag.Parse()

	// Comandos de registro
	if flag.NArg() > 0 {
		cmd := flag.Arg(0)
		extID := ""
		if flag.NArg() > 1 {
			extID = flag.Arg(1)
		}
		switch cmd {
		case "register":
			getRegistrar(extID).Install()
			fmt.Fprintln(os.Stderr, "✅ Registrado")
			os.Exit(0)
		case "unregister":
			getRegistrar("").Uninstall()
			os.Exit(0)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Infraestrutura do Hub e Porta Remota
	hub := bridge.NewBridgeHub()
	remotePort := bridge.NewRemoteBrowserPort()
	hub.Register(remotePort) // Por padrão, usa a porta remota que espera conexão via HTTP

	// Adaptador de sistema de arquivos para ferramentas que salvam dados (ex: capture_node)
	fsAdapter := filesystem.NewLocalFileSystemAdapter(".")

	service := services.NewMCPService(hub, fsAdapter)
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcphttp.NewMCPServer(service))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "OK") })

	// Endpoints para Bridge Remota (quando o servidor já está rodando)
	mux.HandleFunc("/bridge/commands", handleBridgeCommands(remotePort))
	mux.HandleFunc("/bridge/response", handleBridgeResponse(remotePort))
	mux.HandleFunc("/bridge/event", handleBridgeEvent(remotePort))

	// Se for modo servidor persistente, apenas sobe a porta
	if *serverMode {
		slog.Info("Iniciando modo Servidor Singleton Persistente")
		startServerWithRetry(ctx, *port, mux)
		waitForInterrupt(cancel)
		return
	}

	// Caso contrário, tenta pegar a porta ou agir como cliente
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		// Porta ocupada: agir como Ponte Cliente (Relay)
		slog.Info("Porta ocupada, iniciando modo Ponte Cliente")
		runBridgeClient(ctx, *port)
		return
	}
	ln.Close()

	// Porta livre: Iniciar como Servidor E Bridge Local
	slog.Info("Porta livre, iniciando como Servidor e Bridge Local")
	r := bridge.NewReader(os.Stdin)
	w := bridge.NewWriter(os.Stdout)
	localAdapter := bridge.NewNativeMessagingAdapter(r, w)
	hub.Register(localAdapter) // Substitui a porta remota pela local (performance)

	go func() {
		if err := localAdapter.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("Erro na bridge local", "error", err)
		}
	}()

	go startServerWithRetry(ctx, *port, mux)
	waitForInterrupt(cancel)
}

func startServerWithRetry(ctx context.Context, port int, handler http.Handler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			slog.Info("Iniciando servidor HTTP", "port", port)
			server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: handler}

			// Goroutine para desligamento gracioso
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				server.Shutdown(shutdownCtx)
			}()

			err := server.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				slog.Warn("Falha ao iniciar servidor, retentando em 1s...", "error", err)
				time.Sleep(1 * time.Second)
				continue
			}
			return
		}
	}
}

func waitForInterrupt(cancel context.CancelFunc) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()
}

// --- Handlers para o Daemon ---

func handleBridgeCommands(p *bridge.RemoteBrowserPort) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, _ := w.(http.Flusher)

		slog.Info("Bridge remota conectada via SSE")
		commands := p.GetCommands()

		for {
			select {
			case <-r.Context().Done():
				slog.Info("Bridge remota desconectada")
				return
			case cmd := <-commands:
				data, _ := json.Marshal(cmd)
				fmt.Fprintf(w, "event: command\ndata: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

func handleBridgeResponse(p *bridge.RemoteBrowserPort) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var msg domain.BridgeMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		p.HandleResponse(msg)
		w.WriteHeader(200)
	}
}

func handleBridgeEvent(p *bridge.RemoteBrowserPort) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var msg domain.BridgeMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		p.HandleEvent(msg)
		w.WriteHeader(200)
	}
}

// --- Lógica do Cliente (Ponte) ---

func runBridgeClient(ctx context.Context, port int) {
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	r := bridge.NewReader(os.Stdin)
	w := bridge.NewWriter(os.Stdout)
	adapter := bridge.NewNativeMessagingAdapter(r, w)

	// Encaminha eventos locais para o servidor central
	go func() {
		ch, _ := adapter.ReadEvents(ctx)
		for msg := range ch {
			data, _ := json.Marshal(msg)
			http.Post(baseURL+"/bridge/event", "application/json", bytes.NewReader(data))
		}
	}()

	// Loop SSE para receber comandos do servidor central
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := connectToSSE(ctx, baseURL, adapter)
				if err != nil {
					slog.Warn("Erro no SSE cliente, reconectando em 1s...", "error", err)
					time.Sleep(1 * time.Second)
				}
			}
		}
	}()

	if err := adapter.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("Erro no loop de leitura da bridge cliente", "error", err)
	}
}

func connectToSSE(ctx context.Context, baseURL string, adapter *bridge.NativeMessagingAdapter) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/bridge/commands", nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}

	br := bufio.NewReader(resp.Body)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return err
		}

		if strings.HasPrefix(line, "data: ") {
			var cmd domain.BridgeMessage
			data := strings.TrimPrefix(line, "data: ")
			if err := json.Unmarshal([]byte(data), &cmd); err == nil {
				go func(c domain.BridgeMessage) {
					res, err := adapter.ExecuteCommand(ctx, c)
					if err != nil {
						res = domain.BridgeMessage{
							ID:    c.ID,
							Error: err.Error(),
						}
					}
					payload, _ := json.Marshal(res)
					http.Post(baseURL+"/bridge/response", "application/json", bytes.NewReader(payload))
				}(cmd)
			}
		}
	}
}

func getRegistrar(extID string) *register.NativeHostRegistrar {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		home = os.Getenv("APPDATA")
	}
	execPath, _ := os.Executable()
	absPath, _ := filepath.Abs(execPath)
	if runtime.GOOS == "windows" && filepath.Ext(absPath) == "" {
		absPath += ".exe"
	}
	allowedOrigins := []string{"chrome-extension://fofogpkfclbmmhliojekihdghkhkjmhk/"}
	if extID != "" {
		allowedOrigins = []string{fmt.Sprintf("chrome-extension://%s/", extID)}
	}
	return register.NewRegistrar(register.Config{
		Name: "com.ultra_browser.host", Description: "Ultra Browser", BinaryPath: absPath, Type: "stdio", AllowedOrigins: allowedOrigins,
	}, runtime.GOOS, home)
}
