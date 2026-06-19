// ── main.js — app initialization & event binding ───────────────
document.addEventListener('DOMContentLoaded', async () => {

  // ── Initial data load (checked with Auth) ───────────────────
  await window.checkAuth();
  if (window.AuthState.authenticated) {
    await Promise.all([loadConversations(), loadKeys(), checkOllama()]);
    await populateModels(State.provider);
    populateKeySelector();
  }

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

  // ── Sidebar search ─────────────────────────────────────────
  const searchInput = document.getElementById('conv-search');
  if (searchInput) {
    searchInput.addEventListener('input', () => renderConversationList());
  }

  // ── Theme toggle ───────────────────────────────────────────
  const themeBtn = document.getElementById('btn-theme-toggle');
  if (themeBtn) {
    themeBtn.addEventListener('click', () => {
      const isLight = document.body.dataset.theme === 'light';
      document.body.dataset.theme = isLight ? 'dark' : 'light';
      localStorage.setItem('theme', document.body.dataset.theme);
    });
    const savedTheme = localStorage.getItem('theme');
    if (savedTheme) document.body.dataset.theme = savedTheme;
  }

  // ── Chat Settings ──────────────────────────────────────────
  const chatSetBtn = document.getElementById('btn-chat-settings');
  if (chatSetBtn) {
    chatSetBtn.addEventListener('click', () => {
      const c = State.getActiveConv();
      if (!c) return;
      document.getElementById('chat-system-prompt').value = c.system_prompt || '';
      document.getElementById('chat-temperature').value = State.temperature || 0.7;
      document.getElementById('val-temperature').textContent = State.temperature || 0.7;
      document.getElementById('chat-topp').value = State.topp || 1.0;
      document.getElementById('val-topp').textContent = State.topp || 1.0;
      document.getElementById('chat-maxtokens').value = State.max_tokens || '';
      document.getElementById('modal-chat-settings').style.display = 'flex';
    });
  }

  const closeChatSetBtn = document.getElementById('btn-close-chat-settings');
  if (closeChatSetBtn) {
    closeChatSetBtn.addEventListener('click', () => document.getElementById('modal-chat-settings').style.display = 'none');
  }

  const saveChatSetBtn = document.getElementById('btn-save-chat-settings');
  if (saveChatSetBtn) {
    saveChatSetBtn.addEventListener('click', async () => {
      const sysPrompt = document.getElementById('chat-system-prompt').value;
      const temp = parseFloat(document.getElementById('chat-temperature').value);
      const topP = parseFloat(document.getElementById('chat-topp').value);
      const maxTok = parseInt(document.getElementById('chat-maxtokens').value) || 0;
      
      State.temperature = temp;
      State.topp = topP;
      State.max_tokens = maxTok;
      
      const c = State.getActiveConv();
      if (c) {
        c.system_prompt = sysPrompt;
        await fetch(`/api/conversations/${c.id}`, {
          method: 'PUT',
          headers: {'Content-Type':'application/json'},
          body: JSON.stringify({title: c.title, system_prompt: sysPrompt})
        });
      }
      
      document.getElementById('modal-chat-settings').style.display = 'none';
      toast('Chat settings saved', 'success');
    });
  }
  
  // Real-time slider updates
  const tempSlider = document.getElementById('chat-temperature');
  if (tempSlider) {
    tempSlider.addEventListener('input', e => document.getElementById('val-temperature').textContent = e.target.value);
  }
  const toppSlider = document.getElementById('chat-topp');
  if (toppSlider) {
    toppSlider.addEventListener('input', e => document.getElementById('val-topp').textContent = e.target.value);
  }

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
    const isSharedCheck = document.getElementById('new-key-shared');
    const isShared = isSharedCheck ? isSharedCheck.checked : false;

    if (!label || !keyValue) { toast('Label and key are required', 'error'); return; }
    const res = await fetch('/api/keys', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({provider, label, key_value: keyValue, base_url: baseURL, is_shared: isShared}),
    });
    if (res.ok) {
      document.getElementById('new-key-label').value = '';
      document.getElementById('new-key-value').value = '';
      document.getElementById('new-key-baseurl').value = '';
      if (isSharedCheck) isSharedCheck.checked = false;
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
