// ── 消息总览 ──

async function loadMsgStats() {
    try {
        const res = await fetch('/api/stats');
        const data = await res.json();
        document.getElementById('msg-total').textContent = data.total_messages || 0;
        document.getElementById('msg-today').textContent = data.today_messages || 0;
        const appCount = Object.keys(data.app_counts || {}).length;
        document.getElementById('msg-app-count').textContent = appCount;
    } catch (e) {
        console.log('加载统计失败:', e);
    }
}

async function loadMsgList() {
    try {
        const res = await fetch('/api/messages?limit=50');
        const data = await res.json();
        const list = document.getElementById('msg-list');
        if (!data.messages || data.messages.length === 0) {
            list.innerHTML = '<div style="text-align:center;color:#9ca3af;padding:2rem;">暂无消息</div>';
            return;
        }
        list.innerHTML = data.messages.map(msg => `
            <div class="msg-item">
                <div class="msg-meta">
                    <span class="msg-sender">${escapeHtml(msg.sender)}</span>
                    <span class="msg-session">${escapeHtml(msg.session)}</span>
                    <span class="msg-timestamp">${new Date(msg.timestamp).toLocaleString()}</span>
                </div>
                <div class="msg-body">${escapeHtml(msg.content)}</div>
            </div>`).join('');
    } catch (e) {
        console.log('加载消息失败:', e);
    }
}

async function loadMsgApps() {
    try {
        const res = await fetch('/api/apps');
        const data = await res.json();
        const list = document.getElementById('msg-app-list');
        if (!data.apps || data.apps.length === 0) {
            list.innerHTML = '<li style="text-align:center;color:#9ca3af;padding:1rem;">暂无应用</li>';
            return;
        }
        list.innerHTML = data.apps.map(app => `
            <li class="msg-app-item">
                <span class="msg-app-name">${escapeHtml(app.display_name || app.name)}</span>
                <span class="msg-app-status ${app.enabled ? 'enabled' : 'disabled'}">${app.enabled ? '运行中' : '已禁用'}</span>
            </li>`).join('');
    } catch (e) {
        console.log('加载应用列表失败:', e);
    }
}

async function searchMessagesInPage() {
    try {
        const query = document.getElementById('msg-search-input').value.trim();
        if (!query) { loadMsgList(); return; }
        const res = await fetch('/api/search?q=' + encodeURIComponent(query));
        const data = await res.json();
        const list = document.getElementById('msg-list');
        if (!data.messages || data.messages.length === 0) {
            list.innerHTML = '<div style="text-align:center;color:#9ca3af;padding:2rem;">未找到相关消息</div>';
            return;
        }
        list.innerHTML = data.messages.map(msg => `
            <div class="msg-item">
                <div class="msg-meta">
                    <span class="msg-sender">${escapeHtml(msg.sender)}</span>
                    <span class="msg-session">${escapeHtml(msg.session)}</span>
                    <span class="msg-timestamp">${new Date(msg.timestamp).toLocaleString()}</span>
                </div>
                <div class="msg-body">${escapeHtml(msg.content)}</div>
            </div>`).join('');
    } catch (e) {
        console.log('搜索失败:', e);
    }
}

function loadMessagesPage() {
    loadMsgStats();
    loadMsgList();
    loadMsgApps();
    if (!messagesRefreshTimer) {
        messagesRefreshTimer = setInterval(() => {
            loadMsgStats();
            loadMsgList();
        }, 5000);
    }
}
