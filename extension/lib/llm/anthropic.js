export class AnthropicAdapter {
  async *chat(messages, tools, config) {
    const endpoint = 'https://api.anthropic.com/v1/messages';

    // Transform messages to Anthropic format
    const systemMessages = messages.filter(m => m.role === 'system').map(m => m.content).join('\n');
    const chatMessages = messages.filter(m => m.role !== 'system').map(m => {
      let content = m.content;

      // Handle tool results and array content structures
      if (Array.isArray(content)) {
        content = content.map(c => {
          if (c.type === 'tool_result') {
            return {
              type: 'tool_result',
              tool_use_id: c.tool_use_id,
              content: c.content,
              is_error: c.isError || false
            };
          }
          if (c.type === 'image') {
            // strip data:image/xxx;base64, prefix if present
            let data = c.data;
            if (data.startsWith('data:')) {
              data = data.split(',')[1];
            }
            return {
              type: 'image',
              source: { type: 'base64', media_type: c.mimeType || 'image/png', data }
            };
          }
          return c;
        });
      }

      return { role: m.role, content };
    });

    const body = {
      model: config.model || 'claude-3-5-sonnet-20241022',
      max_tokens: config.maxTokens || 8192,
      temperature: config.temperature !== undefined ? config.temperature : 0.7,
      stream: true,
      messages: chatMessages
    };

    if (systemMessages) body.system = systemMessages;
    if (tools && tools.length > 0) body.tools = tools;

    const response = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-api-key': config.apiKey,
        'anthropic-version': '2023-06-01',
        'anthropic-dangerous-direct-browser-access': 'true'
      },
      body: JSON.stringify(body)
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Anthropic API Error: ${response.status} ${errorText}`);
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const dataStr = line.slice(6);
          if (dataStr === '[DONE]') continue;

          try {
            const data = JSON.parse(dataStr);
            if (data.type === 'content_block_delta' && data.delta.type === 'text_delta') {
              yield { type: 'text', content: data.delta.text };
            } else if (data.type === 'content_block_start' && data.content_block.type === 'tool_use') {
              // we don't know the full input yet, it streams in
              yield { type: 'tool_use_start', id: data.content_block.id, name: data.content_block.name };
            } else if (data.type === 'content_block_delta' && data.delta.type === 'input_json_delta') {
              yield { type: 'tool_use_delta', delta: data.delta.partial_json };
            } else if (data.type === 'content_block_stop') {
              // the service worker can aggregate these if needed, or we yield a done signal
            }
          } catch (e) {
            console.warn('Error parsing SSE', e, line);
          }
        }
      }
    }
    yield { type: 'done' };
  }
}
