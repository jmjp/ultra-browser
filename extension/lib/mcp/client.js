export class MCPClient {
  constructor(serverUrl) {
    this.serverUrl = serverUrl;
    this.tools = [];
  }

  async initialize() {
    const response = await this._call('tools/list', {});
    this.tools = response.tools || [];
    return this.tools;
  }

  async callTool(name, args) {
    return this._call('tools/call', { name, arguments: args });
  }

  async _call(method, params) {
    const res = await fetch(this.serverUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ jsonrpc: '2.0', id: Date.now(), method, params })
    });
    const json = await res.json();
    if (json.error) throw new Error(json.error.message);
    return json.result;
  }
}
