// Chisel Admin Console Client Application
(function () {
  const elements = {
    modeBadge: document.getElementById('modeBadge'),
    connBadge: document.getElementById('connBadge'),
    uptimeVal: document.getElementById('uptimeVal'),
    verVal: document.getElementById('verVal'),
    activeSessionsCount: document.getElementById('activeSessionsCount'),
    activeTunnelsCount: document.getElementById('activeTunnelsCount'),
    rateSentVal: document.getElementById('rateSentVal'),
    rateRecvVal: document.getElementById('rateRecvVal'),
    totalSentVal: document.getElementById('totalSentVal'),
    totalRecvVal: document.getElementById('totalRecvVal'),
    tunnelsBody: document.getElementById('tunnelsBody'),
    sessionsBody: document.getElementById('sessionsBody'),
    tunnelBadgeCount: document.getElementById('tunnelBadgeCount'),
    sessionBadgeCount: document.getElementById('sessionBadgeCount'),
    sessionsTabBtn: document.getElementById('sessionsTabBtn'),
    reconnectBtn: document.getElementById('reconnectBtn'),
    clearLogsBtn: document.getElementById('clearLogsBtn'),
    logsTerminal: document.getElementById('logsTerminal')
  };

  let currentMode = null;

  // Tab switching
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));

      btn.classList.add('active');
      const target = btn.getAttribute('data-target');
      const panel = document.getElementById(target);
      if (panel) panel.classList.add('active');
    });
  });

  // Clear logs button
  elements.clearLogsBtn.addEventListener('click', () => {
    elements.logsTerminal.innerHTML = '';
  });

  // Reconnect trigger button (Client mode)
  elements.reconnectBtn.addEventListener('click', async () => {
    try {
      elements.reconnectBtn.disabled = true;
      const res = await fetch('/api/v1/reconnect', { method: 'POST' });
      if (res.ok) {
        appendLog('Manual reconnect triggered by operator');
      } else {
        const txt = await res.text();
        appendLog(`Reconnect failed: ${txt || res.statusText}`);
      }
    } catch (e) {
      appendLog('Failed to trigger reconnect: ' + e);
    } finally {
      setTimeout(() => { elements.reconnectBtn.disabled = false; }, 2000);
    }
  });

  // Helpers
  function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatDuration(sec) {
    if (sec < 60) return sec + 's';
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    if (m < 60) return `${m}m ${s}s`;
    const h = Math.floor(m / 60);
    const remM = m % 60;
    return `${h}h ${remM}m`;
  }

  function appendLog(msg) {
    const entry = document.createElement('div');
    entry.className = 'log-entry';
    if (/^\[\d{2}:\d{2}:\d{2}\]/.test(msg)) {
      entry.textContent = msg;
    } else {
      const time = new Date().toTimeString().split(' ')[0];
      entry.textContent = `[${time}] ${msg}`;
    }
    elements.logsTerminal.appendChild(entry);
    elements.logsTerminal.scrollTop = elements.logsTerminal.scrollHeight;
  }

  // Fetch full system status
  async function fetchStatus() {
    try {
      const res = await fetch('/api/v1/status');
      if (!res.ok) throw new Error('HTTP ' + res.status);
      const data = await res.json();
      renderStatus(data);
    } catch (err) {
      elements.connBadge.textContent = 'OFFLINE';
      elements.connBadge.className = 'badge badge-status disconnected';
    }
  }

  function renderStatus(data) {
    // Mode adaptation
    if (currentMode !== data.mode) {
      currentMode = data.mode;
      elements.modeBadge.textContent = 'MODE: ' + data.mode.toUpperCase();
      if (data.mode === 'client') {
        elements.sessionsTabBtn.style.display = 'none';
        elements.reconnectBtn.style.display = 'inline-flex';
      } else {
        elements.sessionsTabBtn.style.display = 'inline-flex';
        elements.reconnectBtn.style.display = 'none';
      }
    }

    elements.uptimeVal.textContent = formatDuration(data.uptimeSeconds);
    elements.verVal.textContent = data.version;

    // Connection badge
    if (data.mode === 'server') {
      elements.connBadge.textContent = 'LISTENING';
      elements.connBadge.className = 'badge badge-status';
    } else if (data.client) {
      const state = (data.client.connectionState || 'unknown').toUpperCase();
      elements.connBadge.textContent = state;
      if (state === 'CONNECTED') {
        elements.connBadge.className = 'badge badge-status';
      } else if (state === 'CONNECTING') {
        elements.connBadge.className = 'badge badge-status connecting';
      } else {
        elements.connBadge.className = 'badge badge-status disconnected';
      }
    }

    // Counters
    elements.activeSessionsCount.textContent = data.activeSessions || 0;
    elements.activeTunnelsCount.textContent = data.activeTunnels || 0;
    elements.rateSentVal.textContent = formatBytes(data.rateSentSec) + '/s';
    elements.rateRecvVal.textContent = formatBytes(data.rateRecvSec) + '/s';
    elements.totalSentVal.textContent = formatBytes(data.totalBytesSent);
    elements.totalRecvVal.textContent = formatBytes(data.totalBytesRecv);

    // Tunnels Render
    renderTunnels(data);

    // Sessions Render
    renderSessions(data);
  }

  function renderTunnels(data) {
    let tunnels = [];
    if (data.mode === 'server' && data.server) {
      tunnels = data.server.tunnels || [];
    } else if (data.mode === 'client' && data.client) {
      tunnels = data.client.tunnels || [];
    }

    elements.tunnelBadgeCount.textContent = `${tunnels.length} Active`;

    if (tunnels.length === 0) {
      elements.tunnelsBody.innerHTML = '<tr><td colspan="7" class="empty-state">No active tunnels configured</td></tr>';
      return;
    }

    elements.tunnelsBody.innerHTML = tunnels.map(t => {
      let typeClass = 'type-forward';
      if (t.type === 'reverse') typeClass = 'type-reverse';
      if (t.type === 'socks') typeClass = 'type-socks';

      return `
        <tr>
          <td><span class="type-pill ${typeClass}">${t.type}</span></td>
          <td><span>${escapeHtml(t.user || '-')}</span></td>
          <td><span class="mono">${(t.protocol || 'tcp').toUpperCase()}</span></td>
          <td><strong class="mono">${escapeHtml(t.local || '-')}</strong></td>
          <td><span class="mono">${escapeHtml(t.remote || '-')}</span></td>
          <td><span class="mono">${t.activeConns || 0}</span></td>
          <td><span class="mono">${formatBytes(t.bytesSent)} / ${formatBytes(t.bytesRecv)}</span></td>
        </tr>
      `;
    }).join('');
  }

  function renderSessions(data) {
    if (data.mode !== 'server' || !data.server) return;
    const sessions = data.server.sessions || [];
    elements.sessionBadgeCount.textContent = `${sessions.length} Connected`;

    if (sessions.length === 0) {
      elements.sessionsBody.innerHTML = '<tr><td colspan="7" class="empty-state">No active client sessions</td></tr>';
      return;
    }

    elements.sessionsBody.innerHTML = sessions.map(s => {
      const dur = formatDuration(Math.floor((Date.now() - new Date(s.connectedAt).getTime()) / 1000));
      const remotesStr = (s.remotes && s.remotes.length) ? s.remotes.join(', ') : '-';
      return `
        <tr>
          <td><span class="mono">#${s.id}</span></td>
          <td><strong class="mono">${escapeHtml(s.remoteAddr)}</strong></td>
          <td><span>${escapeHtml(s.user || '-')}</span></td>
          <td><span>${dur}</span></td>
          <td><span class="mono">${escapeHtml(remotesStr)}</span></td>
          <td><span class="mono">${formatBytes(s.bytesSent)} / ${formatBytes(s.bytesRecv)}</span></td>
          <td>
            <button class="btn btn-danger btn-sm" onclick="window.disconnectSession(${s.id})">
              Disconnect
            </button>
          </td>
        </tr>
      `;
    }).join('');
  }

  window.disconnectSession = async function (id) {
    if (!confirm(`Are you sure you want to disconnect session #${id}?`)) return;
    try {
      const res = await fetch(`/api/v1/sessions/${id}`, { method: 'DELETE' });
      if (res.ok) {
        appendLog(`Disconnected session #${id}`);
        fetchStatus();
      } else {
        const txt = await res.text();
        alert(`Failed to disconnect: ${txt || res.statusText}`);
      }
    } catch (e) {
      alert('Failed to disconnect: ' + e);
    }
  };

  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  // SSE for live logs
  function initEventStream() {
    const ev = new EventSource('/api/v1/events');
    ev.onmessage = (event) => {
      appendLog(event.data);
    };
    ev.onerror = () => {
      // EventSource reconnects automatically
    };
  }

  // Initial load & Polling Loop
  fetchStatus();
  setInterval(fetchStatus, 1500);
  initEventStream();
})();
