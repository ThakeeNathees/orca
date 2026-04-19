// Hardcoded provider → model catalogue used by the model creation dialog
// and the Configuration tab. Swap with a provider-side manifest when the
// backend lands.

import type { ModelProvider } from "./types";

export const PROVIDER_MODELS: Record<ModelProvider, string[]> = {
  openai: ["gpt-4o", "gpt-4o-mini", "o1", "o1-mini"],
  anthropic: ["claude-sonnet-4-6", "claude-opus-4-7", "claude-haiku-4-5"],
  gemini: ["gemini-2.5-pro", "gemini-2.5-flash"],
  ollama: ["llama3.1", "qwen2.5"],
};

export const PROVIDER_LABELS: Record<ModelProvider, string> = {
  openai: "OpenAI",
  anthropic: "Claude (Anthropic)",
  gemini: "Gemini",
  ollama: "Ollama",
};
