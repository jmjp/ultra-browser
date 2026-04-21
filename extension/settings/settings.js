const models = {
  anthropic: ['claude-3-5-sonnet-20241022', 'claude-3-opus-20240229', 'claude-3-haiku-20240307'],
  openai: ['gpt-4o', 'gpt-4o-mini', 'o1-preview', 'o1-mini'],
  gemini: ['gemini-2.5-pro', 'gemini-2.0-flash', 'gemini-1.5-pro'],
  deepseek: ['deepseek-chat', 'deepseek-reasoner']
};

const providerSelect = document.getElementById('provider-select');
const modelSelect = document.getElementById('model-select');
const customModel = document.getElementById('custom-model');
const customUrlGroup = document.getElementById('custom-url-group');

function updateModelOptions() {
  const provider = providerSelect.value;
  if (provider === 'custom') {
    modelSelect.classList.add('hidden');
    customModel.classList.remove('hidden');
    customUrlGroup.classList.remove('hidden');
  } else {
    modelSelect.classList.remove('hidden');
    customModel.classList.add('hidden');
    customUrlGroup.classList.add('hidden');

    modelSelect.innerHTML = '';
    (models[provider] || []).forEach(m => {
      const opt = document.createElement('option');
      opt.value = m;
      opt.textContent = m;
      modelSelect.appendChild(opt);
    });
  }
}

providerSelect.addEventListener('change', updateModelOptions);
document.getElementById('temperature').addEventListener('input', (e) => document.getElementById('temp-val').textContent = e.target.value);

document.getElementById('save-provider').addEventListener('click', async () => {
  const config = {
    provider: providerSelect.value,
    apiKey: document.getElementById('api-key').value,
    baseUrl: document.getElementById('base-url').value,
    model: providerSelect.value === 'custom' ? customModel.value : modelSelect.value,
    temperature: parseFloat(document.getElementById('temperature').value),
    maxTokens: parseInt(document.getElementById('max-tokens').value)
  };

  await chrome.storage.local.set({ providerConfig: config });

  const msg = document.getElementById('provider-save-msg');
  msg.style.display = 'inline';
  setTimeout(() => msg.style.display = 'none', 2000);
});

async function loadConfig() {
  const { providerConfig } = await chrome.storage.local.get('providerConfig');
  if (providerConfig) {
    providerSelect.value = providerConfig.provider || 'anthropic';
    updateModelOptions();

    document.getElementById('api-key').value = providerConfig.apiKey || '';
    document.getElementById('base-url').value = providerConfig.baseUrl || '';
    if (providerConfig.provider === 'custom') customModel.value = providerConfig.model || '';
    else modelSelect.value = providerConfig.model || '';

    document.getElementById('temperature').value = providerConfig.temperature !== undefined ? providerConfig.temperature : 0.7;
    document.getElementById('temp-val').textContent = document.getElementById('temperature').value;
    document.getElementById('max-tokens').value = providerConfig.maxTokens || 8192;
  } else {
    updateModelOptions();
  }
}

// MCP Servers
const mcpList = document.getElementById('mcp-list');

function renderMCPs(servers) {
  mcpList.innerHTML = '';
  servers.forEach((s, idx) => {
    const div = document.createElement('div');
    div.className = 'mcp-server-item';
    div.innerHTML = `
      <input type="text" value="${s.id}" placeholder="Nome (ex: local-fs)" data-idx="${idx}" class="mcp-name">
      <input type="text" value="${s.url}" placeholder="URL (ex: http://localhost:3001/mcp)" data-idx="${idx}" class="mcp-url">
      <button class="test-mcp" data-idx="${idx}">Testar</button>
      <button class="danger remove-mcp" data-idx="${idx}">Remover</button>
    `;
    mcpList.appendChild(div);
  });

  document.querySelectorAll('.remove-mcp').forEach(btn => {
    btn.addEventListener('click', (e) => {
      const idx = e.target.dataset.idx;
      const servers = getMCPsFromDOM();
      servers.splice(idx, 1);
      renderMCPs(servers);
    });
  });

  document.querySelectorAll('.test-mcp').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      const idx = e.target.dataset.idx;
      const url = document.querySelectorAll('.mcp-url')[idx].value;
      const originalText = e.target.textContent;
      e.target.textContent = 'Testando...';
      try {
        const res = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list', params: {} })
        });
        const data = await res.json();
        const count = data.result?.tools?.length || 0;
        e.target.textContent = `OK (${count} tools)`;
      } catch (err) {
        e.target.textContent = 'Erro!';
      }
      setTimeout(() => e.target.textContent = originalText, 3000);
    });
  });
}

function getMCPsFromDOM() {
  const servers = [];
  const names = document.querySelectorAll('.mcp-name');
  const urls = document.querySelectorAll('.mcp-url');
  for (let i = 0; i < names.length; i++) {
    if (names[i].value && urls[i].value) {
      servers.push({ id: names[i].value, url: urls[i].value });
    }
  }
  return servers;
}

document.getElementById('add-mcp-btn').addEventListener('click', () => {
  const servers = getMCPsFromDOM();
  servers.push({ id: '', url: '' });
  renderMCPs(servers);
});

document.getElementById('save-mcp').addEventListener('click', async () => {
  const servers = getMCPsFromDOM();
  await chrome.storage.local.set({ mcpServers: servers });
  chrome.runtime.sendMessage({ type: 'REFRESH_TOOLS' });

  const msg = document.getElementById('mcp-save-msg');
  msg.style.display = 'inline';
  setTimeout(() => msg.style.display = 'none', 2000);
});

async function loadMCPs() {
  const { mcpServers } = await chrome.storage.local.get('mcpServers');
  renderMCPs(mcpServers || []);
}

document.getElementById('clear-history').addEventListener('click', async () => {
  if (confirm("Limpar todo o histórico?")) {
    // History memory exists only in background script currently, we can reload extension to wipe it
    chrome.runtime.reload();
  }
});

loadConfig();
loadMCPs();
