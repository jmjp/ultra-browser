// Ultra Browser - Content Script

console.log("Ultra Browser Content Script carregado.");

// Escuta mensagens do background script
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  (async () => {
    try {
      switch (request.action) {
        case "extractContent":
          sendResponse({
            text: document.body.innerText,
            url: window.location.href,
            title: document.title
          });
          break;

        case "capture_node":
          handleCaptureNode(request.params, sendResponse);
          break;

        case "type_text":
          sendResponse(await handleTypeText(request.params));
          break;

        case "wait_for_element":
          sendResponse(await handleWaitForElement(request.params));
          break;

        case "get_value":
          sendResponse(handleGetValue(request.params));
          break;

        case "select_option":
          sendResponse(handleSelectOption(request.params));
          break;

        case "upload_file":
          sendResponse(handleUploadFile(request.params));
          break;

        case "scroll":
          sendResponse(handleScroll(request.params));
          break;

        case "hover":
          sendResponse(handleHover(request.params));
          break;

        default:
          sendResponse({ error: `Ação desconhecida: ${request.action}` });
      }
    } catch (err) {
      sendResponse({ error: err.message });
    }
  })();
  return true; // Mantém o canal aberto para respostas assíncronas
});

function handleCaptureNode(params, sendResponse) {
  const { selector, format } = params;
  const element = document.querySelector(selector);
  
  if (!element) {
    sendResponse({ error: `Elemento não encontrado: ${selector}` });
    return;
  }

  if (format === "html") {
    sendResponse({ content: element.outerHTML });
  } else if (format === "png") {
    const rect = element.getBoundingClientRect();
    sendResponse({
      rect: {
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        devicePixelRatio: window.devicePixelRatio || 1
      }
    });
  } else {
    sendResponse({ error: `Formato não suportado: ${format}` });
  }
}

async function handleTypeText({ selector, text }) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Elemento não encontrado: ${selector}`);
  
  element.focus();
  element.value = text;
  
  // Dispara uma sequência de eventos para simular a digitação o mais fielmente possível
  element.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true }));
  element.dispatchEvent(new KeyboardEvent('keypress', { bubbles: true }));
  element.dispatchEvent(new Event('input', { bubbles: true }));
  element.dispatchEvent(new KeyboardEvent('keyup', { bubbles: true }));
  element.dispatchEvent(new Event('change', { bubbles: true }));
  
  return { success: true };
}

async function handleWaitForElement({ selector, timeout = 10000 }) {
  if (document.querySelector(selector)) return { success: true };

  return new Promise((resolve) => {
    const observer = new MutationObserver(() => {
      if (document.querySelector(selector)) {
        observer.disconnect();
        clearTimeout(timer);
        resolve({ success: true });
      }
    });

    observer.observe(document.body, { childList: true, subtree: true });

    const timer = setTimeout(() => {
      observer.disconnect();
      resolve({ error: `Timeout aguardando elemento: ${selector} após ${timeout}ms` });
    }, timeout);
  });
}

function handleGetValue({ selector }) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Elemento não encontrado: ${selector}`);
  return { value: element.value || element.innerText };
}

function handleSelectOption({ selector, value }) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Elemento não encontrado: ${selector}`);
  
  if (element.tagName !== "SELECT") {
    throw new Error(`Elemento não é um SELECT: ${selector}`);
  }

  element.value = value;
  element.dispatchEvent(new Event('change', { bubbles: true }));
  return { success: true };
}

function handleUploadFile({ selector, base64, filename }) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Elemento não encontrado: ${selector}`);
  if (element.type !== "file") throw new Error(`Elemento não é um input de arquivo: ${selector}`);

  const byteCharacters = atob(base64);
  const byteNumbers = new Array(byteCharacters.length);
  for (let i = 0; i < byteCharacters.length; i++) {
    byteNumbers[i] = byteCharacters.charCodeAt(i);
  }
  const byteArray = new Uint8Array(byteNumbers);
  const blob = new Blob([byteArray]);
  const file = new File([blob], filename || 'uploaded_file');

  const dataTransfer = new DataTransfer();
  dataTransfer.items.add(file);
  element.files = dataTransfer.files;
  
  element.dispatchEvent(new Event('change', { bubbles: true }));
  return { success: true };
}

function handleScroll({ selector, x, y }) {
  if (selector) {
    const element = document.querySelector(selector);
    if (!element) throw new Error(`Elemento não encontrado: ${selector}`);
    element.scrollIntoView({ behavior: 'smooth', block: 'center' });
  } else {
    window.scrollTo({ left: x || 0, top: y || 0, behavior: 'smooth' });
  }
  return { success: true };
}

function handleHover({ selector }) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Elemento não encontrado: ${selector}`);

  const events = ['mouseover', 'mouseenter', 'mousemove'];
  events.forEach(type => {
    const event = new MouseEvent(type, {
      view: window,
      bubbles: true,
      cancelable: true
    });
    element.dispatchEvent(event);
  });
  return { success: true };
}
