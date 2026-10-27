/**
 * RunPilot - Control Panel Frontend Engine
 * Modern, Component-based, Reactive Vanilla JavaScript
 */

(function () {
  'use strict';

  // State Management
  const state = {
    token: localStorage.getItem('runpilot_token') || '',
    user: null,
    currentPanel: 'overview',
    applications: [],
    servers: [],
    deployments: [],
    auditLogs: [],
    activeLogAppId: null,
    logRefreshInterval: null,
  };

  // DOM Elements
  const el = {
    loginScreen: document.getElementById('login-screen'),
    loginForm: document.getElementById('login-form'),
    loginEmail: document.getElementById('login-email'),
    loginPassword: document.getElementById('login-password'),
    loginError: document.getElementById('login-error'),
    appContainer: document.getElementById('app-container'),
    currentUserAvatar: document.getElementById('current-user-avatar'),
    currentUserEmail: document.getElementById('current-user-email'),
    currentUserRole: document.getElementById('current-user-role'),
    btnLogout: document.getElementById('btn-logout'),
    pageTitle: document.getElementById('page-title'),
    topbarHostName: document.getElementById('topbar-host-name'),

    // Nav Items
    navItems: document.querySelectorAll('.nav-item[data-panel]'),
    panels: document.querySelectorAll('.panel-section'),

    // Overview Metrics
    metricTotalApps: document.getElementById('metric-total-apps'),
    metricRunningApps: document.getElementById('metric-running-apps'),
    metricCpuUsage: document.getElementById('metric-cpu-usage'),
    metricRamUsage: document.getElementById('metric-ram-usage'),
    metricDiskUsage: document.getElementById('metric-disk-usage'),
    overviewAppsTableBody: document.getElementById('overview-apps-table-body'),
    overviewAuditTableBody: document.getElementById('overview-audit-table-body'),
    btnRefreshOverview: document.getElementById('btn-refresh-overview'),

    // Applications Panel
    appsFullTableBody: document.getElementById('apps-full-table-body'),
    btnCreateAppTop: document.getElementById('btn-create-app-top'),
    btnCreateAppSub: document.getElementById('btn-create-app-sub'),

    // Deployments Panel
    deploymentsTableBody: document.getElementById('deployments-table-body'),

    // Domains Panel
    domainsTableBody: document.getElementById('domains-table-body'),
    btnAttachDomainOpen: document.getElementById('btn-attach-domain-open'),

    // Logs Panel
    logAppSelect: document.getElementById('log-app-select'),
    logLinesSelect: document.getElementById('log-lines-select'),
    btnRefreshLogs: document.getElementById('btn-refresh-logs'),
    liveTerminalViewer: document.getElementById('live-terminal-viewer'),

    // Audit Panel
    fullAuditTableBody: document.getElementById('full-audit-table-body'),
    btnRefreshAudit: document.getElementById('btn-refresh-audit'),

    // Modals
    modalCreateApp: document.getElementById('modal-create-app'),
    formCreateApp: document.getElementById('form-create-app'),
    appRuntimeType: document.getElementById('app-runtime-type'),
    appRuntimeVersion: document.getElementById('app-runtime-version'),

    modalBuildLog: document.getElementById('modal-build-log'),
    buildLogTitle: document.getElementById('build-log-title'),
    buildLogContent: document.getElementById('build-log-content'),

    modalAttachDomain: document.getElementById('modal-attach-domain'),
    formAttachDomain: document.getElementById('form-attach-domain'),
    domainAppId: document.getElementById('domain-app-id'),
    domainNameInput: document.getElementById('domain-name-input'),

    modalEnvVars: document.getElementById('modal-env-vars'),
    formEnvVars: document.getElementById('form-env-vars'),
    envAppId: document.getElementById('env-app-id'),
    envVarsContainer: document.getElementById('env-vars-container'),
    btnAddEnvRow: document.getElementById('btn-add-env-row'),

    toastContainer: document.getElementById('toast-container'),
  };

  // Helper: API Fetcher
  async function api(path, options = {}) {
    const headers = options.headers || {};
    if (state.token) {
      headers['Authorization'] = `Bearer ${state.token}`;
    }
    if (options.body && typeof options.body === 'object' && !(options.body instanceof FormData)) {
      headers['Content-Type'] = 'application/json';
      options.body = JSON.stringify(options.body);
    }
    options.headers = headers;

    const res = await fetch(path, options);
    if (res.status === 401) {
      logout();
      throw new Error('Session expired. Please log in again.');
    }
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(err.error || res.statusText);
    }
    return res.json().catch(() => ({}));
  }

  // Toast System
  function showToast(message, type = 'success') {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.innerHTML = `
      <div style="font-weight: 600;">${type === 'success' ? '✓' : '⚠'}</div>
      <div>${message}</div>
    `;
    el.toastContainer.appendChild(toast);
    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transform = 'translateY(10px)';
      toast.style.transition = 'all 0.2s ease';
      setTimeout(() => toast.remove(), 250);
    }, 4000);
  }

  // Router / Panel Switcher
  function switchPanel(panelName) {
    state.currentPanel = panelName;

    el.navItems.forEach(item => {
      if (item.getAttribute('data-panel') === panelName) {
        item.classList.add('active');
      } else {
        item.classList.remove('active');
      }
    });

    el.panels.forEach(panel => {
      if (panel.id === `panel-${panelName}`) {
        panel.style.display = 'block';
      } else {
        panel.style.display = 'none';
      }
    });

    const titles = {
      overview: 'Overview Dashboard',
      applications: 'Application Lifecycle Management',
      deployments: 'Git Deployments & Releases',
      domains: 'Domains & Dynamic Host Router',
      runtimes: 'Node.js & Python Runtime Manager',
      logs: 'Process Journal Logs',
      servers: 'Managed Linux Nodes',
      packages: 'Hosting Packages & Plans',
      users: 'Users & Hosting Accounts',
      audit: 'Audit Log & Security Trail',
      settings: 'System & Account Profile Settings',
    };
    el.pageTitle.textContent = titles[panelName] || 'Control Panel';

    if (typeof closeMobileSidebar === 'function') {
      closeMobileSidebar();
    }

    // Refresh active panel data
    switch (panelName) {
      case 'overview':
        loadOverview();
        break;
      case 'applications':
        loadApplications();
        break;
      case 'deployments':
        loadDeployments();
        break;
      case 'domains':
        loadDomains();
        break;
      case 'logs':
        populateLogApps();
        break;
      case 'packages':
        loadPackages();
        break;
      case 'users':
        loadUsers();
        break;
      case 'audit':
        loadAuditLogs();
        break;
      case 'settings':
        if (typeof populateProfileInputs === 'function') {
          populateProfileInputs(state.user);
        }
        break;
    }
  }

  // Authentication
  async function initAuth() {
    // SSO Token Support: check if token was passed via URL query parameter (e.g. WHMCS 1-Click Login)
    const urlParams = new URLSearchParams(window.location.search);
    const ssoToken = urlParams.get('token') || urlParams.get('jwt');
    if (ssoToken) {
      state.token = ssoToken;
      localStorage.setItem('runpilot_token', ssoToken);
      if (window.history && window.history.replaceState) {
        window.history.replaceState({}, document.title, window.location.pathname);
      }
    }

    if (!state.token) {
      showLogin();
      return;
    }
    try {
      const claims = await api('/api/v1/auth/me');
      state.user = claims;
      el.currentUserEmail.textContent = claims.username ? `${claims.username} (${claims.email})` : claims.email;
      el.currentUserAvatar.textContent = (claims.username || claims.email || 'A').charAt(0).toUpperCase();
      el.currentUserRole.textContent = claims.role === 'admin' ? 'Administrator' : 'Client';
      if (typeof populateProfileInputs === 'function') {
        populateProfileInputs(claims);
      }
      hideLogin();
      switchPanel('overview');
    } catch (err) {
      showLogin();
    }
  }

  function showLogin() {
    el.loginScreen.classList.add('active');
    el.appContainer.style.display = 'none';
  }

  function hideLogin() {
    el.loginScreen.classList.remove('active');
    el.appContainer.style.display = 'flex';
  }

  function logout() {
    state.token = '';
    state.user = null;
    localStorage.removeItem('runpilot_token');
    showLogin();
  }

  // --------------------------------------------------------------------------
  // Data Loaders
  // --------------------------------------------------------------------------

  async function loadOverview() {
    try {
      // 1. Fetch Server Metrics
      const stats = await api('/api/v1/servers/stats');
      el.metricCpuUsage.textContent = `${stats.cpu_usage_pct.toFixed(1)}%`;
      el.metricRamUsage.textContent = `${stats.memory_used_mb} MB`;
      el.metricDiskUsage.textContent = `${stats.disk_used_gb} GB`;
      el.topbarHostName.textContent = `${stats.hostname || 'Linux'} (${stats.os || 'systemd'})`;

      // 2. Fetch Apps
      const apps = await api('/api/v1/applications');
      state.applications = apps || [];

      el.metricTotalApps.textContent = state.applications.length;
      const runningCount = state.applications.filter(a => a.status === 'running').length;
      const stoppedCount = state.applications.length - runningCount;
      el.metricRunningApps.textContent = `${runningCount} running • ${stoppedCount} stopped`;

      renderOverviewAppsTable(state.applications);

      // 3. Fetch Recent Audit Logs
      const audit = await api('/api/v1/audit-events');
      state.auditLogs = audit || [];
      renderOverviewAuditTable(state.auditLogs.slice(0, 5));
    } catch (err) {
      showToast(`Failed to load dashboard metrics: ${err.message}`, 'error');
    }
  }

  function renderOverviewAppsTable(apps) {
    if (!apps || apps.length === 0) {
      el.overviewAppsTableBody.innerHTML = `
        <tr>
          <td colspan="7">
            <div class="empty-state">
              <div class="empty-title">No applications created yet</div>
              <div class="empty-desc">Deploy your first Node.js or Python application directly to native Linux processes.</div>
              <button class="btn btn-primary btn-sm" onclick="document.getElementById('btn-create-app-top').click()">Create First App</button>
            </div>
          </td>
        </tr>
      `;
      return;
    }

    el.overviewAppsTableBody.innerHTML = apps.map(app => `
      <tr>
        <td>
          <div style="font-weight: 600; color: var(--text-primary);">${escapeHtml(app.name)}</div>
          <div style="font-size: 11px; color: var(--text-muted); font-family: var(--font-mono);">${escapeHtml(app.id)}</div>
        </td>
        <td>
          <span class="runtime-badge">
            ${app.runtime_type === 'nodejs' ? '⬢ Node.js' : '🐍 Python'} ${escapeHtml(app.runtime_version)}
          </span>
        </td>
        <td>
          <span style="font-family: var(--font-mono); font-size: 12px; color: var(--text-secondary);">
            ${escapeHtml(app.git_branch)}
          </span>
        </td>
        <td><span style="font-family: var(--font-mono);">${app.port}</span></td>
        <td>
          <span class="badge ${app.status === 'running' ? 'badge-success' : 'badge-danger'}">
            ${app.status}
          </span>
        </td>
        <td>
          <span style="font-family: var(--font-mono); font-size: 11px; color: var(--text-secondary);">
            ${app.current_release ? escapeHtml(app.current_release) : '<em>Never deployed</em>'}
          </span>
        </td>
        <td style="text-align: right;">
          <button class="btn btn-primary btn-sm" onclick="window.runpilot.deployApp('${app.id}')">Deploy</button>
          <button class="btn btn-secondary btn-sm" onclick="window.runpilot.restartApp('${app.id}')">Restart</button>
        </td>
      </tr>
    `).join('');
  }

  function renderOverviewAuditTable(events) {
    if (!events || events.length === 0) {
      el.overviewAuditTableBody.innerHTML = `
        <tr><td colspan="5" style="text-align: center; color: var(--text-muted); padding: 20px;">No audit events logged yet.</td></tr>
      `;
      return;
    }

    el.overviewAuditTableBody.innerHTML = events.map(ev => `
      <tr>
        <td style="font-family: var(--font-mono); font-size: 11px;">${new Date(ev.created_at).toLocaleString()}</td>
        <td>${escapeHtml(ev.user_email || 'system')}</td>
        <td><span class="badge badge-info">${escapeHtml(ev.action)}</span></td>
        <td style="font-family: var(--font-mono); font-size: 11px;">${escapeHtml(ev.app_id || '-')}</td>
        <td style="color: var(--text-secondary);">${escapeHtml(ev.details || '')}</td>
      </tr>
    `).join('');
  }

  async function loadApplications() {
    try {
      const apps = await api('/api/v1/applications');
      state.applications = apps || [];
      renderFullAppsTable(state.applications);
    } catch (err) {
      showToast(`Error loading applications: ${err.message}`, 'error');
    }
  }

  function renderFullAppsTable(apps) {
    if (!apps || apps.length === 0) {
      el.appsFullTableBody.innerHTML = `
        <tr>
          <td colspan="7">
            <div class="empty-state">
              <div class="empty-title">No applications configured</div>
              <div class="empty-desc">Create your first application to configure Git repositories, runtime versions, and ports.</div>
            </div>
          </td>
        </tr>
      `;
      return;
    }

    el.appsFullTableBody.innerHTML = apps.map(app => `
      <tr>
        <td>
          <div style="font-weight: 600; color: var(--text-primary);">${escapeHtml(app.name)}</div>
          <div style="font-size: 11px; color: var(--text-muted); font-family: var(--font-mono);">ID: ${escapeHtml(app.id)} • User: ${escapeHtml(app.assigned_unix_user)}</div>
        </td>
        <td>
          <span class="runtime-badge">
            ${app.runtime_type === 'nodejs' ? '⬢ Node.js' : '🐍 Python'} ${escapeHtml(app.runtime_version)}
          </span>
        </td>
        <td>
          <div style="font-size: 12px; font-family: var(--font-mono);">${escapeHtml(app.git_repo || 'No Git URL')}</div>
          <div style="font-size: 11px; color: var(--text-muted);">Branch: ${escapeHtml(app.git_branch)}</div>
        </td>
        <td><span style="font-family: var(--font-mono); font-weight: 600;">${app.port}</span></td>
        <td>
          <span class="badge ${app.status === 'running' ? 'badge-success' : 'badge-danger'}">
            ${app.status}
          </span>
        </td>
        <td>
          <span style="font-family: var(--font-mono); font-size: 11px;">
            ${app.current_release ? escapeHtml(app.current_release) : '<span style="color:var(--text-muted)">None</span>'}
          </span>
        </td>
        <td style="text-align: right; white-space: nowrap;">
          <button class="btn btn-primary btn-sm" onclick="window.runpilot.deployApp('${app.id}')">Deploy</button>
          <button class="btn btn-secondary btn-sm" onclick="window.runpilot.processApp('${app.id}', '${app.status === 'running' ? 'stop' : 'start'}')">
            ${app.status === 'running' ? 'Stop' : 'Start'}
          </button>
          <button class="btn btn-secondary btn-sm" onclick="window.runpilot.viewAppLogs('${app.id}')">Logs</button>
          <button class="btn btn-secondary btn-sm" onclick="window.runpilot.openEnvModal('${app.id}')">Env</button>
          <button class="btn btn-danger btn-sm" onclick="window.runpilot.deleteApp('${app.id}', '${escapeHtml(app.name)}')">Delete</button>
        </td>
      </tr>
    `).join('');
  }

  async function loadDeployments() {
    if (state.applications.length === 0) {
      await loadApplications();
    }
    const allDeployments = [];
    for (const app of state.applications) {
      try {
        const deps = await api(`/api/v1/applications/${app.id}/deployments`);
        if (deps && deps.length) {
          deps.forEach(d => d.app_name = app.name);
          allDeployments.push(...deps);
        }
      } catch (e) {}
    }

    allDeployments.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
    state.deployments = allDeployments;

    if (allDeployments.length === 0) {
      el.deploymentsTableBody.innerHTML = `
        <tr><td colspan="7" style="text-align: center; color: var(--text-muted); padding: 30px;">No deployments recorded yet.</td></tr>
      `;
      return;
    }

    el.deploymentsTableBody.innerHTML = allDeployments.map(d => `
      <tr>
        <td style="font-family: var(--font-mono); font-weight: 600;">${escapeHtml(d.id)}</td>
        <td><strong>${escapeHtml(d.app_name || d.app_id)}</strong></td>
        <td style="font-family: var(--font-mono); font-size: 11px;">${escapeHtml(d.commit_hash)}</td>
        <td>${escapeHtml(d.triggered_by)}</td>
        <td>
          <span class="badge ${d.state === 'successful' ? 'badge-success' : (d.state === 'failed' ? 'badge-danger' : 'badge-warning')}">
            ${escapeHtml(d.state)}
          </span>
        </td>
        <td style="font-size: 11px; color: var(--text-muted); font-family: var(--font-mono);">${new Date(d.created_at).toLocaleString()}</td>
        <td style="text-align: right;">
          <button class="btn btn-secondary btn-sm" onclick="window.runpilot.viewBuildLog('${d.app_id}', '${d.id}')">View Output</button>
        </td>
      </tr>
    `).join('');
  }

  async function loadDomains() {
    if (state.applications.length === 0) {
      await loadApplications();
    }

    const domainList = [];
    state.applications.forEach(app => {
      if (app.domains && app.domains.length) {
        app.domains.forEach(d => {
          domainList.push({
            domain: d,
            app_id: app.id,
            app_name: app.name,
            port: app.port,
          });
        });
      }
    });

    if (domainList.length === 0) {
      el.domainsTableBody.innerHTML = `
        <tr>
          <td colspan="6">
            <div class="empty-state">
              <div class="empty-title">No custom domains mapped</div>
              <div class="empty-desc">Attach a domain to automatically configure an Nginx virtual host with reverse proxy routing.</div>
              <button class="btn btn-primary btn-sm" onclick="document.getElementById('btn-attach-domain-open').click()">Attach Domain</button>
            </div>
          </td>
        </tr>
      `;
      return;
    }

    el.domainsTableBody.innerHTML = domainList.map(item => `
      <tr>
        <td><strong style="color: var(--accent-green);">${escapeHtml(item.domain)}</strong></td>
        <td>${escapeHtml(item.app_name)} (${escapeHtml(item.app_id)})</td>
        <td><code style="font-family: var(--font-mono);">127.0.0.1:${item.port}</code></td>
        <td><span class="badge badge-neutral">HTTP / Ready for ACME</span></td>
        <td><span class="badge badge-success">Nginx Routed</span></td>
        <td style="text-align: right;">
          <button class="btn btn-danger btn-sm" onclick="window.runpilot.removeDomain('${item.app_id}', '${escapeHtml(item.domain)}')">Detach</button>
        </td>
      </tr>
    `).join('');
  }

  function populateLogApps() {
    if (state.applications.length === 0) {
      loadApplications().then(() => renderLogAppOptions());
    } else {
      renderLogAppOptions();
    }
  }

  function renderLogAppOptions() {
    el.logAppSelect.innerHTML = state.applications.map(a => `
      <option value="${a.id}">${escapeHtml(a.name)} (${a.id})</option>
    `).join('');

    if (state.applications.length > 0) {
      state.activeLogAppId = state.applications[0].id;
      fetchLogs();
    } else {
      el.liveTerminalViewer.textContent = '[RunPilot] No applications available. Create one to see live logs.';
    }
  }

  async function fetchLogs() {
    if (!state.activeLogAppId) return;
    const lines = el.logLinesSelect.value || 50;
    try {
      const data = await api(`/api/v1/applications/${state.activeLogAppId}/logs?lines=${lines}`);
      if (data && data.lines && data.lines.length) {
        el.liveTerminalViewer.textContent = data.lines.join('\n');
        el.liveTerminalViewer.scrollTop = el.liveTerminalViewer.scrollHeight;
      } else {
        el.liveTerminalViewer.textContent = `[RunPilot] No logs recorded for application: ${state.activeLogAppId}`;
      }
    } catch (err) {
      el.liveTerminalViewer.textContent = `[RunPilot Log Error] ${err.message}`;
    }
  }

  async function loadAuditLogs() {
    try {
      const logs = await api('/api/v1/audit-events');
      if (!logs || logs.length === 0) {
        el.fullAuditTableBody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 20px;">No events recorded.</td></tr>`;
        return;
      }
      el.fullAuditTableBody.innerHTML = logs.map(ev => `
        <tr>
          <td style="font-family: var(--font-mono); font-size: 11px;">#${ev.id}</td>
          <td style="font-family: var(--font-mono); font-size: 11px;">${new Date(ev.created_at).toLocaleString()}</td>
          <td>${escapeHtml(ev.user_email || 'system')}</td>
          <td><span class="badge badge-info">${escapeHtml(ev.action)}</span></td>
          <td style="font-family: var(--font-mono); font-size: 11px;">${escapeHtml(ev.app_id || '-')}</td>
          <td style="font-family: var(--font-mono); font-size: 11px;">${escapeHtml(ev.ip_address || '-')}</td>
          <td style="color: var(--text-secondary);">${escapeHtml(ev.details || '')}</td>
        </tr>
      `).join('');
    } catch (err) {
      showToast(`Error loading audit logs: ${err.message}`, 'error');
    }
  }

  // --------------------------------------------------------------------------
  // Application Actions (Exposed to window.runpilot)
  // --------------------------------------------------------------------------

  window.runpilot = {
    async deployApp(appID) {
      showToast(`Initiating deployment for ${appID}...`, 'success');
      try {
        const dep = await api(`/api/v1/applications/${appID}/deploy`, { method: 'POST' });
        showToast(`Deployment ${dep.id} succeeded! App status: successful`, 'success');
        loadOverview();
        if (state.currentPanel === 'applications') loadApplications();
      } catch (err) {
        showToast(`Deployment failed: ${err.message}`, 'error');
        loadOverview();
      }
    },

    async restartApp(appID) {
      showToast(`Restarting application process ${appID}...`, 'success');
      try {
        const res = await api(`/api/v1/applications/${appID}/restart`, { method: 'POST' });
        showToast(`Application ${appID} restarted (State: ${res.state})`, 'success');
        loadOverview();
      } catch (err) {
        showToast(`Failed to restart: ${err.message}`, 'error');
      }
    },

    async processApp(appID, action) {
      showToast(`Executing ${action} on ${appID}...`, 'success');
      try {
        const res = await api(`/api/v1/applications/${appID}/${action}`, { method: 'POST' });
        showToast(`Process ${appID} is now ${res.state}`, 'success');
        loadApplications();
        loadOverview();
      } catch (err) {
        showToast(`Action failed: ${err.message}`, 'error');
      }
    },

    async deleteApp(appID, appName) {
      if (!confirm(`Are you sure you want to permanently delete application "${appName}" (${appID})?\n\nThis will stop its native Linux process and remove its configuration.`)) {
        return;
      }
      try {
        await api(`/api/v1/applications/${appID}`, { method: 'DELETE' });
        showToast(`Application ${appName} deleted successfully`, 'success');
        loadApplications();
        loadOverview();
      } catch (err) {
        showToast(`Failed to delete application: ${err.message}`, 'error');
      }
    },

    async viewBuildLog(appID, depID) {
      el.buildLogTitle.textContent = `Build Output — Deployment ${depID}`;
      el.buildLogContent.textContent = 'Loading build output from agent...';
      el.modalBuildLog.classList.add('active');

      try {
        const data = await api(`/api/v1/applications/${appID}/deployments/${depID}/log`);
        el.buildLogContent.textContent = data.log || '[RunPilot] No build output recorded for this release.';
      } catch (err) {
        el.buildLogContent.textContent = `Failed to fetch build log: ${err.message}`;
      }
    },

    viewAppLogs(appID) {
      switchPanel('logs');
      el.logAppSelect.value = appID;
      state.activeLogAppId = appID;
      fetchLogs();
    },

    async openEnvModal(appID) {
      el.envAppId.value = appID;
      el.envModalTitle.textContent = `Environment Variables — ${appID}`;
      el.envVarsContainer.innerHTML = 'Loading variables...';
      el.modalEnvVars.classList.add('active');

      try {
        const envVars = await api(`/api/v1/applications/${appID}/env`);
        renderEnvRows(envVars);
      } catch (err) {
        showToast(`Failed to load environment variables: ${err.message}`, 'error');
      }
    },

    async removeDomain(appID, domain) {
      if (!confirm(`Remove domain "${domain}" from application ${appID}? Nginx configuration will be updated.`)) {
        return;
      }
      try {
        await api(`/api/v1/applications/${appID}/domains?domain=${encodeURIComponent(domain)}`, { method: 'DELETE' });
        showToast(`Domain ${domain} removed successfully`, 'success');
        loadDomains();
      } catch (err) {
        showToast(`Failed to remove domain: ${err.message}`, 'error');
      }
    }
  };

  function renderEnvRows(vars) {
    el.envVarsContainer.innerHTML = '';
    const entries = Object.entries(vars || {});
    if (entries.length === 0) {
      addEnvRow('', '');
      return;
    }
    entries.forEach(([k, v]) => addEnvRow(k, v));
  }

  function addEnvRow(key = '', val = '') {
    const row = document.createElement('div');
    row.className = 'env-var-row';
    row.style = 'display: flex; gap: 8px; margin-bottom: 8px;';
    row.innerHTML = `
      <input type="text" class="form-control env-key" placeholder="KEY_NAME" value="${escapeHtml(key)}" style="flex: 1; font-family: var(--font-mono); font-size: 12px;" required>
      <input type="text" class="form-control env-val" placeholder="Value" value="${escapeHtml(val)}" style="flex: 2; font-family: var(--font-mono); font-size: 12px;">
      <button type="button" class="btn btn-danger btn-sm" onclick="this.parentElement.remove()" style="padding: 4px 8px;">✕</button>
    `;
    el.envVarsContainer.appendChild(row);
  }

  // --------------------------------------------------------------------------
  // Event Listeners
  // --------------------------------------------------------------------------

  // Login Submission
  el.loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    el.loginError.style.display = 'none';

    try {
      const res = await api('/api/v1/auth/login', {
        method: 'POST',
        body: {
          identifier: el.loginEmail.value.trim(),
          email: el.loginEmail.value.trim(),
          password: el.loginPassword.value,
        },
      });

      state.token = res.token;
      state.user = res.user;
      localStorage.setItem('runpilot_token', res.token);

      el.currentUserEmail.textContent = res.user.username ? `${res.user.username} (${res.user.email})` : res.user.email;
      el.currentUserAvatar.textContent = (res.user.username || res.user.email || 'A').charAt(0).toUpperCase();
      el.currentUserRole.textContent = res.user.role === 'admin' ? 'Administrator' : 'Client';
      if (typeof populateProfileInputs === 'function') {
        populateProfileInputs(res.user);
      }

      hideLogin();
      switchPanel('overview');
      showToast('Welcome back to RunPilot!', 'success');
    } catch (err) {
      el.loginError.textContent = err.message;
      el.loginError.style.display = 'block';
    }
  });

  // Logout
  el.btnLogout.addEventListener('click', logout);

  // Navigation Click Handlers
  el.navItems.forEach(item => {
    item.addEventListener('click', () => {
      const panel = item.getAttribute('data-panel');
      if (panel) switchPanel(panel);
    });
  });

  // Refresh Overview
  el.btnRefreshOverview.addEventListener('click', () => {
    showToast('Refreshing overview statistics...', 'success');
    loadOverview();
  });

  // Modal Openers
  const openCreateModal = () => {
    el.formCreateApp.reset();
    el.modalCreateApp.classList.add('active');
  };
  el.btnCreateAppTop.addEventListener('click', openCreateModal);
  el.btnCreateAppSub.addEventListener('click', openCreateModal);

  el.btnAttachDomainOpen.addEventListener('click', () => {
    el.domainAppId.innerHTML = state.applications.map(a => `
      <option value="${a.id}">${escapeHtml(a.name)} (${a.id})</option>
    `).join('');
    el.domainNameInput.value = '';
    el.modalAttachDomain.classList.add('active');
  });

  // Modal Closers
  document.querySelectorAll('[data-close]').forEach(btn => {
    btn.addEventListener('click', () => {
      const modalId = btn.getAttribute('data-close');
      document.getElementById(modalId).classList.remove('active');
    });
  });

  // Runtime Type switch in Create Modal
  el.appRuntimeType.addEventListener('change', () => {
    const isNode = el.appRuntimeType.value === 'nodejs';
    if (isNode) {
      el.appRuntimeVersion.innerHTML = `
        <option value="20.x">Node.js 20.x (LTS)</option>
        <option value="22.x">Node.js 22.x</option>
        <option value="18.x">Node.js 18.x</option>
      `;
      document.getElementById('app-port').value = 3000;
      document.getElementById('app-start-cmd').placeholder = 'e.g. node server.js or npm start';
    } else {
      el.appRuntimeVersion.innerHTML = `
        <option value="3.11">Python 3.11</option>
        <option value="3.12">Python 3.12</option>
        <option value="3.10">Python 3.10</option>
      `;
      document.getElementById('app-port').value = 8000;
      document.getElementById('app-start-cmd').placeholder = 'e.g. gunicorn main:app or uvicorn main:app';
    }
  });

  // Create App Form Submission
  el.formCreateApp.addEventListener('submit', async (e) => {
    e.preventDefault();
    const payload = {
      name: document.getElementById('app-name').value.trim(),
      runtime_type: el.appRuntimeType.value,
      runtime_version: el.appRuntimeVersion.value,
      git_repo: document.getElementById('app-git-repo').value.trim(),
      git_branch: document.getElementById('app-git-branch').value.trim() || 'main',
      port: parseInt(document.getElementById('app-port').value, 10),
      health_path: document.getElementById('app-health-path').value.trim() || '/',
      build_command: document.getElementById('app-build-cmd').value.trim(),
      start_command: document.getElementById('app-start-cmd').value.trim(),
      package: document.getElementById('app-package-select') ? document.getElementById('app-package-select').value : 'starter',
      resource_limits: {
        cpu_quota: document.getElementById('app-cpu-quota').value.trim() || '100%',
        memory_max: document.getElementById('app-ram-max').value.trim() || '512M',
      }
    };

    try {
      const created = await api('/api/v1/applications', {
        method: 'POST',
        body: payload,
      });

      el.modalCreateApp.classList.remove('active');
      showToast(`Application "${created.name}" created successfully!`, 'success');
      loadOverview();
      if (state.currentPanel === 'applications') loadApplications();
    } catch (err) {
      showToast(`Creation failed: ${err.message}`, 'error');
    }
  });

  // Attach Domain Form Submission
  el.formAttachDomain.addEventListener('submit', async (e) => {
    e.preventDefault();
    const appID = el.domainAppId.value;
    const domain = el.domainNameInput.value.trim();

    try {
      await api(`/api/v1/applications/${appID}/domains`, {
        method: 'POST',
        body: { domain },
      });
      el.modalAttachDomain.classList.remove('active');
      showToast(`Domain ${domain} attached successfully!`, 'success');
      loadDomains();
    } catch (err) {
      showToast(`Failed to attach domain: ${err.message}`, 'error');
    }
  });

  // Env Vars Form Submission
  el.btnAddEnvRow.addEventListener('click', () => addEnvRow());
  el.formEnvVars.addEventListener('submit', async (e) => {
    e.preventDefault();
    const appID = el.envAppId.value;
    const rows = el.envVarsContainer.querySelectorAll('.env-var-row');
    const vars = {};

    rows.forEach(r => {
      const k = r.querySelector('.env-key').value.trim();
      const v = r.querySelector('.env-val').value;
      if (k) vars[k] = v;
    });

    try {
      await api(`/api/v1/applications/${appID}/env`, {
        method: 'POST',
        body: vars,
      });
      el.modalEnvVars.classList.remove('active');
      showToast('Environment variables encrypted and updated!', 'success');
    } catch (err) {
      showToast(`Failed to save env vars: ${err.message}`, 'error');
    }
  });

  // Log Viewer Controls
  el.logAppSelect.addEventListener('change', () => {
    state.activeLogAppId = el.logAppSelect.value;
    fetchLogs();
  });
  el.logLinesSelect.addEventListener('change', fetchLogs);
  el.btnRefreshLogs.addEventListener('click', fetchLogs);

  // Refresh Audit
  el.btnRefreshAudit.addEventListener('click', loadAuditLogs);

  // --------------------------------------------------------------------------
  // Mobile Drawer Navigation
  // --------------------------------------------------------------------------
  const sidebar = document.getElementById('sidebar');
  const sidebarBackdrop = document.getElementById('sidebar-backdrop');
  const btnMobileMenu = document.getElementById('btn-mobile-menu');

  function openMobileSidebar() {
    if (sidebar) sidebar.classList.add('open');
    if (sidebarBackdrop) sidebarBackdrop.classList.add('active');
  }

  function closeMobileSidebar() {
    if (sidebar) sidebar.classList.remove('open');
    if (sidebarBackdrop) sidebarBackdrop.classList.remove('active');
  }

  if (btnMobileMenu) {
    btnMobileMenu.addEventListener('click', () => {
      if (sidebar && sidebar.classList.contains('open')) {
        closeMobileSidebar();
      } else {
        openMobileSidebar();
      }
    });
  }

  if (sidebarBackdrop) {
    sidebarBackdrop.addEventListener('click', closeMobileSidebar);
  }

  // Close modals when clicking overlay backdrop
  document.querySelectorAll('.modal-overlay').forEach(overlay => {
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) {
        overlay.classList.remove('active');
      }
    });
  });

  // Package select auto-fills quotas in Create App Modal
  const appPackageSelect = document.getElementById('app-package-select');
  if (appPackageSelect) {
    appPackageSelect.addEventListener('change', () => {
      const val = appPackageSelect.value;
      const cpuInput = document.getElementById('app-cpu-quota');
      const ramInput = document.getElementById('app-ram-max');
      if (val === 'starter') {
        if (cpuInput) cpuInput.value = '100%';
        if (ramInput) ramInput.value = '512M';
      } else if (val === 'professional') {
        if (cpuInput) cpuInput.value = '150%';
        if (ramInput) ramInput.value = '1G';
      } else if (val === 'business') {
        if (cpuInput) cpuInput.value = '200%';
        if (ramInput) ramInput.value = '2G';
      }
    });
  }

  // --------------------------------------------------------------------------
  // Profile & Password Management
  // --------------------------------------------------------------------------
  const modalProfile = document.getElementById('modal-profile');
  const btnOpenProfile = document.getElementById('btn-open-profile');
  const sidebarUserSnippet = document.getElementById('sidebar-user-snippet');

  function populateProfileInputs(user) {
    if (!user) return;
    const uName = user.username || (user.email ? user.email.split('@')[0] : 'admin');
    const uEmail = user.email || '';
    const uFullName = user.full_name || '';

    const mU = document.getElementById('modal-profile-username');
    const mE = document.getElementById('modal-profile-email');
    const mF = document.getElementById('modal-profile-fullname');
    if (mU) mU.value = uName;
    if (mE) mE.value = uEmail;
    if (mF) mF.value = uFullName;

    const sU = document.getElementById('settings-profile-username');
    const sE = document.getElementById('settings-profile-email');
    const sF = document.getElementById('settings-profile-fullname');
    if (sU) sU.value = uName;
    if (sE) sE.value = uEmail;
    if (sF) sF.value = uFullName;
  }

  if (btnOpenProfile) {
    btnOpenProfile.addEventListener('click', () => {
      populateProfileInputs(state.user);
      if (modalProfile) modalProfile.classList.add('active');
    });
  }
  if (sidebarUserSnippet) {
    sidebarUserSnippet.addEventListener('click', () => {
      populateProfileInputs(state.user);
      if (modalProfile) modalProfile.classList.add('active');
    });
  }

  async function handleProfileUpdate(username, email, fullName) {
    try {
      const updated = await api('/api/v1/profile', {
        method: 'PUT',
        body: { username, email, full_name: fullName },
      });
      state.user = { ...state.user, ...updated };
      el.currentUserEmail.textContent = updated.username ? `${updated.username} (${updated.email})` : updated.email;
      el.currentUserAvatar.textContent = (updated.username || updated.email || 'A').charAt(0).toUpperCase();
      populateProfileInputs(state.user);
      showToast('Profile credentials updated successfully!', 'success');
      if (modalProfile) modalProfile.classList.remove('active');
    } catch (err) {
      showToast(`Failed to update profile: ${err.message}`, 'error');
    }
  }

  const formModalProfile = document.getElementById('form-modal-profile');
  if (formModalProfile) {
    formModalProfile.addEventListener('submit', (e) => {
      e.preventDefault();
      const u = document.getElementById('modal-profile-username').value.trim();
      const em = document.getElementById('modal-profile-email').value.trim();
      const fn = document.getElementById('modal-profile-fullname').value.trim();
      handleProfileUpdate(u, em, fn);
    });
  }

  const formSettingsProfile = document.getElementById('form-settings-profile');
  if (formSettingsProfile) {
    formSettingsProfile.addEventListener('submit', (e) => {
      e.preventDefault();
      const u = document.getElementById('settings-profile-username').value.trim();
      const em = document.getElementById('settings-profile-email').value.trim();
      const fn = document.getElementById('settings-profile-fullname').value.trim();
      handleProfileUpdate(u, em, fn);
    });
  }

  async function handlePasswordChange(oldPassword, newPassword, confirmPassword) {
    if (newPassword !== confirmPassword) {
      showToast('New passwords do not match!', 'error');
      return;
    }
    if (newPassword.length < 6) {
      showToast('New password must be at least 6 characters!', 'error');
      return;
    }
    try {
      await api('/api/v1/profile/password', {
        method: 'PUT',
        body: { old_password: oldPassword, new_password: newPassword },
      });
      showToast('Password changed successfully!', 'success');
      if (modalProfile) modalProfile.classList.remove('active');
      const f1 = document.getElementById('form-modal-password');
      if (f1) f1.reset();
      const f2 = document.getElementById('form-settings-password');
      if (f2) f2.reset();
    } catch (err) {
      showToast(`Failed to change password: ${err.message}`, 'error');
    }
  }

  const formModalPassword = document.getElementById('form-modal-password');
  if (formModalPassword) {
    formModalPassword.addEventListener('submit', (e) => {
      e.preventDefault();
      const oldP = document.getElementById('modal-old-pass').value;
      const newP = document.getElementById('modal-new-pass').value;
      const confP = document.getElementById('modal-confirm-pass').value;
      handlePasswordChange(oldP, newP, confP);
    });
  }

  const formSettingsPassword = document.getElementById('form-settings-password');
  if (formSettingsPassword) {
    formSettingsPassword.addEventListener('submit', (e) => {
      e.preventDefault();
      const oldP = document.getElementById('settings-old-pass').value;
      const newP = document.getElementById('settings-new-pass').value;
      const confP = document.getElementById('settings-confirm-pass').value;
      handlePasswordChange(oldP, newP, confP);
    });
  }

  // --------------------------------------------------------------------------
  // Hosting Packages (Plans) Management
  // --------------------------------------------------------------------------
  async function loadPackages() {
    const tableBody = document.getElementById('packages-table-body');
    if (!tableBody) return;
    try {
      const pkgs = await api('/api/v1/packages');
      if (!pkgs || pkgs.length === 0) {
        tableBody.innerHTML = '<tr><td colspan="8" style="text-align: center; color: var(--text-muted); padding: 20px;">No hosting packages created yet</td></tr>';
        return;
      }
      tableBody.innerHTML = pkgs.map(p => `
        <tr>
          <td><strong style="color: var(--text-primary);">${escapeHtml(p.name)}</strong></td>
          <td><code style="font-family: var(--font-mono); color: var(--info);">${escapeHtml(p.slug)}</code></td>
          <td><span class="badge badge-info">${escapeHtml(p.memory_max || '512M')}</span></td>
          <td><span class="badge badge-neutral">${escapeHtml(p.cpu_quota || '100%')}</span></td>
          <td>${p.tasks_max || 128}</td>
          <td>${escapeHtml(p.disk_quota || '5G')}</td>
          <td>${p.max_apps || 1}</td>
          <td>${p.max_domains || 2}</td>
        </tr>
      `).join('');
    } catch (err) {
      tableBody.innerHTML = `<tr><td colspan="8" style="color: var(--error); padding: 20px;">Failed to load packages: ${err.message}</td></tr>`;
    }
  }

  const btnCreatePkgOpen = document.getElementById('btn-create-pkg-open');
  const modalCreatePkg = document.getElementById('modal-create-pkg');
  const formCreatePkg = document.getElementById('form-create-pkg');
  if (btnCreatePkgOpen && modalCreatePkg) {
    btnCreatePkgOpen.addEventListener('click', () => modalCreatePkg.classList.add('active'));
  }
  if (formCreatePkg) {
    formCreatePkg.addEventListener('submit', async (e) => {
      e.preventDefault();
      const payload = {
        name: document.getElementById('pkg-name').value.trim(),
        slug: document.getElementById('pkg-slug').value.trim(),
        memory_max: document.getElementById('pkg-ram').value.trim(),
        cpu_quota: document.getElementById('pkg-cpu').value.trim(),
        disk_quota: document.getElementById('pkg-disk').value.trim(),
        tasks_max: parseInt(document.getElementById('pkg-tasks').value, 10) || 128,
        max_apps: parseInt(document.getElementById('pkg-max-apps').value, 10) || 1,
        max_domains: parseInt(document.getElementById('pkg-max-domains').value, 10) || 2,
      };
      try {
        await api('/api/v1/packages', {
          method: 'POST',
          body: payload,
        });
        showToast(`Hosting package "${payload.name}" created!`, 'success');
        modalCreatePkg.classList.remove('active');
        formCreatePkg.reset();
        loadPackages();
      } catch (err) {
        showToast(`Failed to create package: ${err.message}`, 'error');
      }
    });
  }

  // --------------------------------------------------------------------------
  // Users & Teams Management
  // --------------------------------------------------------------------------
  async function loadUsers() {
    const tableBody = document.getElementById('users-table-body');
    if (!tableBody) return;
    try {
      const users = await api('/api/v1/users');
      if (!users || users.length === 0) {
        tableBody.innerHTML = '<tr><td colspan="8" style="text-align: center; color: var(--text-muted); padding: 20px;">No users found</td></tr>';
        return;
      }
      tableBody.innerHTML = users.map(u => `
        <tr>
          <td>${u.id}</td>
          <td><strong style="color: var(--text-primary); font-family: var(--font-mono);">${escapeHtml(u.username || '—')}</strong></td>
          <td>${escapeHtml(u.email)}</td>
          <td>${escapeHtml(u.full_name || '—')}</td>
          <td><span class="badge ${u.role === 'admin' ? 'badge-info' : 'badge-neutral'}">${escapeHtml(u.role)}</span></td>
          <td><span class="badge ${u.is_active ? 'badge-success' : 'badge-danger'}">${u.is_active ? 'Active' : 'Disabled'}</span></td>
          <td style="font-size: 11px; color: var(--text-muted);">${u.created_at ? new Date(u.created_at).toLocaleDateString() : 'System'}</td>
          <td>
            <button class="btn btn-secondary btn-sm" onclick="window.runpilotResetUserPass('${escapeHtml(u.username || u.email)}')">
              Reset Password
            </button>
          </td>
        </tr>
      `).join('');
    } catch (err) {
      tableBody.innerHTML = `<tr><td colspan="8" style="color: var(--error); padding: 20px;">Failed to load users: ${err.message}</td></tr>`;
    }
  }

  window.runpilotResetUserPass = function(identifier) {
    const modal = document.getElementById('modal-reset-user-pass');
    const targetInput = document.getElementById('reset-target-user');
    const desc = document.getElementById('reset-user-desc');
    if (targetInput) targetInput.value = identifier;
    if (desc) desc.textContent = `Set a new password for user "${identifier}".`;
    if (modal) modal.classList.add('active');
  };

  const btnCreateUserOpen = document.getElementById('btn-create-user-open');
  const modalCreateUser = document.getElementById('modal-create-user');
  const formCreateUser = document.getElementById('form-create-user');
  if (btnCreateUserOpen && modalCreateUser) {
    btnCreateUserOpen.addEventListener('click', () => modalCreateUser.classList.add('active'));
  }
  if (formCreateUser) {
    formCreateUser.addEventListener('submit', async (e) => {
      e.preventDefault();
      const payload = {
        username: document.getElementById('new-user-username').value.trim(),
        email: document.getElementById('new-user-email').value.trim(),
        full_name: document.getElementById('new-user-fullname').value.trim(),
        password: document.getElementById('new-user-password').value,
        role: document.getElementById('new-user-role').value,
      };
      try {
        await api('/api/v1/users', {
          method: 'POST',
          body: payload,
        });
        showToast(`User account "${payload.username}" created successfully!`, 'success');
        modalCreateUser.classList.remove('active');
        formCreateUser.reset();
        loadUsers();
      } catch (err) {
        showToast(`Failed to create user: ${err.message}`, 'error');
      }
    });
  }

  const formResetUserPass = document.getElementById('form-reset-user-pass');
  const modalResetUserPass = document.getElementById('modal-reset-user-pass');
  if (formResetUserPass) {
    formResetUserPass.addEventListener('submit', async (e) => {
      e.preventDefault();
      const identifier = document.getElementById('reset-target-user').value;
      const password = document.getElementById('admin-new-user-pass').value;
      try {
        await api('/api/v1/users/reset-password', {
          method: 'POST',
          body: { identifier, password },
        });
        showToast(`Password for ${identifier} reset successfully!`, 'success');
        if (modalResetUserPass) modalResetUserPass.classList.remove('active');
        formResetUserPass.reset();
      } catch (err) {
        showToast(`Failed to reset password: ${err.message}`, 'error');
      }
    });
  }

  // Helper: Escape HTML
  function escapeHtml(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // Initialize Application
  initAuth();

})();
