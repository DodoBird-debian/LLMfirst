// ── main.js — app initialization & event binding ───────────────
document.addEventListener('DOMContentLoaded', async () => {

  // ── Initial data load ──────────────────────────────────────
  await Promise.all([loadConversations(), loadKeys(), checkOllama()]);
  await populateModels(State.provider);
  populateKeySelector();

  // ── Provider selector ──────────────────────────────────────
  document.getElementById('sel-provider').addEventListener('change', async e => {
    State.setProvider(e.target.value);
    await populateModels(e.target.value);
    populateKeySelector();
  });

  document.getElementById('sel-model').addEventListener('change', e => {
    State.setModel(e.target.value);
  });

  document.getElementById('sel-key').addEventListener('change', async e => {
    State.setKeyId(e.target.value);
    await populateModels(State.provider);
  });

  // ── New chat ───────────────────────────────────────────────
  document.getElementById('btn-new-chat').addEventListener('click', startNewChat);

  // ── Send / Stop ────────────────────────────────────────────
  document.getElementById('btn-send').addEventListener('click', sendMessage);
  document.getElementById('btn-stop').addEventListener('click', () => {
    if (State.abortController) State.abortController.abort();
  });

  // ── Textarea keyboard shortcuts ────────────────────────────
  const input = document.getElementById('msg-input');
  input.addEventListener('keydown', e => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });
  input.addEventListener('input', () => {
    autoResize(input);
    updateCharCount();
  });

  // ── Ctrl+K: new chat ───────────────────────────────────────
  document.addEventListener('keydown', e => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      startNewChat();
    }
  });

  // ── Sidebar toggle ─────────────────────────────────────────
  document.getElementById('btn-sidebar-toggle').addEventListener('click', () => {
    document.getElementById('sidebar').classList.toggle('collapsed');
  });

  // ── Settings modal ─────────────────────────────────────────
  document.getElementById('btn-settings').addEventListener('click', () => {
    document.getElementById('modal-settings').style.display = 'flex';
    loadKeys();
    refreshOllamaSettings();
  });

  document.getElementById('btn-close-settings').addEventListener('click', () => {
    document.getElementById('modal-settings').style.display = 'none';
  });

  document.getElementById('modal-settings').addEventListener('click', e => {
    if (e.target === document.getElementById('modal-settings')) {
      document.getElementById('modal-settings').style.display = 'none';
    }
  });

  // ── Modal tabs ─────────────────────────────────────────────
  document.querySelectorAll('.modal-tab').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.modal-tab').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      document.querySelectorAll('.modal-tab-content').forEach(c => c.style.display = 'none');
      document.getElementById('tab-' + tab.dataset.tab).style.display = 'flex';
    });
  });

  // ── Add API key ────────────────────────────────────────────
  document.getElementById('btn-add-key').addEventListener('click', async () => {
    const provider = document.getElementById('new-key-provider').value;
    const label    = document.getElementById('new-key-label').value.trim();
    const keyValue = document.getElementById('new-key-value').value.trim();
    const baseURL  = document.getElementById('new-key-baseurl').value.trim();
    if (!label || !keyValue) { toast('Label and key are required', 'error'); return; }
    const res = await fetch('/api/keys', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({provider, label, key_value: keyValue, base_url: baseURL}),
    });
    if (res.ok) {
      document.getElementById('new-key-label').value = '';
      document.getElementById('new-key-value').value = '';
      document.getElementById('new-key-baseurl').value = '';
      await loadKeys();
      toast('API key added', 'success');
    } else {
      toast('Failed to add key', 'error');
    }
  });

  // ── Ollama URL ─────────────────────────────────────────────
  document.getElementById('btn-set-ollama-url').addEventListener('click', async () => {
    const url = document.getElementById('ollama-url-input').value.trim();
    if (!url) return;
    const res = await fetch('/api/ollama/url', {
      method: 'PUT',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({url}),
    });
    if (res.ok) {
      toast('Ollama URL updated', 'success');
      refreshOllamaSettings();
      checkOllama();
    } else {
      toast('Failed to reach Ollama at that URL', 'error');
    }
  });

  // ── Rename modal close ─────────────────────────────────────
  document.getElementById('btn-close-rename').addEventListener('click', () => {
    document.getElementById('modal-rename').style.display = 'none';
  });

  document.getElementById('modal-rename').addEventListener('click', e => {
    if (e.target === document.getElementById('modal-rename')) {
      document.getElementById('modal-rename').style.display = 'none';
    }
  });

  document.getElementById('rename-input').addEventListener('keydown', e => {
    if (e.key === 'Enter') document.getElementById('btn-rename-save').click();
    if (e.key === 'Escape') document.getElementById('modal-rename').style.display = 'none';
  });

  // ── Conv title click → rename ──────────────────────────────
  document.getElementById('conv-title-display').addEventListener('click', () => {
    if (State.activeConvId) openRenameModal(State.activeConvId);
  });

  // ── Export ─────────────────────────────────────────────────
  document.getElementById('btn-export').addEventListener('click', exportConversation);

  // ── Welcome chips ──────────────────────────────────────────
  document.querySelectorAll('.chip').forEach(chip => {
    chip.addEventListener('click', () => {
      input.value = chip.dataset.prompt;
      autoResize(input);
      updateCharCount();
      input.focus();
    });
  });
});

// ── API helpers ────────────────────────────────────────────────
async function loadConversations() {
  try {
    const res = await fetch('/api/conversations');
    State.conversations = await res.json() || [];
    renderConversationList();
  } catch {}
}

async function checkOllama() {
  try {
    const res = await fetch('/api/ollama/status');
    const data = await res.json();
    setOllamaPill(data.detected);
  } catch {
    setOllamaPill(false);
  }
}

async function refreshOllamaSettings() {
  try {
    const res = await fetch('/api/ollama/status');
    const data = await res.json();
    const dot = document.getElementById('ollama-dot');
    const text = document.getElementById('ollama-status-text');
    const info = document.getElementById('ollama-models-info');
    const urlInput = document.getElementById('ollama-url-input');

    dot.className = 'status-dot ' + (data.detected ? 'ok' : 'err');
    text.textContent = data.detected
      ? `Connected: ${data.url}`
      : `Not reachable: ${data.url}`;
    info.textContent = data.detected && data.models
      ? `${data.models.length} model(s) available`
      : '';
    urlInput.value = data.url || '';
  } catch {}
}
