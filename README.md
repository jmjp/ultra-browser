# 🚀 Ultra Browser — AI Browser Agent

**Ultra Browser** é um AI Browser Agent completo que roda 100% dentro de uma Chrome Extension.
Nenhum binário externo, nenhum processo em segundo plano fora do Chrome.

O usuário instala a extensão e pode utilizá-la fornecendo sua própria API Key de provedores suportados (BYOK - Bring Your Own Key). A extensão possui integração total com APIs nativas e suporte a ferramentas Model Context Protocol (MCP) via HTTP/SSE.

---

## 🏗️ Arquitetura Final

A extensão atua tanto como a UI do Agente quanto o orquestrador das tarefas e interações:

```
Usuário
  ↓ (Side Panel)
Service Worker
  ├── LLM API (fetch direto: Anthropic, OpenAI, Gemini, DeepSeek, Custom)
  ├── Browser Tools (chrome.tabs, chrome.scripting, chrome.debugger)
  └── MCP HTTP/SSE servers (fetch direto, configurados pelo usuário)
```

### Componentes Principais

1. **Manifest V3:** Configuração moderna de permissões, focando no Side Panel e permissões do Debugger/Scripting para automação.
2. **Service Worker (`background/service-worker.js`):** Ponto central que orquestra o Agentic Loop, delega chamadas de ferramentas nativas do browser e interage com os clientes de IA e MCP.
3. **LLM Adapters (`lib/llm/`):** Adaptadores independentes implementando interfaces de stream (SSE) para suportar múltiplos provedores usando endpoints diretos.
4. **Browser Tools (`lib/tools/browser-tools.js`):** Implementação via `chrome.debugger` e `chrome.tabs` para automação que burla restrições nativas como o Content Security Policy (CSP).
5. **Side Panel (`sidepanel/`):** Interface de usuário focada com Chat interativo, Planner para tarefas complexas e Visualizador de Logs.
6. **Settings Page (`settings/`):** Gerencia chaves de API, provedores e endpoints MCP (Tudo salvo em armazenamento local).

---

## 🚀 Instalação e Configuração

### 1. Requisitos
- **Google Chrome**

### 2. Configurando a Extensão (Chrome)
1. Abra `chrome://extensions` no seu navegador.
2. Ative o **"Modo do desenvolvedor"** no canto superior direito.
3. Clique em **"Carregar sem compactação"** e selecione a pasta `extension/` deste repositório.
4. Para acessar a extensão, abra o painel lateral do Chrome e selecione o "Ultra Browser" no menu de Side Panels.

### 3. Configurando as Ferramentas MCP (Opcional)
Você pode adicionar servidores compatíveis com o Model Context Protocol (MCP) que rodam via HTTP/SSE. Vá na página de configurações do "Ultra Browser", seção de *Servidores MCP* e inclua as credenciais de URL/Porta dos seus servidores.

---

## 🛠️ Ferramentas Nativas

O sistema expõe diversas capacidades de automação da navegação para o agente:

| Ferramenta | Descrição |
| :--- | :--- |
| `list_tabs` | Lista todas as abas abertas. |
| `navigate` | Navega para uma URL (aba ativa ou nova). |
| `screenshot` | Captura um screenshot da aba ativa (PNG). |
| `get_content` | Extrai todo o conteúdo de texto da página atual (via Debugger). |
| `click` | Clica em um elemento via seletor CSS (via Debugger). |
| `type_text` | Digita texto em campos de formulário (via Debugger). |
| `execute_script` | Executa JavaScript arbitrário na página (via Debugger). |
| `scroll` | Interações de rolagem e passar o mouse. |
| `wait_for_element` | Aguarda a aparição de um elemento no DOM. |
| `get_page_info` | Retorna metadados da página atual. |

---

## 🔒 Privacidade e Segurança
- Todo o código interage diretamente usando o Chrome Extensions API.
- As suas API Keys são armazenadas localmente no seu computador e comunicam-se diretamente com o serviço de LLM selecionado sem servidores intermediários de terceiros.
- Não existem binários compilados em Go a serem executados separadamente, ou Native Messaging Hosts, assegurando que o escopo fica limitado às capacidades providenciadas pelo seu navegador Chrome e ferramentas MCP externas configuradas por si próprio.

---
