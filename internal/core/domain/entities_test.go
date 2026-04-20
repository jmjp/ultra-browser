package domain_test

import (
	"testing"
	"ultra-browser/internal/core/domain"
)

func TestTab_Validate(t *testing.T) {
	tests := []struct {
		name    string
		tab     domain.Tab
		wantErr error
	}{
		{
			name: "aba válida",
			tab: domain.Tab{
				ID:    1,
				URL:   "https://google.com",
				Title: "Google",
			},
			wantErr: nil,
		},
		{
			name: "id inválido (negativo)",
			tab: domain.Tab{
				ID:  -1,
				URL: "https://google.com",
			},
			wantErr: domain.ErrInvalidTabID,
		},
		{
			name: "url vazia",
			tab: domain.Tab{
				ID:  1,
				URL: "",
			},
			wantErr: domain.ErrInvalidURL,
		},
		{
			name: "url malformada",
			tab: domain.Tab{
				ID:  1,
				URL: "htt p://google.com", // Espaço no meio
			},
			wantErr: domain.ErrInvalidURL,
		},
		{
			name: "url sem protocolo ou absoluta",
			tab: domain.Tab{
				ID:  1,
				URL: "google.com",
			},
			wantErr: domain.ErrInvalidURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tab.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWindow_Structure(t *testing.T) {
	// Teste simples para garantir que a estrutura Window contém abas
	w := domain.Window{
		ID: 1,
		Tabs: []domain.Tab{
			{ID: 1, URL: "https://a.com"},
			{ID: 2, URL: "https://b.com"},
		},
	}

	if len(w.Tabs) != 2 {
		t.Errorf("Esperava 2 abas, obteve %d", len(w.Tabs))
	}
}

func TestTool_Validate(t *testing.T) {
	tests := []struct {
		name    string
		tool    domain.Tool
		wantErr bool
	}{
		{
			name: "ferramenta válida",
			tool: domain.Tool{
				Name:        "list_tabs",
				Description: "Lista abas",
			},
			wantErr: false,
		},
		{
			name: "nome vazio",
			tool: domain.Tool{
				Name:        "",
				Description: "Lista abas",
			},
			wantErr: true,
		},
		{
			name: "descrição vazia",
			tool: domain.Tool{
				Name:        "list_tabs",
				Description: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tool.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
