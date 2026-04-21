import { BROWSER_TOOLS } from '../lib/tools/browser-tools.js';
import { MCPClient } from '../lib/mcp/client.js';
import { getAdapter, buildToolsForProvider } from '../lib/llm/adapter.js';

const state = {
  allTools: [],
  mcpClients: {},
  sessions: {},
};

async function initialize() {
  await aggregateAllTools();
}

async function aggregateAllTools() {
  state.allTools = [...BROWSER_TOOLS];

  const storage = await chrome.storage.local.get('mcpServers');
  const mcpServers = storage.mcpServers || [];

  for (const server of mcpServers) {
    try {
      const client = new MCPClient(server.url);
      const tools = await client.initialize();
      state.mcpClients[server.id] = client;
      tools.forEach(t => state.allTools.push({
        ...t,
        name: `${server.id}__${t.name}`,
        _mcpServerId: server.id,
        _originalName: t.name
      }));
    } catch (e) {
      console.warn(`MCP server ${server.id} indisponível:`, e.message);
    }
  }
}

async function executeToolCall(toolName, toolInput) {
  const tool = state.allTools.find(t => t.name === toolName);
  if (!tool) throw new Error(`Tool não encontrada: ${toolName}`);

  if (tool._mcpServerId) {
    const client = state.mcpClients[tool._mcpServerId];
    return client.callTool(tool._originalName, toolInput);
  } else {
    return tool.execute(toolInput);
  }
}

async function* runAgent(userMessage, attachments, agentId = 'main') {
  const { providerConfig } = await chrome.storage.local.get('providerConfig');
  if (!providerConfig) {
    yield { type: 'error', message: 'Provider não configurado.' };
    return;
  }

  const adapter = getAdapter(providerConfig);

  if (!state.sessions[agentId]) {
    state.sessions[agentId] = { messages: [], status: 'idle' };
  }
  const session = state.sessions[agentId];
  session.status = 'running';

  const userContent = buildUserContent(userMessage, attachments);
  session.messages.push({ role: 'user', content: userContent });

  try {
    while (session.status === 'running') {
      const toolsForProvider = buildToolsForProvider(state.allTools, providerConfig.provider);
      const stream = adapter.chat(session.messages, toolsForProvider, providerConfig);

      let assistantMessage = { role: 'assistant', content: [] };
      let toolCallsMap = new Map();

      for await (const chunk of stream) {
        if (session.status === 'cancelled') break;

        yield chunk; // stream em tempo real para o side panel

        if (chunk.type === 'text') {
          assistantMessage.content.push({ type: 'text', text: chunk.content });
        } else if (chunk.type === 'tool_use_start') {
          toolCallsMap.set(chunk.id, { id: chunk.id, name: chunk.name, argsString: '' });
          yield { type: 'tool_start', name: chunk.name };
        } else if (chunk.type === 'tool_use_delta') {
          const tc = Array.from(toolCallsMap.values()).pop(); // simplistic approach, assume sequential delta
          if (tc) tc.argsString += chunk.delta;
        } else if (chunk.type === 'tool_use') { // Gemini emits full at once
           toolCallsMap.set(chunk.id, { id: chunk.id, name: chunk.name, input: chunk.input });
           yield { type: 'tool_start', name: chunk.name };
        }
      }

      if (session.status === 'cancelled') break;

      const toolCalls = Array.from(toolCallsMap.values()).map(tc => {
        if (!tc.input) {
           try { tc.input = tc.argsString ? JSON.parse(tc.argsString) : {}; }
           catch (e) { tc.input = {}; }
        }
        return tc;
      });

      if (assistantMessage.content.length > 0 || toolCalls.length > 0) {
        session.messages.push(assistantMessage);
      }

      if (toolCalls.length === 0) break;

      const toolResults = await Promise.all(
        toolCalls.map(async (tc) => {
          try {
            const result = await executeToolCall(tc.name, tc.input);
            yield { type: 'tool_done', name: tc.name, result };
            return { toolUseId: tc.id, content: typeof result === 'string' ? result : JSON.stringify(result) };
          } catch (e) {
            yield { type: 'tool_error', name: tc.name, error: e.message };
            return { toolUseId: tc.id, content: `Error: ${e.message}`, isError: true };
          }
        })
      );

      session.messages.push({ role: 'user', content: toolResults.map(r => ({
        type: 'tool_result', tool_use_id: r.toolUseId, content: r.content, isError: r.isError
      }))});
    }
  } catch (e) {
    yield { type: 'error', message: e.message };
  } finally {
    session.status = 'idle';
    yield { type: 'done' };
  }
}

function buildUserContent(text, attachments = []) {
  const content = [];
  if (text) content.push({ type: 'text', text });
  attachments.forEach(att => {
    if (att.type === 'image') {
      content.push({ type: 'image', data: att.base64, mimeType: att.mimeType });
    } else if (att.type === 'file') {
      content.push({ type: 'text', text: `\nConteúdo do arquivo "${att.name}":\n${att.content}` });
    }
  });
  return content;
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === 'RUN_AGENT') {
    (async () => {
      for await (const chunk of runAgent(msg.message, msg.attachments, msg.agentId || 'main')) {
        chrome.runtime.sendMessage({ type: 'AGENT_CHUNK', agentId: msg.agentId || 'main', chunk });
      }
    })();
    sendResponse({ started: true });
  }
  if (msg.type === 'CANCEL_AGENT') {
    if (state.sessions[msg.agentId]) state.sessions[msg.agentId].status = 'cancelled';
    sendResponse({ cancelled: true });
  }
  if (msg.type === 'REFRESH_TOOLS') {
    aggregateAllTools().then(() => sendResponse({ tools: state.allTools.length }));
  }
  return true;
});

initialize();
