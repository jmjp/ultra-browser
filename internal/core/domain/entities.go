package domain

import (
	"errors"
	"net/url"
)

var (
	ErrInvalidTabID = errors.New("invalid tab id")
	ErrInvalidURL   = errors.New("invalid url format")
)

// Tab representa uma aba do navegador.
type Tab struct {
	ID       int    `json:"id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	WindowID int    `json:"windowId"`
}

// Validate verifica se a aba possui dados consistentes.
func (t Tab) Validate() error {
	if t.ID < 0 {
		return ErrInvalidTabID
	}
	if t.URL == "" {
		return ErrInvalidURL
	}
	_, err := url.ParseRequestURI(t.URL)
	if err != nil {
		return ErrInvalidURL
	}
	return nil
}

// Window representa uma janela do navegador contendo abas.
type Window struct {
	ID   int   `json:"id"`
	Tabs []Tab `json:"tabs,omitempty"`
}

// Tool define uma capacidade do browser exposta via MCP.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

// Validate verifica se a ferramenta possui dados obrigatórios.
func (t Tool) Validate() error {
	if t.Name == "" {
		return errors.New("tool name is required")
	}
	if t.Description == "" {
		return errors.New("tool description is required")
	}
	return nil
}
