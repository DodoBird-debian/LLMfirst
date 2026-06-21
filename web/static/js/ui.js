// ── ui.js — DOM manipulation helpers ──────────────────────────
const $ = id => document.getElementById(id);

// ── Toast ──────────────────────────────────────────────────────
window.toast = function(msg, type = '') {
  const el = document.createElement('div');
  el.className = 'toast ' + type;
  el.textContent = msg;
  $('toast-container').appendChild(el);
  setTimeout(() => el.remove(), 3200);
};

// ── Conversations sidebar ──────────────────────────────────────
window.renderConversationList = function(filterText = '') {
  const list = $('conversation-list');
  const searchEl = $('conv-search');
  if (searchEl && searchEl.value) {
    filterText = searchEl.value.toLowerCase();
  }

  let convs = State.conversations;
  if (filterText) {
    convs = convs.filter(c => c.title.toLowerCase().includes(filterText));
  }

  if (!convs.length) {
    list.innerHTML = filterText ? '<div class="empty-state">No matches.</div>' : '<div class="empty-state">No conversations yet.<br/>Start a new chat above.</div>';
    return;
  }
  list.innerHTML = '';
  convs.forEach(c => {
    const item = document.createElement('div');
    item.className = 'conv-item' + (c.id === State.activeConvId ? ' active' : '');
    item.dataset.id = c.id;

    const date = new Date(c.updated_at);
    const ago = formatAgo(date);

    item.innerHTML = `
      <div class="conv-item-inner">
        <div class="conv-title">${escHtml(c.title)}</div>
        <div class="conv-meta">${escHtml(c.provider)} · ${ago}</div>
      </div>
      <div class="conv-actions">
        <button class="conv-action-btn" title="Rename" onclick="openRenameModal('${c.id}', event)">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        </button>
        <button class="conv-action-btn danger" title="Delete" onclick="deleteConversation('${c.id}', event)">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4h6v2"/></svg>
        </button>
      </div>`;

    item.addEventListener('click', () => loadConversation(c.id));
    list.appendChild(item);
  });
};

// ── Messages ───────────────────────────────────────────────────
window.renderMessages = function() {
  const msgsEl = $('messages');
  msgsEl.innerHTML = '';
  State.messages.forEach(m => appendMessage(m.role, m.content, m.id));
  msgsEl.scrollTop = msgsEl.scrollHeight;
};

window.appendMessage = function(role, content, id) {
  const msgsEl = $('messages');
  $('welcome').style.display = 'none';
  msgsEl.style.display = 'flex';

  const div = document.createElement('div');
  div.className = `msg ${role}`;
  if (id) div.dataset.msgId = id;

  const avatarChar = role === 'user' ? 'U' : '⬡';
  const label = role === 'user' ? 'You' : 'Assistant';
  const time = new Date().toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'});

  div.innerHTML = `
    <div class="msg-header">
      <div class="msg-avatar">${avatarChar}</div>
      <span>${label}</span>
      <span>${time}</span>
    </div>
    <div class="msg-bubble">${renderMarkdown(content)}</div>
    <div class="msg-actions">
      <button class="msg-action-btn" onclick="copyMsg(this)">Copy</button>
      ${role === 'user' ? '<button class="msg-action-btn" onclick="editMsg(this)">Edit</button>' : ''}
      ${role === 'assistant' ? '<button class="msg-action-btn" onclick="regenerateMsg(this)">Regenerate</button>' : ''}
    </div>`;

  msgsEl.appendChild(div);
  msgsEl.scrollTop = msgsEl.scrollHeight;
  return div;
};

window.showTypingIndicator = function() {
  const msgsEl = $('messages');
  $('welcome').style.display = 'none';
  msgsEl.style.display = 'flex';

  const wrap = document.createElement('div');
  wrap.className = 'msg assistant';
  wrap.id = 'typing-indicator';
  wrap.innerHTML = `
    <div class="msg-header">
      <div class="msg-avatar">⬡</div><span>Assistant</span>
    </div>
    <div class="typing-indicator">
      <div class="typing-dot"></div>
      <div class="typing-dot"></div>
      <div class="typing-dot"></div>
    </div>`;
  msgsEl.appendChild(wrap);
  msgsEl.scrollTop = msgsEl.scrollHeight;
};

window.removeTypingIndicator = function() {
  const el = $('typing-indicator');
  if (el) el.remove();
};

window.appendStreamBubble = function() {
  const msgsEl = $('messages');
  const div = document.createElement('div');
  div.className = 'msg assistant';
  div.id = 'stream-bubble';
  const time = new Date().toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'});
  div.innerHTML = `
    <div class="msg-header">
      <div class="msg-avatar">⬡</div><span>Assistant</span><span>${time}</span>
    </div>
    <div class="msg-bubble" id="stream-content"></div>
    <div class="msg-actions">
      <button class="msg-action-btn" onclick="copyMsg(this)">Copy</button>
    </div>`;
  msgsEl.appendChild(div);
  msgsEl.scrollTop = msgsEl.scrollHeight;
  return $('stream-content');
};

window.finalizeStreamBubble = function(fullText) {
  const bubble = $('stream-bubble');
  if (bubble) {
    bubble.removeAttribute('id');
    const content = $('stream-content');
    if (content) { content.removeAttribute('id'); content.innerHTML = renderMarkdown(fullText); }
    const msgsEl = $('messages');
    msgsEl.scrollTop = msgsEl.scrollHeight;
  }
};

// ── Copy helpers ───────────────────────────────────────────────
window.copyMsg = function(btn) {
  const bubble = btn.closest('.msg').querySelector('.msg-bubble');
  navigator.clipboard.writeText(bubble ? bubble.innerText : '').then(() => {
    btn.textContent = 'Copied!';
    setTimeout(() => btn.textContent = 'Copy', 1500);
  });
};

window.editMsg = function(btn) {
  const msgEl = btn.closest('.msg');
  const msgId = msgEl.dataset.msgId;
  const bubble = msgEl.querySelector('.msg-bubble');
  if (!bubble) return;
  // Get raw text from State
  const stateMsg = State.messages.find(m => m.id == msgId);
  if (!stateMsg) return;
  
  const input = $('msg-input');
  input.value = stateMsg.content;
  autoResize(input);
  updateCharCount();
  input.focus();
  
  // Truncate conversation to this point
  const idx = State.messages.findIndex(m => m.id == msgId);
  if (idx !== -1) {
    State.messages = State.messages.slice(0, idx);
    renderMessages();
  }
};

window.regenerateMsg = function(btn) {
  const msgEl = btn.closest('.msg');
  const msgId = msgEl.dataset.msgId;
  
  // Truncate conversation to before this assistant message
  const idx = State.messages.findIndex(m => m.id == msgId);
  if (idx !== -1) {
    State.messages = State.messages.slice(0, idx);
    renderMessages();
    // Restart stream with updated state
    const msgs = State.messages.map(m => ({role: m.role, content: m.content}));
    startStreaming(msgs);
  }
};

// ── Model selector ─────────────────────────────────────────────
window.populateModels = async function(provider) {
  const sel = $('sel-model');
  const keyId = State.keyId || 0;

  // Cloud providers require a key to list models
  const needsKey = provider !== 'ollama';
  if (needsKey && keyId === 0) {
    sel.innerHTML = '<option value="">— select a key above —</option>';
    return;
  }

  sel.innerHTML = '<option value="">Loading…</option>';
  try {
    const res = await fetch(`/api/models?provider=${provider}&keyId=${keyId}`);
    if (!res.ok) {
      const errText = await res.text();
      console.error('ListModels error:', errText);
      sel.innerHTML = '';
      const opt = document.createElement('option');
      opt.value = '';
      opt.textContent = '— key error, check console —';
      sel.appendChild(opt);
      return;
    }
    const models = await res.json();
    sel.innerHTML = '';
    if (!models || !models.length) {
      const opt = document.createElement('option');
      opt.value = ''; opt.textContent = '— no models found —';
      sel.appendChild(opt);
      return;
    }
    models.forEach(m => {
      const opt = document.createElement('option');
      opt.value = m; opt.textContent = m;
      sel.appendChild(opt);
    });
    State.setModel(sel.value);
  } catch(e) {
    console.error('populateModels exception:', e);
    sel.innerHTML = '';
    const opt = document.createElement('option');
    opt.value = ''; opt.textContent = '— network error —';
    sel.appendChild(opt);
  }
};

// ── Key selector ───────────────────────────────────────────────
window.populateKeySelector = function() {
  const sel = $('sel-key');
  const provider = State.provider;
  const filtered = State.keys.filter(k => k.provider === provider);
  sel.innerHTML = '<option value="0">— no key —</option>';
  filtered.forEach(k => {
    const opt = document.createElement('option');
    opt.value = k.id;
    opt.textContent = `${k.label} (${k.key_value})`;
    sel.appendChild(opt);
  });
  // hide for ollama (no key needed unless custom)
  $('key-selector-wrapper').style.display = provider === 'ollama' ? 'none' : '';
};

// ── Keys settings list ─────────────────────────────────────────
window.renderKeysList = function() {
  const list = $('keys-list');
  if (!State.keys.length) {
    list.innerHTML = '<div class="empty-state">No API keys stored yet.</div>';
    return;
  }
  list.innerHTML = '';
  State.keys.forEach(k => {
    const div = document.createElement('div');
    div.className = 'key-item';
    div.innerHTML = `
      <span class="key-provider-badge ${k.provider}">${k.provider}</span>
      <span class="key-label">${escHtml(k.label)}</span>
      <span class="key-value">${escHtml(k.key_value)}</span>
      <button class="key-delete-btn" title="Delete" onclick="deleteKey(${k.id})">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6"/></svg>
      </button>`;
    list.appendChild(div);
  });
};

// ── Ollama status pill ─────────────────────────────────────────
window.setOllamaPill = function(online) {
  const pill = $('ollama-status');
  pill.className = 'ollama-pill ' + (online ? 'online' : 'offline');
};

// ── Toolbar conv title ─────────────────────────────────────────
window.updateToolbarTitle = function() {
  const c = State.getActiveConv();
  const el = $('conv-title-display');
  const expBtn = $('btn-export');
  const setBtn = $('btn-chat-settings');
  if (c) {
    el.textContent = c.title;
    el.style.display = '';
  } else {
    el.style.display = 'none';
  }
  expBtn.style.display = '';
  setBtn.style.display = '';
};

// ── Rename modal ───────────────────────────────────────────────
window.openRenameModal = function(convId, e) {
  if (e) e.stopPropagation();
  const c = State.conversations.find(x => x.id === convId);
  if (!c) return;
  $('rename-input').value = c.title;
  $('modal-rename').style.display = 'flex';
  $('rename-input').focus();
  $('btn-rename-save').onclick = async () => {
    const newTitle = $('rename-input').value.trim();
    if (!newTitle) return;
    await fetch(`/api/conversations/${convId}`, {
      method: 'PUT',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({title: newTitle, system_prompt: c.system_prompt || ''})
    });
    c.title = newTitle;
    renderConversationList();
    updateToolbarTitle();
    $('modal-rename').style.display = 'none';
    toast('Conversation renamed', 'success');
  };
};

// ── Helpers ────────────────────────────────────────────────────
function escHtml(s) {
  return String(s || '').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

function formatAgo(date) {
  const diff = (Date.now() - date.getTime()) / 1000;
  if (diff < 60) return 'just now';
  if (diff < 3600) return Math.floor(diff/60) + 'm ago';
  if (diff < 86400) return Math.floor(diff/3600) + 'h ago';
  return Math.floor(diff/86400) + 'd ago';
}
