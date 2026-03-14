// ── 告警配置 ──

function updateAlertStatusBar(enabled, keywordCount) {
    const dot = document.getElementById('alert-status-dot');
    const text = document.getElementById('alert-status-text');
    const badge = document.getElementById('alert-status-badge');
    if (!enabled) {
        dot.className = 'status-dot warn';
        text.textContent = '告警检测已禁用';
        badge.textContent = '已禁用';
        badge.className = 'card-badge warn';
    } else {
        dot.className = 'status-dot ok';
        text.textContent = '✅ 已启用，共 ' + keywordCount + ' 个关键词，修改立即生效';
        badge.textContent = keywordCount + ' 个关键词';
        badge.className = 'card-badge ok';
    }
}

async function loadAlertConfig() {
    try {
        const res = await fetch('/api/alert-config');
        const cfg = await res.json();
        document.getElementById('alert-enabled').checked = cfg.enabled !== false;
        const keywords = Array.isArray(cfg.keywords) ? cfg.keywords : [];
        document.getElementById('alert-keywords').value = keywords.join('\n');
        const filterKeywords = Array.isArray(cfg.filter_keywords) ? cfg.filter_keywords : [];
        document.getElementById('alert-filter-keywords').value = filterKeywords.join('\n');
        updateAlertStatusBar(cfg.enabled !== false, keywords.length);
    } catch (e) {
        showToast('加载告警配置失败：' + e.message, 'error');
    }
}

async function saveAlertConfig() {
    const enabled = document.getElementById('alert-enabled').checked;
    const keywords = document.getElementById('alert-keywords').value
        .split('\n').map(k => k.trim()).filter(k => k.length > 0);
    const filterKeywords = document.getElementById('alert-filter-keywords').value
        .split('\n').map(k => k.trim()).filter(k => k.length > 0);
    try {
        const res = await fetch('/api/alert-config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled, keywords, filter_keywords: filterKeywords }),
        });
        const data = await res.json();
        if (res.ok) {
            showToast('✅ ' + data.message, 'success');
            updateAlertStatusBar(enabled, keywords.length);
        } else {
            showToast('❌ 保存失败：' + data.error, 'error');
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
    }
}
