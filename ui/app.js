/* SlackHog UI — vanilla JS */

const ALL_CHANNELS = '__all__';

let messages = [];           // all messages cached
let currentChannel = ALL_CHANNELS;
let ws = null;
let wsReconnectTimer = null;
let readCounts = {};         // channel -> number of messages already "seen"
let currentThreadID = null;  // currently open thread parent ID

// ── DOM refs ──────────────────────────────────────────────────────────────────

const channelList    = document.getElementById('channel-list');
const messageList    = document.getElementById('message-list');
const channelHeader  = document.getElementById('channel-header-name');
const clearBtn       = document.getElementById('clear-btn');
const themeToggle    = document.getElementById('theme-toggle');
const threadPanel    = document.getElementById('thread-panel');
const threadParent   = document.getElementById('thread-parent');
const threadReplies  = document.getElementById('thread-replies');
const threadCloseBtn = document.getElementById('thread-close-btn');

// WS status badge (injected into body)
const wsStatus = document.createElement('div');
wsStatus.id = 'ws-status';
wsStatus.innerHTML = '<span class="dot"></span><span class="label">Connected</span>';
document.body.appendChild(wsStatus);

// ── Init ──────────────────────────────────────────────────────────────────────

(function init() {
  initTheme();
  fetchAllMessages();
  connectWebSocket();
  clearBtn.addEventListener('click', handleClearAll);
  themeToggle.addEventListener('click', toggleTheme);
  threadCloseBtn.addEventListener('click', closeThread);
})();

// ── Theme ──────────────────────────────────────────────────────────────────────

function initTheme() {
  const saved = localStorage.getItem('slackhog-theme');
  if (saved) {
    document.body.setAttribute('data-theme', saved);
  }
  updateThemeIcon();
}

function toggleTheme() {
  const current = document.body.getAttribute('data-theme');
  const next = current === 'dark' ? 'light' : 'dark';
  document.body.setAttribute('data-theme', next);
  localStorage.setItem('slackhog-theme', next);
  updateThemeIcon();
}

function updateThemeIcon() {
  const theme = document.body.getAttribute('data-theme');
  themeToggle.textContent = theme === 'dark' ? '☀️' : '🌙';
  themeToggle.title = theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode';
}

// ── Data fetching ─────────────────────────────────────────────────────────────

async function fetchAllMessages() {
  try {
    const res = await fetch('/_api/messages');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    messages = data.messages || [];
    markCurrentChannelRead();
    renderSidebar();
    renderMessages();
  } catch (err) {
    console.error('Failed to fetch messages:', err);
  }
}

async function fetchChannelMessages(channel) {
  try {
    const url = channel === ALL_CHANNELS
      ? '/_api/messages'
      : `/_api/messages?channel=${encodeURIComponent(channel)}`;
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    const msgs = data.messages || [];
    if (channel === ALL_CHANNELS) {
      messages = msgs;
    }
    return msgs;
  } catch (err) {
    console.error('Failed to fetch channel messages:', err);
    return [];
  }
}

async function handleClearAll() {
  if (!confirm('Clear all messages?')) return;
  try {
    const res = await fetch('/_api/messages', { method: 'DELETE' });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    messages = [];
    readCounts = {};
    currentChannel = ALL_CHANNELS;
    closeThread();
    renderSidebar();
    renderMessages();
  } catch (err) {
    console.error('Failed to clear messages:', err);
  }
}

// ── WebSocket ─────────────────────────────────────────────────────────────────

function connectWebSocket() {
  if (wsReconnectTimer) {
    clearTimeout(wsReconnectTimer);
    wsReconnectTimer = null;
  }

  ws = new WebSocket(`ws://${location.host}/ws`);

  ws.addEventListener('open', () => {
    setWsStatus(true);
  });

  ws.addEventListener('message', (event) => {
    let msg;
    try {
      msg = JSON.parse(event.data);
    } catch {
      console.warn('Bad WS message:', event.data);
      return;
    }
    handleIncomingMessage(msg);
  });

  ws.addEventListener('close', () => {
    setWsStatus(false);
    scheduleReconnect();
  });

  ws.addEventListener('error', () => {
    setWsStatus(false);
  });
}

function scheduleReconnect() {
  if (wsReconnectTimer) return;
  wsReconnectTimer = setTimeout(() => {
    wsReconnectTimer = null;
    connectWebSocket();
  }, 3000);
}

function setWsStatus(connected) {
  const label = wsStatus.querySelector('.label');
  if (connected) {
    wsStatus.classList.remove('disconnected');
    label.textContent = 'Connected';
  } else {
    wsStatus.classList.add('disconnected');
    label.textContent = 'Reconnecting…';
  }
}

function handleIncomingMessage(msg) {
  // Avoid duplicates by checking id
  const exists = messages.some(m => m.id === msg.id);
  if (!exists) {
    messages.push(msg);
  }

  // If this is a new reply, update the parent's reply_count in the local cache
  if (!exists && msg.thread_ts) {
    const parent = messages.find(m => m.id === msg.thread_ts);
    if (parent) {
      parent.reply_count = (parent.reply_count || 0) + 1;
    }
  }

  // If we're viewing this channel (or all), mark as read and re-render
  if (currentChannel === ALL_CHANNELS || msg.channel === currentChannel) {
    markCurrentChannelRead();
    if (msg.thread_ts) {
      // Reply: re-render the message list to refresh reply counts
      renderMessages();
    } else {
      appendMessageToList(msg, true);
    }
  }

  // If the thread panel is open for this thread, append the reply there too
  if (msg.thread_ts && msg.thread_ts === currentThreadID) {
    const el = buildMessageElement(msg, true);
    threadReplies.appendChild(el);
    threadReplies.scrollTop = threadReplies.scrollHeight;
  }

  renderSidebar();
}

// ── Sidebar rendering ─────────────────────────────────────────────────────────

// Mark current channel (or all channels if ALL_CHANNELS) as fully read.
function markCurrentChannelRead() {
  if (currentChannel === ALL_CHANNELS) {
    // Mark every channel as read
    const counts = getTotalChannelCounts();
    for (const ch in counts) {
      readCounts[ch] = counts[ch];
    }
  } else {
    readCounts[currentChannel] = getTotalChannelCounts()[currentChannel] || 0;
  }
}

// Total messages per channel (not unread).
function getTotalChannelCounts() {
  const counts = {};
  for (const m of messages) {
    const ch = m.channel || 'unknown';
    counts[ch] = (counts[ch] || 0) + 1;
  }
  return counts;
}

// Unread messages per channel.
function getUnreadCounts() {
  const total = getTotalChannelCounts();
  const unread = {};
  for (const ch in total) {
    const diff = total[ch] - (readCounts[ch] || 0);
    if (diff > 0) unread[ch] = diff;
  }
  return unread;
}

function renderSidebar() {
  const totalCounts = getTotalChannelCounts();
  const unreadCounts = getUnreadCounts();
  const channels = Object.keys(totalCounts).sort();
  const totalUnread = Object.values(unreadCounts).reduce((a, b) => a + b, 0);

  channelList.innerHTML = '';

  // "All Channels" item
  const allItem = createChannelItem(ALL_CHANNELS, 'All Channels', totalUnread);
  channelList.appendChild(allItem);

  if (channels.length > 0) {
    const label = document.createElement('div');
    label.className = 'channel-section-label';
    label.textContent = 'Channels';
    channelList.appendChild(label);

    for (const ch of channels) {
      const item = createChannelItem(ch, ch, unreadCounts[ch] || 0);
      channelList.appendChild(item);
    }
  }
}

function createChannelItem(value, label, count) {
  const item = document.createElement('div');
  item.className = 'channel-item' + (value === currentChannel ? ' active' : '');
  item.dataset.channel = value;

  const hash = document.createElement('span');
  hash.className = 'channel-hash';
  hash.textContent = value === ALL_CHANNELS ? '☰' : '#';

  const name = document.createElement('span');
  name.className = 'channel-name';
  name.textContent = label;

  item.appendChild(hash);
  item.appendChild(name);

  if (count > 0) {
    const badge = document.createElement('span');
    badge.className = 'channel-badge';
    badge.textContent = count > 99 ? '99+' : count;
    item.appendChild(badge);
  }

  item.addEventListener('click', () => handleChannelClick(value));
  return item;
}

// ── Channel selection ─────────────────────────────────────────────────────────

async function handleChannelClick(channel) {
  if (currentChannel === channel) return;
  currentChannel = channel;
  markCurrentChannelRead();
  renderSidebar();

  // Update header
  if (channel === ALL_CHANNELS) {
    channelHeader.textContent = '☰ All Channels';
  } else {
    channelHeader.textContent = `# ${channel}`;
  }

  // Fetch and render
  if (channel === ALL_CHANNELS) {
    renderMessages();
  } else {
    const filtered = await fetchChannelMessages(channel);
    renderMessages(filtered);
  }
}

// ── Message list rendering ────────────────────────────────────────────────────

function renderMessages(msgs) {
  messageList.innerHTML = '';

  const list = msgs !== undefined ? msgs : (
    currentChannel === ALL_CHANNELS
      ? messages
      : messages.filter(m => m.channel === currentChannel)
  );

  if (!list || list.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty-state';
    empty.innerHTML = '<span class="empty-icon">🐗</span><span class="empty-text">No messages yet. Waiting for Slack events…</span>';
    messageList.appendChild(empty);
    return;
  }

  for (const msg of list) {
    const el = buildMessageElement(msg);
    messageList.appendChild(el);
  }

  scrollToBottom();
}

function appendMessageToList(msg, scroll) {
  // Remove empty state if present
  const empty = messageList.querySelector('.empty-state');
  if (empty) empty.remove();

  const el = buildMessageElement(msg);
  el.classList.add('message-group-start');
  messageList.appendChild(el);

  if (scroll) scrollToBottom();
}

function scrollToBottom() {
  messageList.scrollTop = messageList.scrollHeight;
}

// ── Message element builder ───────────────────────────────────────────────────

function buildMessageElement(msg, inThread) {
  const wrapper = document.createElement('div');
  wrapper.className = 'message message-group-start';

  // Avatar
  const avatar = document.createElement('div');
  avatar.className = 'message-avatar';
  const resolvedEmoji = msg.icon_emoji ? resolveEmoji(msg.icon_emoji) : null;
  if (resolvedEmoji) {
    avatar.textContent = resolvedEmoji;
    avatar.style.background = 'transparent';
    avatar.style.fontSize = '28px';
  } else if (msg.icon_url) {
    const img = document.createElement('img');
    img.src = msg.icon_url;
    img.alt = '';
    avatar.appendChild(img);
  } else {
    // Default: show initials from username
    const name = msg.username || 'bot';
    avatar.textContent = name.charAt(0).toUpperCase();
    avatar.style.fontSize = '16px';
    avatar.style.fontWeight = '700';
  }

  // Body
  const body = document.createElement('div');
  body.className = 'message-body';

  // Meta row
  const meta = document.createElement('div');
  meta.className = 'message-meta';

  const username = document.createElement('span');
  username.className = 'message-username';
  username.textContent = msg.username || msg.bot_id || 'bot';

  const timestamp = document.createElement('span');
  timestamp.className = 'message-timestamp';
  timestamp.textContent = formatTimestamp(msg.received_at);
  timestamp.title = formatFullTimestamp(msg.received_at);

  meta.appendChild(username);
  meta.appendChild(timestamp);

  // Channel tag (shown in all-channels view, not in thread panel)
  if (!inThread && currentChannel === ALL_CHANNELS && msg.channel) {
    const tag = document.createElement('span');
    tag.className = 'message-channel-tag';
    tag.textContent = `#${msg.channel}`;
    meta.appendChild(tag);
  }

  body.appendChild(meta);

  // Text
  if (msg.text) {
    const text = document.createElement('div');
    text.className = 'message-text';
    text.innerHTML = formatSlackText(msg.text);
    body.appendChild(text);
  }

  // Blocks
  if (Array.isArray(msg.blocks) && msg.blocks.length > 0) {
    const blocksEl = renderBlocks(msg.blocks);
    body.appendChild(blocksEl);
  }

  // Attachments
  if (Array.isArray(msg.attachments) && msg.attachments.length > 0) {
    const attEl = renderAttachments(msg.attachments);
    body.appendChild(attEl);
  }

  // Reply badge
  if (!inThread && msg.reply_count > 0) {
    const badge = document.createElement('div');
    badge.className = 'reply-badge';
    badge.textContent = `💬 ${msg.reply_count}件の返信`;
    badge.addEventListener('click', (e) => {
      e.stopPropagation();
      openThread(msg.id);
    });
    body.appendChild(badge);
  }

  wrapper.appendChild(avatar);
  wrapper.appendChild(body);
  return wrapper;
}

// ── Thread panel ──────────────────────────────────────────────────────────────

async function openThread(parentID) {
  currentThreadID = parentID;
  threadPanel.classList.remove('hidden');

  // Find parent message from cache
  const parent = messages.find(m => m.id === parentID);
  threadParent.innerHTML = '';
  if (parent) {
    const el = buildMessageElement(parent, true);  // true = inThread (no reply badge)
    threadParent.appendChild(el);
  }

  // Fetch replies
  try {
    const res = await fetch(`/_api/messages/${encodeURIComponent(parentID)}/replies`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    const replies = data.replies || [];
    threadReplies.innerHTML = '';
    for (const reply of replies) {
      const el = buildMessageElement(reply, true);
      threadReplies.appendChild(el);
    }
    threadReplies.scrollTop = threadReplies.scrollHeight;
  } catch (err) {
    console.error('Failed to fetch replies:', err);
  }
}

function closeThread() {
  currentThreadID = null;
  threadPanel.classList.add('hidden');
  threadParent.innerHTML = '';
  threadReplies.innerHTML = '';
}

// ── Blocks renderer ───────────────────────────────────────────────────────────

function renderBlocks(blocks) {
  const container = document.createElement('div');
  container.className = 'blocks';

  for (const block of blocks) {
    switch (block.type) {
      case 'section': {
        const el = document.createElement('div');
        el.className = 'block-section';
        const textObj = block.text;
        if (textObj) {
          el.innerHTML = formatSlackText(textObj.text || '');
        }
        container.appendChild(el);
        break;
      }
      case 'divider': {
        const hr = document.createElement('hr');
        hr.className = 'block-divider';
        container.appendChild(hr);
        break;
      }
      // Other block types: render nothing (future extension point)
      default:
        break;
    }
  }

  return container;
}

// ── Attachments renderer ──────────────────────────────────────────────────────

function renderAttachments(attachments) {
  const container = document.createElement('div');
  container.className = 'attachments';

  for (const att of attachments) {
    const attEl = document.createElement('div');
    attEl.className = 'attachment';

    // Color bar
    const colorBar = document.createElement('div');
    colorBar.className = 'attachment-color-bar';
    if (att.color) {
      const slackColors = { good: '#2eb886', warning: '#daa038', danger: '#a30200' };
      const color = slackColors[att.color] || (att.color.startsWith('#') ? att.color : `#${att.color}`);
      colorBar.style.background = color;
    }
    attEl.appendChild(colorBar);

    // Content area
    const content = document.createElement('div');
    content.className = 'attachment-content';

    if (att.title) {
      const title = document.createElement('div');
      title.className = 'attachment-title';
      if (att.title_link) {
        const a = document.createElement('a');
        a.href = att.title_link;
        a.target = '_blank';
        a.rel = 'noopener noreferrer';
        a.textContent = att.title;
        title.appendChild(a);
      } else {
        title.textContent = att.title;
      }
      content.appendChild(title);
    }

    if (att.text) {
      const text = document.createElement('div');
      text.className = 'attachment-text';
      text.innerHTML = formatSlackText(att.text);
      content.appendChild(text);
    }

    if (Array.isArray(att.fields) && att.fields.length > 0) {
      const fieldsEl = document.createElement('div');
      fieldsEl.className = 'attachment-fields';

      for (const field of att.fields) {
        const fieldEl = document.createElement('div');
        fieldEl.className = field.short ? 'attachment-field-short' : 'attachment-field-long';

        const fieldTitle = document.createElement('div');
        fieldTitle.className = 'attachment-field-title';
        fieldTitle.textContent = field.title || '';

        const fieldValue = document.createElement('div');
        fieldValue.className = 'attachment-field-value';
        fieldValue.innerHTML = formatSlackText(field.value || '');

        fieldEl.appendChild(fieldTitle);
        fieldEl.appendChild(fieldValue);
        fieldsEl.appendChild(fieldEl);
      }

      content.appendChild(fieldsEl);
    }

    if (att.footer) {
      const footer = document.createElement('div');
      footer.className = 'attachment-footer';
      footer.textContent = att.footer;
      content.appendChild(footer);
    }

    attEl.appendChild(content);
    container.appendChild(attEl);
  }

  return container;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

/**
 * Resolve a Slack emoji shortcode (e.g. ":rocket:" or "rocket") to a Unicode emoji.
 * Returns the Unicode character, or null if not found.
 */
function resolveEmoji(raw) {
  if (!raw) return null;
  // Strip colons: ":rocket:" -> "rocket"
  const code = raw.replace(/^:/, '').replace(/:$/, '');
  // Check EMOJI_MAP (from emoji.js)
  if (typeof EMOJI_MAP !== 'undefined' && EMOJI_MAP[code]) {
    return EMOJI_MAP[code];
  }
  // If the raw value is already a Unicode emoji (not a shortcode), return it
  if (!raw.startsWith(':')) return raw;
  return null;
}

/**
 * Format a timestamp (ISO8601 string or Unix seconds) to HH:MM
 */
function formatTimestamp(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function formatFullTimestamp(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  return d.toLocaleString();
}

/**
 * Very basic Slack mrkdwn → HTML conversion.
 * Handles: *bold*, _italic_, `code`, ```pre```, <url|label>, plain URLs.
 */
function formatSlackText(text) {
  if (!text) return '';

  // Escape HTML first
  let out = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

  // Slack links: &lt;url|label&gt; or &lt;url&gt;
  out = out.replace(/&lt;(https?:\/\/[^|&]+)\|([^&]+)&gt;/g, '<a href="$1" target="_blank" rel="noopener noreferrer">$2</a>');
  out = out.replace(/&lt;(https?:\/\/[^&]+)&gt;/g, '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>');

  // Code block (triple backtick)
  out = out.replace(/```([\s\S]*?)```/g, '<pre style="background:var(--code-bg);padding:8px;border-radius:4px;font-size:13px;overflow-x:auto;"><code>$1</code></pre>');

  // Inline code
  out = out.replace(/`([^`]+)`/g, '<code style="background:var(--code-bg);padding:1px 5px;border-radius:3px;font-size:13px;">$1</code>');

  // Bold
  out = out.replace(/\*([^*]+)\*/g, '<strong>$1</strong>');

  // Italic
  out = out.replace(/_([^_]+)_/g, '<em>$1</em>');

  // Strike
  out = out.replace(/~([^~]+)~/g, '<del>$1</del>');

  return out;
}
