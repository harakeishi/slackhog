/* SlackHog UI — vanilla JS */

const ALL_CHANNELS = '__all__';

let messages = [];           // all messages cached
let currentChannel = ALL_CHANNELS;
let ws = null;
let wsReconnectTimer = null;

// ── DOM refs ──────────────────────────────────────────────────────────────────

const channelList    = document.getElementById('channel-list');
const messageList    = document.getElementById('message-list');
const channelHeader  = document.getElementById('channel-header-name');
const clearBtn       = document.getElementById('clear-btn');

// WS status badge (injected into body)
const wsStatus = document.createElement('div');
wsStatus.id = 'ws-status';
wsStatus.innerHTML = '<span class="dot"></span><span class="label">Connected</span>';
document.body.appendChild(wsStatus);

// ── Init ──────────────────────────────────────────────────────────────────────

(function init() {
  fetchAllMessages();
  connectWebSocket();
  clearBtn.addEventListener('click', handleClearAll);
})();

// ── Data fetching ─────────────────────────────────────────────────────────────

async function fetchAllMessages() {
  try {
    const res = await fetch('/_api/messages');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    messages = await res.json() || [];
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
    const data = await res.json() || [];
    if (channel === ALL_CHANNELS) {
      messages = data;
    }
    return data;
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
    currentChannel = ALL_CHANNELS;
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
  // Avoid duplicates by checking channel+ts
  const exists = messages.some(m => m.channel === msg.channel && m.ts === msg.ts);
  if (!exists) {
    messages.push(msg);
  }

  renderSidebar();

  // Only append to view if it matches the current filter
  if (currentChannel === ALL_CHANNELS || msg.channel === currentChannel) {
    appendMessageToList(msg, true);
  }
}

// ── Sidebar rendering ─────────────────────────────────────────────────────────

function getChannelCounts() {
  const counts = {};
  for (const m of messages) {
    const ch = m.channel || 'unknown';
    counts[ch] = (counts[ch] || 0) + 1;
  }
  return counts;
}

function renderSidebar() {
  const counts = getChannelCounts();
  const channels = Object.keys(counts).sort();
  const totalCount = messages.length;

  channelList.innerHTML = '';

  // "All Channels" item
  const allItem = createChannelItem(ALL_CHANNELS, 'All Channels', totalCount);
  channelList.appendChild(allItem);

  if (channels.length > 0) {
    const label = document.createElement('div');
    label.className = 'channel-section-label';
    label.textContent = 'Channels';
    channelList.appendChild(label);

    for (const ch of channels) {
      const item = createChannelItem(ch, ch, counts[ch]);
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

  // Update header
  if (channel === ALL_CHANNELS) {
    channelHeader.textContent = '☰ All Channels';
  } else {
    channelHeader.textContent = `# ${channel}`;
  }

  // Re-highlight sidebar items
  document.querySelectorAll('.channel-item').forEach(el => {
    el.classList.toggle('active', el.dataset.channel === channel);
  });

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

function buildMessageElement(msg) {
  const wrapper = document.createElement('div');
  wrapper.className = 'message message-group-start';

  // Avatar
  const avatar = document.createElement('div');
  avatar.className = 'message-avatar';
  if (msg.icon_emoji) {
    avatar.textContent = msg.icon_emoji;
    avatar.style.background = 'transparent';
    avatar.style.fontSize = '28px';
  } else if (msg.icon_url) {
    const img = document.createElement('img');
    img.src = msg.icon_url;
    img.alt = '';
    avatar.appendChild(img);
  } else {
    avatar.textContent = '🤖';
    avatar.style.background = 'transparent';
    avatar.style.fontSize = '28px';
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
  timestamp.textContent = formatTimestamp(msg.ts);
  timestamp.title = formatFullTimestamp(msg.ts);

  meta.appendChild(username);
  meta.appendChild(timestamp);

  // Channel tag (shown in all-channels view)
  if (currentChannel === ALL_CHANNELS && msg.channel) {
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

  wrapper.appendChild(avatar);
  wrapper.appendChild(body);
  return wrapper;
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
      const color = att.color.startsWith('#') ? att.color : `#${att.color}`;
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

    attEl.appendChild(content);
    container.appendChild(attEl);
  }

  return container;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

/**
 * Format a Slack ts (e.g. "1710000000.123456") to HH:MM
 */
function formatTimestamp(ts) {
  if (!ts) return '';
  const secs = parseFloat(ts);
  if (isNaN(secs)) return ts;
  const d = new Date(secs * 1000);
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function formatFullTimestamp(ts) {
  if (!ts) return '';
  const secs = parseFloat(ts);
  if (isNaN(secs)) return ts;
  const d = new Date(secs * 1000);
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
  out = out.replace(/```([\s\S]*?)```/g, '<pre style="background:rgba(0,0,0,.3);padding:8px;border-radius:4px;font-size:13px;overflow-x:auto;"><code>$1</code></pre>');

  // Inline code
  out = out.replace(/`([^`]+)`/g, '<code style="background:rgba(0,0,0,.3);padding:1px 5px;border-radius:3px;font-size:13px;">$1</code>');

  // Bold
  out = out.replace(/\*([^*]+)\*/g, '<strong>$1</strong>');

  // Italic
  out = out.replace(/_([^_]+)_/g, '<em>$1</em>');

  // Strike
  out = out.replace(/~([^~]+)~/g, '<del>$1</del>');

  return out;
}
