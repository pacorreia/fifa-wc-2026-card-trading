const config = window.APP_CONFIG || {};
const state = {
  apiBaseUrl: config.API_BASE_URL || (window.location.port === '8081' ? `${window.location.protocol}//${window.location.hostname}:8080` : window.location.origin),
  wsBaseUrl: config.WS_BASE_URL || (window.location.port === '8081' ? `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.hostname}:8080` : `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}`),
  accessToken: localStorage.getItem('accessToken') || '',
  refreshToken: localStorage.getItem('refreshToken') || '',
  user: JSON.parse(localStorage.getItem('user') || 'null'),
  collection: [],
  missing: [],
  duplicates: [],
  matches: [],
  trades: [],
  notifications: [],
  ws: null,
  refreshInFlight: null,
};

const els = {
  authSection: document.getElementById('authSection'),
  appSection: document.getElementById('appSection'),
  currentUser: document.getElementById('currentUser'),
  logoutBtn: document.getElementById('logoutBtn'),
  statusMessage: document.getElementById('statusMessage'),
  loginForm: document.getElementById('loginForm'),
  registerForm: document.getElementById('registerForm'),
  statsCards: document.getElementById('statsCards'),
  collectionTable: document.getElementById('collectionTable'),
  collectionFilter: document.getElementById('collectionFilter'),
  missingList: document.getElementById('missingList'),
  duplicatesList: document.getElementById('duplicatesList'),
  matchesList: document.getElementById('matchesList'),
  tradesList: document.getElementById('tradesList'),
  notificationsList: document.getElementById('notificationsList'),
  notificationBadge: document.getElementById('notificationBadge'),
  reloadBtn: document.getElementById('reloadBtn'),
};

function setTokens(tokens) {
  state.accessToken = tokens?.access_token || '';
  state.refreshToken = tokens?.refresh_token || state.refreshToken;
  localStorage.setItem('accessToken', state.accessToken);
  localStorage.setItem('refreshToken', state.refreshToken);
}

function setUser(user) {
  state.user = user;
  if (user) {
    localStorage.setItem('user', JSON.stringify(user));
  } else {
    localStorage.removeItem('user');
  }
  renderAuthState();
}

function renderAuthState() {
  const isAuthed = Boolean(state.accessToken && state.user);
  els.authSection.classList.toggle('hidden', isAuthed);
  els.appSection.classList.toggle('hidden', !isAuthed);
  els.logoutBtn.classList.toggle('hidden', !isAuthed);
  els.currentUser.textContent = isAuthed ? `Signed in as ${state.user.username}` : '';
}

function showStatus(message, isError = false) {
  els.statusMessage.textContent = message;
  els.statusMessage.classList.remove('hidden', 'error');
  if (isError) els.statusMessage.classList.add('error');
  window.clearTimeout(showStatus.timer);
  showStatus.timer = window.setTimeout(() => els.statusMessage.classList.add('hidden'), 4000);
}

async function apiFetch(path, options = {}, retry = true) {
  const headers = new Headers(options.headers || {});
  headers.set('Content-Type', 'application/json');
  if (state.accessToken) headers.set('Authorization', 'Bearer ' + state.accessToken);
  const response = await fetch(`${state.apiBaseUrl}${path}`, { ...options, headers });
  if (response.status === 401 && retry && state.refreshToken) {
    await refreshTokens();
    return apiFetch(path, options, false);
  }
  const text = await response.text();
  const data = text ? JSON.parse(text) : null;
  if (!response.ok) throw new Error(data?.error || `Request failed: ${response.status}`);
  return data;
}

async function refreshTokens() {
  if (state.refreshInFlight) return state.refreshInFlight;
  state.refreshInFlight = fetch(`${state.apiBaseUrl}/api/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: state.refreshToken }),
  }).then(async (response) => {
    const data = await response.json();
    if (!response.ok) throw new Error(data?.error || 'Session expired');
    setTokens(data.tokens);
    return data.tokens;
  }).catch((error) => {
    logout(false);
    throw error;
  }).finally(() => {
    state.refreshInFlight = null;
  });
  return state.refreshInFlight;
}

async function login(username, password) {
  const data = await apiFetch('/api/auth/login', {
    method: 'POST',
    headers: {},
    body: JSON.stringify({ username, password }),
  }, false);
  setUser(data.user);
  setTokens(data.tokens);
  connectWebSocket();
  await loadDashboard();
  showStatus('Logged in successfully');
}

async function register(payload) {
  const data = await apiFetch('/api/auth/register', {
    method: 'POST',
    headers: {},
    body: JSON.stringify(payload),
  }, false);
  setUser(data.user);
  setTokens(data.tokens);
  connectWebSocket();
  await loadDashboard();
  showStatus('Account created');
}

async function logout(callApi = true) {
  try {
    if (callApi && state.refreshToken) {
      await fetch(`${state.apiBaseUrl}/api/auth/logout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: state.refreshToken }),
      });
    }
  } catch (_) {}
  if (state.ws) state.ws.close();
  state.ws = null;
  state.accessToken = '';
  state.refreshToken = '';
  state.collection = [];
  state.missing = [];
  state.duplicates = [];
  state.matches = [];
  state.trades = [];
  state.notifications = [];
  localStorage.removeItem('accessToken');
  localStorage.removeItem('refreshToken');
  setUser(null);
  renderAll();
}

async function loadDashboard() {
  const [collection, stats, missing, duplicates, matches, trades, notifications] = await Promise.all([
    apiFetch('/api/collections/me'),
    apiFetch('/api/collections/stats'),
    apiFetch('/api/collections/missing'),
    apiFetch('/api/collections/duplicates'),
    apiFetch('/api/trades/matches'),
    apiFetch('/api/trades'),
    apiFetch('/api/notifications'),
  ]);
  state.collection = collection.items || [];
  state.stats = stats;
  state.missing = missing.stickers || [];
  state.duplicates = duplicates.items || [];
  state.matches = matches.matches || [];
  state.trades = trades.trades || [];
  state.notifications = notifications.notifications || [];
  renderAll();
}

function renderStats() {
  const stats = state.stats || {};
  const cards = [
    ['Owned', `${stats.owned || 0}/${stats.total || 0}`],
    ['Completion', `${(stats.overall_percent || 0).toFixed(1)}%`],
    ['Missing', `${stats.missing || 0}`],
    ['Duplicates', `${stats.duplicates || 0}`],
    ['Specials', `${stats.specials_owned || 0}/${stats.specials_total || 0}`],
    ['Special %', `${(stats.specials_percent || 0).toFixed(1)}%`],
  ];
  const teams = (stats.team_stats || []).slice(0, 8).map((team) => `<li>${team.team}: ${team.owned}/${team.total} (${team.percent.toFixed(0)}%)</li>`).join('');
  els.statsCards.innerHTML = cards.map(([label, value]) => `<div class="stat-card"><span>${label}</span><strong>${value}</strong></div>`).join('') + `<div class="stat-card wide"><span>Top Team Progress</span><ul>${teams}</ul></div>`;
}

function renderCollection() {
  const filter = els.collectionFilter.value.trim().toLowerCase();
  const filtered = state.collection.filter((item) => {
    if (!filter) return true;
    return item.sticker.code.toLowerCase().includes(filter) || item.sticker.team.toLowerCase().includes(filter);
  });
  els.collectionTable.innerHTML = filtered.map((item) => `
    <tr>
      <td>${item.sticker.code}</td>
      <td>${item.sticker.team}</td>
      <td>${item.sticker.category}</td>
      <td>${item.sticker.position}</td>
      <td><input class="count-input" type="number" min="0" max="99" value="${item.have_count}" data-code="${item.sticker.code}" /></td>
    </tr>
  `).join('');
}

function stickerCard(sticker, extra = '') {
  return `<article class="card"><h3>${sticker.code}</h3><p>${sticker.team} · ${sticker.category}</p><p>${sticker.position}</p>${extra}</article>`;
}

function renderMissing() {
  els.missingList.innerHTML = state.missing.map((sticker) => stickerCard(sticker)).join('') || '<p>No missing stickers. Nice!</p>';
}

function renderDuplicates() {
  els.duplicatesList.innerHTML = state.duplicates.map((item) => stickerCard(item.sticker, `<p>Count: ${item.have_count}</p>`)).join('') || '<p>No duplicates yet.</p>';
}

function renderMatches() {
  els.matchesList.innerHTML = state.matches.map((match) => {
    const offered = match.you_have_they_need.map((s) => `<label><input type="checkbox" class="trade-offer" value="${s.code}" /> ${s.code}</label>`).join('');
    const requested = match.they_have_you_need.map((s) => `<label><input type="checkbox" class="trade-request" value="${s.code}" /> ${s.code}</label>`).join('');
    return `
      <article class="card match-card" data-user-id="${match.user_id}">
        <h3>${match.username}</h3>
        <p>Match score: ${match.match_score}</p>
        <div class="trade-columns">
          <div><strong>You can offer</strong><div class="checklist">${offered || '<p>Nothing yet</p>'}</div></div>
          <div><strong>You can request</strong><div class="checklist">${requested || '<p>Nothing yet</p>'}</div></div>
        </div>
        <button class="create-trade-btn">Create trade proposal</button>
      </article>`;
  }).join('') || '<p>No matches yet. Keep updating your collection.</p>';
}

function renderTrades() {
  els.tradesList.innerHTML = state.trades.map((trade) => {
    const isResponder = trade.responder_id === state.user.id;
    const items = trade.items.map((item) => `<li>${item.direction}: ${item.sticker_code}</li>`).join('');
    const actions = isResponder && trade.status === 'pending'
      ? `<div class="inline-actions"><button data-trade-id="${trade.id}" data-response="accepted">Accept</button><button class="secondary" data-trade-id="${trade.id}" data-response="rejected">Reject</button></div>`
      : '';
    return `<article class="card"><h3>Trade #${trade.id}</h3><p>${trade.proposer_username} → ${trade.responder_username}</p><p>Status: ${trade.status}</p><ul>${items}</ul>${actions}</article>`;
  }).join('') || '<p>No trades yet.</p>';
}

function renderNotifications() {
  const unread = state.notifications.filter((item) => !item.is_read).length;
  els.notificationBadge.textContent = unread;
  els.notificationsList.innerHTML = state.notifications.map((notification) => `
    <article class="card ${notification.is_read ? '' : 'unread'}">
      <h3>${notification.type}</h3>
      <p>${notification.message}</p>
      <small>${new Date(notification.created_at).toLocaleString()}</small>
      ${notification.is_read ? '' : `<button data-notification-id="${notification.id}" class="secondary">Mark read</button>`}
    </article>`).join('') || '<p>No notifications.</p>';
}

function renderAll() {
  renderAuthState();
  renderStats();
  renderCollection();
  renderMissing();
  renderDuplicates();
  renderMatches();
  renderTrades();
  renderNotifications();
}

function connectWebSocket() {
  if (!state.accessToken) return;
  if (state.ws) state.ws.close();
  state.ws = new WebSocket(`${state.wsBaseUrl}/ws?token=${encodeURIComponent(state.accessToken)}`);
  state.ws.onmessage = async (event) => {
    const message = JSON.parse(event.data);
    if (message.type === 'notification.unread.count') {
      const payload = JSON.parse(message.payload || '{}');
      els.notificationBadge.textContent = payload.count || 0;
      return;
    }
    if (['collection.updated', 'stats.updated', 'trade.request.received', 'trade.request.accepted', 'trade.request.rejected'].includes(message.type)) {
      await loadDashboard();
      showStatus(`Real-time update: ${message.type}`);
    }
  };
  state.ws.onclose = () => {
    if (state.accessToken) {
      window.setTimeout(connectWebSocket, 2000);
    }
  };
}

async function updateCollection(code, haveCount) {
  await apiFetch(`/api/collections/stickers/${encodeURIComponent(code)}`, {
    method: 'PUT',
    body: JSON.stringify({ have_count: Number(haveCount) }),
  });
  await loadDashboard();
  showStatus(`Updated ${code}`);
}

async function createTrade(card) {
  const responderId = Number(card.dataset.userId);
  const offeredCodes = Array.from(card.querySelectorAll('.trade-offer:checked')).map((el) => el.value);
  const requestedCodes = Array.from(card.querySelectorAll('.trade-request:checked')).map((el) => el.value);
  if (!offeredCodes.length || !requestedCodes.length) {
    throw new Error('Select at least one offered and one requested sticker');
  }
  await apiFetch('/api/trades', {
    method: 'POST',
    body: JSON.stringify({ responder_id: responderId, offered_codes: offeredCodes, requested_codes: requestedCodes }),
  });
  await loadDashboard();
  showStatus('Trade proposal sent');
}

async function respondTrade(tradeId, response) {
  await apiFetch(`/api/trades/${tradeId}/respond`, {
    method: 'PUT',
    body: JSON.stringify({ response }),
  });
  await loadDashboard();
  showStatus(`Trade ${response}`);
}

async function markNotificationRead(notificationId) {
  await apiFetch(`/api/notifications/${notificationId}/read`, { method: 'PUT' });
  await loadDashboard();
}

document.querySelectorAll('.tab').forEach((tabButton) => {
  tabButton.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach((tab) => tab.classList.remove('active'));
    document.querySelectorAll('.tab-panel').forEach((panel) => panel.classList.remove('active'));
    tabButton.classList.add('active');
    document.getElementById(`tab-${tabButton.dataset.tab}`).classList.add('active');
  });
});

els.loginForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const form = new FormData(event.target);
  try {
    await login(form.get('username'), form.get('password'));
  } catch (error) {
    showStatus(error.message, true);
  }
});

els.registerForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const form = new FormData(event.target);
  try {
    await register({ username: form.get('username'), email: form.get('email'), password: form.get('password') });
  } catch (error) {
    showStatus(error.message, true);
  }
});

els.logoutBtn.addEventListener('click', () => logout());
els.reloadBtn.addEventListener('click', () => loadDashboard().catch((error) => showStatus(error.message, true)));
els.collectionFilter.addEventListener('input', renderCollection);

els.collectionTable.addEventListener('change', async (event) => {
  if (!event.target.matches('.count-input')) return;
  try {
    await updateCollection(event.target.dataset.code, event.target.value);
  } catch (error) {
    showStatus(error.message, true);
  }
});

els.matchesList.addEventListener('click', async (event) => {
  if (!event.target.matches('.create-trade-btn')) return;
  try {
    await createTrade(event.target.closest('.match-card'));
  } catch (error) {
    showStatus(error.message, true);
  }
});

els.tradesList.addEventListener('click', async (event) => {
  if (!event.target.dataset.tradeId) return;
  try {
    await respondTrade(event.target.dataset.tradeId, event.target.dataset.response);
  } catch (error) {
    showStatus(error.message, true);
  }
});

els.notificationsList.addEventListener('click', async (event) => {
  if (!event.target.dataset.notificationId) return;
  try {
    await markNotificationRead(event.target.dataset.notificationId);
  } catch (error) {
    showStatus(error.message, true);
  }
});

if (state.accessToken && state.user) {
  renderAuthState();
  connectWebSocket();
  loadDashboard().catch((error) => showStatus(error.message, true));
} else {
  renderAll();
}
