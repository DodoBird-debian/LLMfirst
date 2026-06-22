// ── chat.js — SSE streaming + conversation management ──────────

// Send a message and stream the response
window.sendMessage = async function() {
  const input = document.getElementById('msg-input');
  const text = input.value.trim();
  if (!text || State.streaming) return;
  if (!State.model) { toast('Please select a model first', 'error'); return; }

  // Instantly lock UI to prevent duplicate submissions on rapid clicks
  State.streaming = true;
  toggleSendStop(true);

  // Ensure we have an active conversation
  if (!State.activeConvId) {
    const conv = await createNewConversation();
    if (!conv) { 
      toast('Failed to create conversation', 'error'); 
      State.streaming = false;
      toggleSendStop(false);
      return; 
    }
  }

  // Add user message to UI
  appendMessage('user', text);
  State.messages.push({role:'user', content: text});
  input.value = '';
  autoResize(input);
  updateCharCount();

  // Build messages array for API
  let msgs = State.messages.map(m => ({role: m.role, content: m.content}));
  const c = State.getActiveConv();
  if (c && c.system_prompt) {
    msgs.unshift({role: 'system', content: c.system_prompt});
  }

  startStreaming(msgs);
};

window.startStreaming = async function(msgs) {
  State.abortController = new AbortController();
  removeTypingIndicator();
  showTypingIndicator();

  try {
    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({
        conversationId: State.activeConvId,
        provider: State.provider,
        model: State.model,
        keyId: State.keyId,
        messages: msgs,
        temperature: State.temperature,
        top_p: State.topp,
        max_tokens: State.max_tokens,
      }),
      signal: State.abortController.signal,
    });

    removeTypingIndicator();

    if (!res.ok) {
      const errText = await res.text();
      toast('API error: ' + errText, 'error');
      return;
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    const contentEl = appendStreamBubble();
    let fullText = '';
    let buffer = '';

    while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, {stream: true});
      const lines = buffer.split('\n');
      buffer = lines.pop(); // keep incomplete line

      for (const line of lines) {
        if (!line.startsWith('data: ')) continue;
        const data = line.slice(6);
        if (data === '[DONE]') break;
        if (!data || data.startsWith('{"error"')) {
          try {
            const err = JSON.parse(data);
            if (err.error) toast('Error: ' + err.error, 'error');
          } catch {}
          continue;
        }
        fullText += data;
        // Render progressively
        contentEl.innerHTML = renderMarkdown(fullText);
        const msgsEl = document.getElementById('messages');
        msgsEl.scrollTop = msgsEl.scrollHeight;
      }
    }

    finalizeStreamBubble(fullText);
    State.messages.push({role:'assistant', content: fullText});

    // Auto-title from first exchange
    const conv = State.getActiveConv();
    if (conv && conv.title === 'New Chat' && State.messages.length <= 3) {
      const autoTitle = msgs[0].content.slice(0,48).replace(/\n/g,' ') + (msgs[0].content.length > 48 ? '…' : '');
      await fetch(`/api/conversations/${State.activeConvId}`, {
        method:'PUT',
        headers:{'Content-Type':'application/json'},
        body: JSON.stringify({title: autoTitle, system_prompt: ''})
      });
      conv.title = autoTitle;
      renderConversationList();
      updateToolbarTitle();
    }

  } catch (err) {
    removeTypingIndicator();
    if (err.name !== 'AbortError') {
      toast('Stream error: ' + err.message, 'error');
    }
  } finally {
    State.streaming = false;
    State.abortController = null;
    toggleSendStop(false);
  }
};

// ── Conversation lifecycle ─────────────────────────────────────
window.createNewConversation = async function() {
  try {
    const sysPrompt = State.pendingSystemPrompt || '';
    const res = await fetch('/api/conversations', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({provider: State.provider, model: State.model, system_prompt: sysPrompt}),
    });
    State.pendingSystemPrompt = '';
    if (!res.ok) return null;
    const conv = await res.json();
    State.conversations.unshift(conv);
    State.activeConvId = conv.id;
    State.messages = [];
    renderConversationList();
    updateToolbarTitle();
    return conv;
  } catch { return null; }
};

window.loadConversation = async function(id) {
  if (State.streaming) { toast('Please wait for the current response to finish', ''); return; }
  try {
    const res = await fetch(`/api/conversations/${id}`);
    const data = await res.json();
    State.activeConvId = id;
    State.messages = data.messages || [];

    // Switch provider/model selectors to match conversation
    const c = data.conversation;
    if (c) {
      document.getElementById('sel-provider').value = c.provider;
      State.setProvider(c.provider);
      await populateModels(c.provider);
      document.getElementById('sel-model').value = c.model;
      State.setModel(c.model);
      populateKeySelector();
    }

    renderConversationList();
    renderMessages();
    updateToolbarTitle();

    // Show chat area
    document.getElementById('welcome').style.display = 'none';
    document.getElementById('messages').style.display = 'flex';

    // Clear ghost attachments
    const preview = document.getElementById('attachment-preview');
    if (preview) {
      preview.innerHTML = '';
      preview.style.display = 'none';

      // Rebuild attachments if any exist
      if (data.attachments && data.attachments.length > 0) {
        preview.style.display = 'flex';
        data.attachments.forEach(att => {
          const pillId = 'pill-' + att.id;
          const pill = document.createElement('div');
          pill.className = 'attachment-pill';
          pill.id = pillId;
          pill.innerHTML = `
            <span>📎 ${att.filename}</span>
            <button class="remove-btn" onclick="removeAttachment('${att.id}', '${pillId}')">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          `;
          preview.appendChild(pill);
        });
      }
    }
  } catch (e) {
    toast('Failed to load conversation: ' + e.message, 'error');
  }
};

window.deleteConversation = async function(id, e) {
  if (e) e.stopPropagation();
  if (!confirm('Delete this conversation?')) return;
  await fetch(`/api/conversations/${id}`, {method:'DELETE'});
  State.conversations = State.conversations.filter(c => c.id !== id);
  if (State.activeConvId === id) {
    State.activeConvId = null;
    State.messages = [];
    document.getElementById('messages').style.display = 'none';
    document.getElementById('welcome').style.display = 'flex';
    updateToolbarTitle();

    // Clear ghost attachments
    const preview = document.getElementById('attachment-preview');
    if (preview) {
      preview.innerHTML = '';
      preview.style.display = 'none';
    }
  }
  renderConversationList();
  toast('Conversation deleted');
};

window.startNewChat = function() {
  if (State.streaming) { toast('Please wait for streaming to finish', ''); return; }
  State.activeConvId = null;
  State.messages = [];
  document.getElementById('messages').innerHTML = '';
  document.getElementById('messages').style.display = 'none';
  document.getElementById('welcome').style.display = 'flex';
  renderConversationList();
  updateToolbarTitle();
  
  // Clear ghost attachments
  const preview = document.getElementById('attachment-preview');
  if (preview) {
    preview.innerHTML = '';
    preview.style.display = 'none';
  }

  document.getElementById('msg-input').focus();
};

// ── Key management ─────────────────────────────────────────────
window.loadKeys = async function() {
  try {
    const res = await fetch('/api/keys');
    State.keys = await res.json() || [];
    renderKeysList();
    populateKeySelector();
  } catch {}
};

window.deleteKey = async function(id) {
  if (!confirm('Delete this API key?')) return;
  await fetch(`/api/keys/${id}`, {method:'DELETE'});
  await loadKeys();
  toast('Key deleted');
};

// ── UI helpers ─────────────────────────────────────────────────
window.toggleSendStop = function(streaming) {
  document.getElementById('btn-send').style.display = streaming ? 'none' : '';
  document.getElementById('btn-stop').style.display = streaming ? '' : 'none';
  document.getElementById('sel-provider').disabled = streaming;
  document.getElementById('sel-model').disabled = streaming;
  document.getElementById('sel-key').disabled = streaming;
};

let ghostTextarea = null;
window.autoResize = function(el) {
  if (!ghostTextarea) {
    ghostTextarea = document.createElement('div');
    ghostTextarea.style.cssText = 'position:absolute; visibility:hidden; white-space:pre-wrap; word-wrap:break-word; overflow-wrap:break-word;';
    document.body.appendChild(ghostTextarea);
  }
  const comp = window.getComputedStyle(el);
  ghostTextarea.style.width = comp.width;
  ghostTextarea.style.font = comp.font;
  ghostTextarea.style.padding = comp.padding;
  ghostTextarea.style.lineHeight = comp.lineHeight;
  ghostTextarea.style.boxSizing = comp.boxSizing;
  
  ghostTextarea.textContent = el.value + '\n';
  el.style.height = Math.min(ghostTextarea.scrollHeight, 200) + 'px';
};

window.updateCharCount = function() {
  const input = document.getElementById('msg-input');
  document.getElementById('char-count').textContent = `${input.value.length} / 32000`;
};

// ── Export ─────────────────────────────────────────────────────
window.exportConversation = function() {
  const c = State.getActiveConv();
  if (!c) { toast('Start a conversation first to export', 'error'); return; }
  let md = `# ${c.title}\n\n_Provider: ${c.provider} · Model: ${c.model}_\n\n---\n\n`;
  State.messages.forEach(m => {
    md += `**${m.role === 'user' ? 'You' : 'Assistant'}:**\n\n${m.content}\n\n---\n\n`;
  });
  const blob = new Blob([md], {type:'text/markdown'});
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = c.title.replace(/[^a-z0-9]/gi,'_') + '.md';
  a.click();
};

// ── File Uploads ───────────────────────────────────────────────
window.handleFileUpload = async function(files) {
  if (!State.activeConvId) {
    const conv = await createNewConversation();
    if (!conv) {
      toast('Failed to create conversation for upload', 'error');
      return;
    }
  }

  const preview = document.getElementById('attachment-preview');
  preview.style.display = 'flex';

  for (const file of files) {
    const pillId = 'pill-' + Date.now() + Math.floor(Math.random()*1000);
    const pill = document.createElement('div');
    pill.className = 'attachment-pill';
    pill.id = pillId;
    pill.innerHTML = `<span>⏳ ${file.name}</span>`;
    preview.appendChild(pill);

    const formData = new FormData();
    formData.append('file', file);
    formData.append('conversationId', State.activeConvId);

    try {
      const res = await fetch('/api/upload', {
        method: 'POST',
        body: formData
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      
      pill.innerHTML = `
        <span>📎 ${data.filename}</span>
        <button class="remove-btn" onclick="removeAttachment('${data.id}', '${pillId}')">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      `;
    } catch (err) {
      pill.innerHTML = `<span style="color:red">❌ ${file.name} (failed)</span>`;
      setTimeout(() => pill.remove(), 3000);
      toast('Upload failed: ' + err.message, 'error');
    }
  }
  document.getElementById('file-upload').value = '';
};

window.removeAttachment = async function(id, pillId) {
  try {
    await fetch('/api/attachments/' + id, { method: 'DELETE' });
    document.getElementById(pillId).remove();
    const preview = document.getElementById('attachment-preview');
    if (preview.children.length === 0) {
      preview.style.display = 'none';
    }
  } catch (err) {
    toast('Failed to remove attachment', 'error');
  }
};
