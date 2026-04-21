export class GeminiAdapter {
  async *chat(messages, tools, config) {
    const model = config.model || 'gemini-2.5-pro';
    const endpoint = `https://generativelanguage.googleapis.com/v1beta/models/${model}:streamGenerateContent?key=${config.apiKey}`;

    // Convert to Gemini format
    const contents = [];
    let systemInstruction = null;

    for (const m of messages) {
      if (m.role === 'system') {
        systemInstruction = { parts: [{ text: m.content }] };
        continue;
      }

      const role = m.role === 'assistant' ? 'model' : 'user';
      let parts = [];

      if (typeof m.content === 'string') {
        parts.push({ text: m.content });
      } else if (Array.isArray(m.content)) {
        for (const c of m.content) {
          if (c.type === 'text') parts.push({ text: c.text });
          if (c.type === 'image') {
            let data = c.data;
            if (data.startsWith('data:')) {
              data = data.split(',')[1];
            }
            parts.push({ inlineData: { mimeType: c.mimeType || 'image/png', data } });
          }
          if (c.type === 'tool_result') {
            parts.push({
              functionResponse: {
                name: c.tool_use_id, // Gemini uses name as identifier for response
                response: c.isError ? { error: c.content } : JSON.parse(c.content)
              }
            });
          }
        }
      }

      if (parts.length > 0) contents.push({ role, parts });
    }

    const body = {
      contents,
      generationConfig: {
        temperature: config.temperature !== undefined ? config.temperature : 0.7,
        maxOutputTokens: config.maxTokens || 8192
      }
    };

    if (systemInstruction) body.systemInstruction = systemInstruction;
    if (tools && tools.length > 0) body.tools = tools;

    const response = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Gemini API Error: ${response.status} ${errorText}`);
    }

    // Process array of Server-Sent Events stream from Gemini
    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // Basic split on array bracket boundaries for JSON chunks (simplified, usually streams objects)
      // Since Gemini returns a JSON array over stream, we find objects
      let openBraces = 0;
      let startIdx = -1;

      for (let i = 0; i < buffer.length; i++) {
        if (buffer[i] === '{') {
          if (openBraces === 0) startIdx = i;
          openBraces++;
        } else if (buffer[i] === '}') {
          openBraces--;
          if (openBraces === 0 && startIdx !== -1) {
            const jsonStr = buffer.substring(startIdx, i + 1);
            try {
              const data = JSON.parse(jsonStr);
              if (data.candidates && data.candidates[0].content && data.candidates[0].content.parts) {
                for (const part of data.candidates[0].content.parts) {
                  if (part.text) {
                    yield { type: 'text', content: part.text };
                  }
                  if (part.functionCall) {
                    yield {
                      type: 'tool_use',
                      id: part.functionCall.name,
                      name: part.functionCall.name,
                      input: part.functionCall.args
                    };
                  }
                }
              }
            } catch (e) {
              // Not complete JSON or not an object we care about
            }
            startIdx = -1; // reset
          }
        }
      }
      if (openBraces === 0 && startIdx === -1) {
          buffer = ''; // clear processed
      } else if (startIdx !== -1) {
          buffer = buffer.substring(startIdx); // keep incomplete
      }
    }
    yield { type: 'done' };
  }
}
