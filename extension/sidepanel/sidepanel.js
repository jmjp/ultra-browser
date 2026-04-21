let attachments = [];
let currentAgentId = 'main';
let logs = [];

// Tabs
document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    tab.classList.add('active');
    document.getElementById(`tab-${tab.dataset.tab}`).classList.add('active');
  });
});

// Settings
document.getElementById('settings-btn').addEventListener('click', () => {
  chrome.runtime.openOptionsPage ? chrome.runtime.openOptionsPage() : window.open(chrome.runtime.getURL('settings/index.html'));
});

// Auto-resize textarea
const chatInput = document.getElementById('chat-input');
chatInput.addEventListener('input', function() {
  this.style.height = 'auto';
  this.style.height = (this.scrollHeight) + 'px';
});

// Handle Send
chatInput.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    sendMessage();
  }
});
document.getElementById('send-btn').addEventListener('click', sendMessage);

// Paste images
document.addEventListener('paste', async (e) => {
  const items = (e.clipboardData || e.originalEvent.clipboardData).items;
  for (let index in items) {
    const item = items[index];
    if (item.kind === 'file') {
      const blob = item.getAsFile();
      const reader = new FileReader();
      reader.onload = (event) => {
        addAttachment({ type: blob.type.startsWith('image/') ? 'image' : 'file', name: blob.name || 'Pasted File', base64: event.target.result, content: event.target.result, mimeType: blob.type });
      };
      if (blob.type.startsWith('image/')) reader.readAsDataURL(blob);
      else reader.readAsText(blob);
    }
  }
});

function addAttachment(att) {
  attachments.push(att);
  renderAttachments();
}

function renderAttachments() {
  const container = document.getElementById('attachments-preview');
  container.innerHTML = '';
  attachments.forEach((att, idx) => {
    const div = document.createElement('div');
    div.className = 'attachment-item';
    if (att.type === 'image') {
      div.innerHTML = `<img src="${att.base64}"><div class="remove" data-idx="${idx}">×</div>`;
    } else {
      div.innerHTML = `<span>📄 ${att.name}</span><div class="remove" data-idx="${idx}">×</div>`;
    }
    container.appendChild(div);
  });
  document.querySelectorAll('.remove').forEach(btn => {
    btn.addEventListener('click', (e) => {
      attachments.splice(e.target.dataset.idx, 1);
      renderAttachments();
    });
  });
}

document.getElementById('attach-btn').addEventListener('click', () => document.getElementById('file-input').click());
document.getElementById('file-input').addEventListener('change', (e) => {
  for (const file of e.target.files) {
    const reader = new FileReader();
    reader.onload = (event) => {
      addAttachment({ type: file.type.startsWith('image/') ? 'image' : 'file', name: file.name, base64: event.target.result, content: event.target.result, mimeType: file.type });
    };
    if (file.type.startsWith('image/')) reader.readAsDataURL(file);
    else reader.readAsText(file);
  }
  e.target.value = ''; // reset
});

let currentAssistantMessageDiv = null;

function appendMessage(role, content) {
  const list = document.getElementById('messages');
  const div = document.createElement('div');
  div.className = `message ${role}`;
  if (role === 'user') {
     div.textContent = content;
  } else {
     div.innerHTML = marked.parse(content);
     currentAssistantMessageDiv = div;
  }
  list.appendChild(div);
  list.scrollTop = list.scrollHeight;
  return div;
}

function updateAssistantMessage(content) {
  if (currentAssistantMessageDiv) {
    currentAssistantMessageDiv.innerHTML = marked.parse(content);
    const list = document.getElementById('messages');
    list.scrollTop = list.scrollHeight;
  }
}

function addLog(text, type = '') {
  const d = new Date();
  const time = `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`;
  const div = document.createElement('div');
  div.className = `log-entry ${type}`;
  div.textContent = `[${time}] ${text}`;
  document.getElementById('logs-list').appendChild(div);
}

document.getElementById('clear-logs-btn').addEventListener('click', () => {
  document.getElementById('logs-list').innerHTML = '';
});

let assistantTextBuffer = '';
let toolStartTimes = {};

function sendMessage() {
  const text = chatInput.value.trim();
  if (!text && attachments.length === 0) return;

  appendMessage('user', text);
  chatInput.value = '';
  chatInput.style.height = 'auto';

  document.getElementById('send-btn').classList.add('hidden');
  document.getElementById('cancel-btn').classList.remove('hidden');
  document.getElementById('status-indicator').className = 'status running';

  const currentAttachments = [...attachments];
  attachments = [];
  renderAttachments();

  assistantTextBuffer = '';
  currentAssistantMessageDiv = null;

  chrome.runtime.sendMessage({
    type: 'RUN_AGENT',
    message: text,
    attachments: currentAttachments,
    agentId: currentAgentId
  });
}

document.getElementById('cancel-btn').addEventListener('click', () => {
  chrome.runtime.sendMessage({ type: 'CANCEL_AGENT', agentId: currentAgentId });
  resetUI();
});

function resetUI() {
  document.getElementById('send-btn').classList.remove('hidden');
  document.getElementById('cancel-btn').classList.add('hidden');
  document.getElementById('tool-indicator').classList.add('hidden');
  document.getElementById('status-indicator').className = 'status online';
}

chrome.runtime.onMessage.addListener((msg) => {
  if (msg.type === 'AGENT_CHUNK' && msg.agentId === currentAgentId) {
    const chunk = msg.chunk;

    if (chunk.type === 'text') {
      if (!currentAssistantMessageDiv) appendMessage('assistant', '');
      assistantTextBuffer += chunk.content;
      updateAssistantMessage(assistantTextBuffer);
    }
    else if (chunk.type === 'tool_start') {
      document.getElementById('tool-indicator').classList.remove('hidden');
      document.getElementById('tool-name').textContent = `Executando: ${chunk.name}`;
      toolStartTimes[chunk.name] = performance.now();
    }
    else if (chunk.type === 'tool_done') {
      document.getElementById('tool-indicator').classList.add('hidden');
      const time = toolStartTimes[chunk.name] ? Math.round(performance.now() - toolStartTimes[chunk.name]) : 0;
      addLog(`${chunk.name}() → ${time}ms ✓`, 'success');
    }
    else if (chunk.type === 'tool_error') {
      document.getElementById('tool-indicator').classList.add('hidden');
      const time = toolStartTimes[chunk.name] ? Math.round(performance.now() - toolStartTimes[chunk.name]) : 0;
      addLog(`${chunk.name}() → ${time}ms ✗ (${chunk.error})`, 'error');
    }
    else if (chunk.type === 'error') {
      appendMessage('assistant', `**Erro:** ${chunk.message}`);
      resetUI();
    }
    else if (chunk.type === 'done') {
      resetUI();
    }
  }
});

// Basic Planner implementation stub
document.getElementById('generate-plan-btn').addEventListener('click', () => {
   const text = document.getElementById('planner-input').value;
   if (!text) return;

   // In a real implementation this would trigger a specific prompt to the service worker.
   // Here we just mock it for UI demo purposes as per instructions to run inside the extension.
   const steps = document.getElementById('plan-steps');
   steps.innerHTML = `<div class="step">Gerando plano...</div>`;

   chrome.runtime.sendMessage({
      type: 'RUN_AGENT',
      message: `Gere um plano detalhado em JSON para: ${text}\nFormato: { "steps": [{ "id": 1, "title": "...", "description": "...", "requires_approval": false }] }\nResponda APENAS com o JSON.`,
      attachments: [],
      agentId: 'planner'
   });
});

chrome.runtime.sendMessage({ type: 'REFRESH_TOOLS' }, () => {
   document.getElementById('status-indicator').className = 'status online';
});
