let socket = null;
const pendingRequests = new Map();
let reconnectDelay = 1000; // Começa com 1 segundo
const maxReconnectDelay = 30000; // Máximo de 30 segundos
let reconnectTimeoutId = null;

// Mantém o Service Worker vivo
chrome.alarms.create('keepAlive', { periodInMinutes: 0.1 });
chrome.alarms.onAlarm.addListener((alarm) => {
    if (alarm.name === 'keepAlive' && (!socket || socket.readyState !== WebSocket.OPEN)) {
        console.log("KeepAlive: Verificando conexão...");
        connectWebSocket();
    }
});

function connectWebSocket() {
    if (reconnectTimeoutId) {
        clearTimeout(reconnectTimeoutId);
        reconnectTimeoutId = null;
    }

    if (socket && (socket.readyState === WebSocket.CONNECTING || socket.readyState === WebSocket.OPEN)) return;
    
    console.log("Conectando ao WebSocket: ws://localhost:12306/ws");
    socket = new WebSocket("ws://localhost:12306/ws");

    socket.onopen = () => {
        console.log("WebSocket conectado com sucesso");
        reconnectDelay = 1000; // Reseta o backoff ao conectar com sucesso
    };

    socket.onmessage = async (event) => {
        try {
            const msg = JSON.parse(event.data);
            
            // Se for uma requisição do Go (tool call)
            if (msg.tool) {
                console.log("Executando tool:", msg.tool, msg.params);
                const result = await executeTool(msg.tool, msg.params);
                const response = { id: msg.id };
                if (result && result.error) {
                    response.error = result.error;
                } else {
                    response.result = result;
                }
                socket.send(JSON.stringify(response));
                return;
            }

            // Se for uma resposta do Go para o Chrome
            const cb = pendingRequests.get(msg.id);
            if (cb) {
                cb(msg);
                pendingRequests.delete(msg.id);
            }
        } catch (err) {
            console.error("Erro ao processar mensagem do WebSocket:", err);
        }
    };

    socket.onclose = () => {
        console.warn(`WebSocket desconectado. Tentando reconectar em ${reconnectDelay / 1000}s...`);
        socket = null;
        
        // Agenda a reconexão com backoff exponencial
        reconnectTimeoutId = setTimeout(() => {
            connectWebSocket();
        }, reconnectDelay);
        
        // Aumenta o delay para a próxima tentativa (dobra, até o máximo)
        reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
    };

    socket.onerror = (err) => {
        console.error("Erro no WebSocket:", err);
        // O onclose será chamado automaticamente após o onerror
    };
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
                return { success: true, content: dataUrl.split(',')[1], format: "png" };
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
                const target = { tabId: tab.id };
                try {
                    await chrome.debugger.attach(target, "1.3");
                    const response = await new Promise((resolve, reject) => {
                        chrome.debugger.sendCommand(target, "Runtime.evaluate", {
                            expression: `(async () => { ${params.script} })()`,
                            returnByValue: true,
                            awaitPromise: true,
                            userGesture: true
                        }, (resp) => {
                            if (chrome.runtime.lastError) {
                                reject(new Error(chrome.runtime.lastError.message));
                            } else if (resp.exceptionDetails) {
                                reject(new Error(resp.exceptionDetails.exception.description || "Erro na execução do script"));
                            } else {
                                resolve(resp.result.value);
                            }
                        });
                    });
                    await chrome.debugger.detach(target);
                    return { result: response };
                } catch (err) {
                    try { await chrome.debugger.detach(target); } catch (e) {}
                    return { error: err.message };
                }
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
        sendResponse({ 
            connected: socket?.readyState === WebSocket.OPEN, 
            hostName: "WebSocket (ws://localhost:12306/ws)" 
        });
    }
    return true;
});

// Inicializa a conexão
connectWebSocket();

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
