import { AnthropicAdapter } from './anthropic.js';
import { OpenAIAdapter } from './openai.js';
import { GeminiAdapter } from './gemini.js';

export function getAdapter(config) {
  if (!config || !config.provider) {
    throw new Error("Provider not configured");
  }

  switch (config.provider.toLowerCase()) {
    case 'anthropic':
      return new AnthropicAdapter();
    case 'openai':
    case 'deepseek':
    case 'custom':
      return new OpenAIAdapter();
    case 'gemini':
      return new GeminiAdapter();
    default:
      throw new Error(`Unsupported provider: ${config.provider}`);
  }
}

export function buildToolsForProvider(tools, provider) {
  provider = provider.toLowerCase();

  if (provider === 'anthropic') {
    return tools.map(t => ({
      name: t.name,
      description: t.description,
      input_schema: {
        type: t.parameters?.type || "object",
        properties: t.parameters?.properties || {},
        required: t.parameters?.required || []
      }
    }));
  }

  if (provider === 'openai' || provider === 'deepseek' || provider === 'custom') {
    return tools.map(t => ({
      type: "function",
      function: {
        name: t.name,
        description: t.description,
        parameters: {
          type: t.parameters?.type || "object",
          properties: t.parameters?.properties || {},
          required: t.parameters?.required || []
        }
      }
    }));
  }

  if (provider === 'gemini') {
    return [{
      functionDeclarations: tools.map(t => ({
        name: t.name,
        description: t.description,
        parameters: {
          type: t.parameters?.type?.toUpperCase() || "OBJECT",
          properties: t.parameters?.properties || {},
          required: t.parameters?.required || []
        }
      }))
    }];
  }

  return tools;
}
