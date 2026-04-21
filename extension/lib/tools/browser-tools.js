// extension/lib/tools/browser-tools.js

// Function to attach debugger if not attached
async function ensureDebuggerAttached(tabId) {
  const targets = await chrome.debugger.getTargets();
  const isAttached = targets.some(t => t.tabId === tabId && t.attached);
  if (!isAttached) {
    await chrome.debugger.attach({ tabId: tabId }, "1.3");
  }
}

// Function to run script via debugger (evaluates in page context bypassing CSP)
async function evaluateInTab(tabId, expression) {
  await ensureDebuggerAttached(tabId);
  return new Promise((resolve, reject) => {
    chrome.debugger.sendCommand(
      { tabId: tabId },
      "Runtime.evaluate",
      {
        expression: expression,
        returnByValue: true,
        awaitPromise: true
      },
      (result) => {
        if (chrome.runtime.lastError) {
          reject(new Error(chrome.runtime.lastError.message));
        } else if (result.exceptionDetails) {
          // If there's an exception, format it
          const e = result.exceptionDetails;
          const msg = e.exception ? e.exception.description : e.text;
          reject(new Error(msg));
        } else {
          resolve(result.result.value);
        }
      }
    );
  });
}

export const BROWSER_TOOLS = [
  {
    name: "list_tabs",
    description: "Lista todas as abas abertas",
    parameters: { type: "object", properties: {}, required: [] },
    async execute(params) {
      const tabs = await chrome.tabs.query({});
      return tabs.map(t => ({ id: t.id, title: t.title, url: t.url, active: t.active }));
    }
  },
  {
    name: "navigate",
    description: "Navega para uma URL na aba ativa ou em nova aba",
    parameters: {
      type: "object",
      properties: {
        url: { type: "string" },
        new_tab: { type: "boolean", default: false }
      },
      required: ["url"]
    },
    async execute({ url, new_tab }) {
      if (new_tab) {
        const tab = await chrome.tabs.create({ url });
        await waitForTabLoad(tab.id);
        return { tabId: tab.id, url };
      } else {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        await chrome.tabs.update(tab.id, { url });
        await waitForTabLoad(tab.id);
        return { tabId: tab.id, url };
      }
    }
  },
  {
    name: "screenshot",
    description: "Captura screenshot da aba ativa em base64 PNG",
    parameters: { type: "object", properties: {}, required: [] },
    async execute() {
      const dataUrl = await chrome.tabs.captureVisibleTab(null, { format: "png" });
      return { image: dataUrl }; // base64 PNG pronto para enviar ao LLM como vision
    }
  },
  {
    name: "get_content",
    description: "Extrai todo o texto visível da página atual",
    parameters: { type: "object", properties: {}, required: [] },
    async execute() {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      try {
        const result = await evaluateInTab(tab.id, "document.body.innerText");
        return { content: result };
      } catch(e) {
        return { error: e.message };
      }
    }
  },
  {
    name: "click",
    description: "Clica em elemento via seletor CSS",
    parameters: {
      type: "object",
      properties: { selector: { type: "string" } },
      required: ["selector"]
    },
    async execute({ selector }) {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      const script = `
        (() => {
          const el = document.querySelector(${JSON.stringify(selector)});
          if (!el) return { error: "Elemento não encontrado: " + ${JSON.stringify(selector)} };
          el.click();
          return { success: true, selector: ${JSON.stringify(selector)} };
        })();
      `;
      try {
        const result = await evaluateInTab(tab.id, script);
        return result;
      } catch(e) {
        return { error: e.message };
      }
    }
  },
  {
    name: "type_text",
    description: "Digita texto em um campo via seletor CSS",
    parameters: {
      type: "object",
      properties: {
        selector: { type: "string" },
        text: { type: "string" },
        clear_first: { type: "boolean", default: true }
      },
      required: ["selector", "text"]
    },
    async execute({ selector, text, clear_first }) {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      const script = `
        (() => {
          const el = document.querySelector(${JSON.stringify(selector)});
          if (!el) return { error: "Elemento não encontrado: " + ${JSON.stringify(selector)} };
          if (${clear_first}) el.value = '';
          el.focus();
          el.value = ${JSON.stringify(text)};
          el.dispatchEvent(new Event('input', { bubbles: true }));
          el.dispatchEvent(new Event('change', { bubbles: true }));
          return { success: true };
        })();
      `;
      try {
        const result = await evaluateInTab(tab.id, script);
        return result;
      } catch(e) {
        return { error: e.message };
      }
    }
  },
  {
    name: "execute_script",
    description: "Executa JavaScript arbitrário na página ativa",
    parameters: {
      type: "object",
      properties: { script: { type: "string" } },
      required: ["script"]
    },
    async execute({ script }) {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      try {
        const result = await evaluateInTab(tab.id, script);
        return { result };
      } catch (e) {
        return { error: e.message };
      }
    }
  },
  {
    name: "scroll",
    description: "Rola a página",
    parameters: {
      type: "object",
      properties: {
        direction: { type: "string", enum: ["up", "down", "top", "bottom"] },
        amount: { type: "number", default: 500 }
      },
      required: ["direction"]
    },
    async execute({ direction, amount = 500 }) {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      const script = `
        (() => {
          const dir = ${JSON.stringify(direction)};
          const amt = ${amount};
          let dx = 0, dy = 0;
          if (dir === 'up') dy = -amt;
          else if (dir === 'down') dy = amt;
          else if (dir === 'top') dy = -999999;
          else if (dir === 'bottom') dy = 999999;
          window.scrollBy(dx, dy);
          return { success: true };
        })();
      `;
      try {
        const result = await evaluateInTab(tab.id, script);
        return result;
      } catch(e) {
        return { error: e.message };
      }
    }
  },
  {
    name: "wait_for_element",
    description: "Aguarda um elemento aparecer no DOM",
    parameters: {
      type: "object",
      properties: {
        selector: { type: "string" },
        timeout_ms: { type: "number", default: 5000 }
      },
      required: ["selector"]
    },
    async execute({ selector, timeout_ms = 5000 }) {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      const script = `
        new Promise((resolve) => {
          const sel = ${JSON.stringify(selector)};
          const timeout = ${timeout_ms};
          if (document.querySelector(sel)) return resolve({ found: true });
          const obs = new MutationObserver(() => {
            if (document.querySelector(sel)) { obs.disconnect(); resolve({ found: true }); }
          });
          obs.observe(document.body, { childList: true, subtree: true });
          setTimeout(() => { obs.disconnect(); resolve({ found: false, timeout: true }); }, timeout);
        })
      `;
      try {
        const result = await evaluateInTab(tab.id, script);
        return result;
      } catch(e) {
        return { error: e.message };
      }
    }
  },
  {
    name: "get_page_info",
    description: "Retorna título, URL e metadados da página atual",
    parameters: { type: "object", properties: {}, required: [] },
    async execute() {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      return { title: tab.title, url: tab.url, tabId: tab.id };
    }
  }
];

async function waitForTabLoad(tabId) {
  return new Promise((resolve) => {
    const listener = (id, info) => {
      if (id === tabId && info.status === 'complete') {
        chrome.tabs.onUpdated.removeListener(listener);
        resolve();
      }
    };
    chrome.tabs.onUpdated.addListener(listener);
    setTimeout(resolve, 10000); // timeout 10s
  });
}
