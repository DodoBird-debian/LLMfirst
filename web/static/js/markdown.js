// ── markdown.js — lightweight MD → HTML renderer ──────────────
window.renderMarkdown = function(text) {
  if (!text) return '';

  let html = text
    // Escape HTML entities first
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

  // Fenced code blocks ```lang\n...\n```
  html = html.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) => {
    const id = 'code-' + Math.random().toString(36).slice(2,8);
    return `<pre id="${id}" style="position:relative"><button class="code-copy-btn" onclick="copyCode('${id}')">Copy</button><code class="lang-${lang}">${code.trim()}</code></pre>`;
  });

  // Inline code
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

  // Headers
  html = html.replace(/^#### (.+)$/gm, '<h4>$1</h4>');
  html = html.replace(/^### (.+)$/gm,  '<h3>$1</h3>');
  html = html.replace(/^## (.+)$/gm,   '<h2>$1</h2>');
  html = html.replace(/^# (.+)$/gm,    '<h1>$1</h1>');

  // Blockquote
  html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>');

  // Bold & italic
  html = html.replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>');
  html = html.replace(/\*\*(.+?)\*\*/g,    '<strong>$1</strong>');
  html = html.replace(/\*(.+?)\*/g,        '<em>$1</em>');
  html = html.replace(/_(.+?)_/g,          '<em>$1</em>');

  // HR
  html = html.replace(/^---+$/gm, '<hr/>');

  // Unordered lists
  html = html.replace(/((?:^[-*+] .+\n?)+)/gm, match => {
    const items = match.trim().split('\n')
      .map(l => `<li>${l.replace(/^[-*+] /, '')}</li>`)
      .join('');
    return `<ul>${items}</ul>`;
  });

  // Ordered lists
  html = html.replace(/((?:^\d+\. .+\n?)+)/gm, match => {
    const items = match.trim().split('\n')
      .map(l => `<li>${l.replace(/^\d+\. /, '')}</li>`)
      .join('');
    return `<ol>${items}</ol>`;
  });

  // Links
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');

  // Paragraphs — wrap non-tag, non-empty lines
  html = html.replace(/^(?!<[a-z]|$)(.+)$/gm, '<p>$1</p>');

  // Double newlines between block elements
  html = html.replace(/\n{2,}/g, '\n');

  return html;
};

window.copyCode = function(preId) {
  const pre = document.getElementById(preId);
  if (!pre) return;
  const code = pre.querySelector('code');
  navigator.clipboard.writeText(code ? code.textContent : '').then(() => {
    const btn = pre.querySelector('.code-copy-btn');
    if (btn) { btn.textContent = 'Copied!'; setTimeout(() => btn.textContent = 'Copy', 1500); }
  });
};
