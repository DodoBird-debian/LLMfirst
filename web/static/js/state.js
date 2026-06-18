// ── state.js — global app state ───────────────────────────────
window.State = {
  conversations: [],          // [{id, title, provider, model, updated_at}]
  activeConvId: null,         // string | null
  messages: [],               // [{id, role, content}]
  provider: 'ollama',
  model: '',
  keyId: 0,
  keys: [],                   // [{id, provider, label, key_value}]
  ollamaURL: 'http://localhost:11434',
  streaming: false,
  abortController: null,

  setProvider(p) { this.provider = p; },
  setModel(m)    { this.model = m; },
  setKeyId(k)    { this.keyId = Number(k); },

  getActiveConv() {
    return this.conversations.find(c => c.id === this.activeConvId) || null;
  },
};
