let nativePort = null;
const pendingRequests = new Map();

// Mantém o Service Worker vivo
chrome.alarms.create('keepAlive', { periodInMinutes: 0.1 });
chrome.alarms.onAlarm.addListener((alarm) => {
    if (alarm.name === 'keepAlive' && !nativePort) {
        connectNative();
    }
});

function connectNative() {
    if (nativePort) return;
    
    console.log("Conectando ao host nativo: com.ultra_browser.host");
    nativePort = chrome.runtime.connectNative("com.ultra_browser.host");

    nativePort.onMessage.addListener(async (msg) => {
        // Se for uma requisição do Go (tool call)
        if (msg.tool) {
            console.log("Executando tool:", msg.tool, msg.params);
            const result = await executeTool(msg.tool, msg.params);
            if (result && result.error) {
                nativePort.postMessage({ id: msg.id, error: result.error });
            } else {
                nativePort.postMessage({ id: msg.id, result });
            }
            return;
        }

        // Se for uma resposta do Go para o Chrome
        const cb = pendingRequests.get(msg.id);
        if (cb) {
            cb(msg);
            pendingRequests.delete(msg.id);
        }
    });

    nativePort.onDisconnect.addListener(() => {
        console.warn("Host nativo desconectado:", chrome.runtime.lastError?.message);
        nativePort = null;
    });
}

async function executeTool(tool, params) {
    try {
        const [tab] = await chrome.tabs.query({ active: true });
        if (!tab && tool !== "list_tabs" && tool !== "navigate") {
            return { error: "Nenhuma aba ativa encontrada" };
        }

        switch (tool) {
            case "list_tabs": {
                const tabs = await chrome.tabs.query({});
                return { tabs: tabs.map(t => ({ id: t.id, title: t.title, url: t.url })) };
            }
            case "navigate": {
                const newTab = await chrome.tabs.create({ url: params.url });
                return { success: true, tabId: newTab.id, status: "loading" };
            }
            case "screenshot": {
                const dataUrl = await chrome.tabs.captureVisibleTab(null, { format: "png" });
                return { success: true, base64: dataUrl.split(',')[1] };
            }
            case "capture_node": {
                const [result] = await chrome.scripting.executeScript({
                    target: { tabId: tab.id },
                    func: (params) => {
                        const { selector, format } = params;
                        const element = document.querySelector(selector);
                        if (!element) return { error: `Elemento não encontrado: ${selector}` };

                        if (format === "html") {
                            return { content: element.outerHTML };
                        } else if (format === "png") {
                            const rect = element.getBoundingClientRect();
                            return {
                                rect: {
                                    x: rect.left,
                                    y: rect.top,
                                    width: rect.width,
                                    height: rect.height,
                                    devicePixelRatio: window.devicePixelRatio || 1
                                }
                            };
                        }
                        return { error: `Formato inválido: ${format}` };
                    },
                    args: [params],
                });

                const response = result.result;
                if (response.error) return response;

                if (params.format === "html") {
                    return { success: true, content: response.content, format: "html" };
                } else if (params.format === "png") {
                    const dataUrl = await chrome.tabs.captureVisibleTab(null, { format: "png" });
                    if (response.rect) {
                        const croppedBase64 = await cropImage(dataUrl, response.rect);
                        return { success: true, content: croppedBase64, format: "png" };
                    }
                    return { success: true, content: dataUrl.split(',')[1], format: "png" };
                }
                return { error: `Formato inválido: ${params.format}` };
            }
            case "type_text":
            case "wait_for_element":
            case "get_value":
            case "select_option":
            case "upload_file":
            case "scroll":
            case "hover": {
                // Delega ao content.js via sendToContentScript, que injeta o script
                // programaticamente caso a aba não o tenha (ex: aba já estava aberta).
                return await sendToContentScript(tab.id, { action: tool, params });
            }
            case "get_content": {
                const [result] = await chrome.scripting.executeScript({
                    target: { tabId: tab.id },
                    func: () => document.body.innerText,
                });
                return { success: true, content: result.result };
            }
            case "click": {
                const [result] = await chrome.scripting.executeScript({
                    target: { tabId: tab.id },
                    func: (selector) => {
                        const el = document.querySelector(selector);
                        if (el) {
                            el.click();
                            return { success: true };
                        }
                        return { error: `Elemento não encontrado: ${selector}` };
                    },
                    args: [params.selector],
                });
                return result.result;
            }
            case "execute_script": {
                const [result] = await chrome.scripting.executeScript({
                    target: { tabId: tab.id },
                    world: "MAIN",
                    func: (code) => {
                        try {
                            return { result: eval(code) };
                        } catch (err) {
                            return { error: err.message };
                        }
                    },
                    args: [params.script],
                });
                return result.result;
            }
            case "switch_tab": {
                const tabId = parseInt(params.tab_id);
                if (isNaN(tabId)) return { error: "ID da aba inválido" };
                await chrome.tabs.update(tabId, { active: true });
                // Opcional: trazer a janela para frente também
                const tab = await chrome.tabs.get(tabId);
                if (tab.windowId) {
                    await chrome.windows.update(tab.windowId, { focused: true });
                }
                return { success: true, message: `Aba ${tabId} agora está ativa` };
            }
            default:
                return { error: `Ferramenta não suportada: ${tool}` };
        }
    } catch (err) {
        return { error: err.message };
    }
}

/**
 * Envia uma mensagem para o content script da aba.
 * Se o content script não estiver injetado (aba aberta antes da extensão),
 * injeta-o programaticamente e tenta novamente.
 */
async function sendToContentScript(tabId, message) {
    try {
        return await chrome.tabs.sendMessage(tabId, message);
    } catch (err) {
        // "Could not establish connection" = content script não está na aba
        if (!err.message?.includes('Could not establish connection')) throw err;

        console.log(`Content script não encontrado na aba ${tabId}. Injetando...`);
        await chrome.scripting.executeScript({
            target: { tabId },
            files: ['content.js'],
        });

        // Pequena pausa para o script inicializar antes de reenviar
        await new Promise(r => setTimeout(r, 100));
        return await chrome.tabs.sendMessage(tabId, message);
    }
}

// Escuta mensagens do popup
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    if (msg.action === "getStatus") {
        sendResponse({ connected: !!nativePort, hostName: "com.ultra_browser.host" });
    }
    return true;
});

// Inicializa a conexão
connectNative();

/**
 * Recorta uma imagem em Base64 usando OffscreenCanvas.
 */
async function cropImage(dataUrl, rect) {
    const response = await fetch(dataUrl);
    const blob = await response.blob();
    const imageBitmap = await createImageBitmap(blob);
    
    const dpr = rect.devicePixelRatio || 1;
    const canvas = new OffscreenCanvas(rect.width * dpr, rect.height * dpr);
    const ctx = canvas.getContext('2d');
    
    ctx.drawImage(
        imageBitmap,
        rect.x * dpr, rect.y * dpr, rect.width * dpr, rect.height * dpr, // Origem
        0, 0, rect.width * dpr, rect.height * dpr // Destino
    );
    
    const croppedBlob = await canvas.convertToBlob({ type: 'image/png' });
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onloadend = () => resolve(reader.result.split(',')[1]);
        reader.onerror = reject;
        reader.readAsDataURL(croppedBlob);
    });
}
