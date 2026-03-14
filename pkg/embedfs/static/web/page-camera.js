// ── 摄像头监测设置 ──

const cameraLogPageSize = 10;
let cameraLogCurrentPage = 0;
let cameraLogTotal = 0;

async function loadCameraLog() {
    const list = document.getElementById('camera-log-list');
    const badge = document.getElementById('camera-log-total-badge');
    list.innerHTML = '<div style="padding:2rem;text-align:center;color:#9ca3af;font-size:0.875rem;">加载中...</div>';
    badge.textContent = '加载中...';

    try {
        const offset = cameraLogCurrentPage * cameraLogPageSize;
        const res = await fetch(`/api/camera-detections?limit=${cameraLogPageSize}&offset=${offset}`);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data = await res.json();

        cameraLogTotal = data.total || 0;
        badge.textContent = `共 ${cameraLogTotal} 条`;

        const detections = data.detections || [];
        if (detections.length === 0) {
            list.innerHTML = '<div style="padding:3rem;text-align:center;color:#9ca3af;font-size:0.875rem;">暂无检测记录<br><span style="font-size:0.78rem;margin-top:0.4rem;display:block;">摄像头守卫检测到背后有人时，记录会自动出现在这里</span></div>';
            document.getElementById('camera-log-pagination').style.display = 'none';
            return;
        }

        list.innerHTML = detections.map(item => buildCameraLogItem(item)).join('');

        const totalPages = Math.ceil(cameraLogTotal / cameraLogPageSize);
        const pagination = document.getElementById('camera-log-pagination');
        if (totalPages > 1) {
            pagination.style.display = 'flex';
            document.getElementById('camera-log-page-info').textContent =
                `第 ${cameraLogCurrentPage + 1} / ${totalPages} 页`;
            document.getElementById('camera-log-prev').disabled = cameraLogCurrentPage === 0;
            document.getElementById('camera-log-next').disabled = cameraLogCurrentPage >= totalPages - 1;
        } else {
            pagination.style.display = 'none';
        }
    } catch (e) {
        list.innerHTML = `<div style="padding:1.5rem;color:#ef4444;font-size:0.875rem;">加载失败：${escapeHtml(e.message)}</div>`;
        badge.textContent = '加载失败';
    }
}

function buildCameraLogItem(item) {
    const thumbHtml = item.image_url
        ? `<div class="guard-log-thumb" onclick="openLightbox('${escapeHtml(item.image_url)}', '${escapeHtml(item.detected_at)}')">
               <img src="${escapeHtml(item.image_url)}" alt="检测截图" loading="lazy">
               <div class="thumb-overlay">🔍</div>
           </div>`
        : `<div class="guard-log-thumb" style="display:flex;align-items:center;justify-content:center;color:#9ca3af;font-size:0.75rem;">图片不存在</div>`;

    const reasonHtml = item.ai_reason
        ? `<div class="guard-log-reason">🤖 ${escapeHtml(item.ai_reason)}</div>`
        : `<div class="guard-log-reason"><span class="guard-log-reason-empty">AI 未提供详细理由（旧版记录）</span></div>`;

    return `
    <div class="guard-log-item">
        ${thumbHtml}
        <div class="guard-log-info">
            <div class="guard-log-time">
                <span>⏰</span>
                <span>${escapeHtml(item.detected_at)}</span>
                <span style="color:#ef4444;font-weight:600;">⚠️ 检测到背后有人</span>
            </div>
            ${reasonHtml}
        </div>
    </div>`;
}

function cameraLogPrevPage() {
    if (cameraLogCurrentPage > 0) {
        cameraLogCurrentPage--;
        loadCameraLog();
    }
}

function cameraLogNextPage() {
    const totalPages = Math.ceil(cameraLogTotal / cameraLogPageSize);
    if (cameraLogCurrentPage < totalPages - 1) {
        cameraLogCurrentPage++;
        loadCameraLog();
    }
}

async function loadCameraSettings() {
    try {
        const res = await fetch('/api/camera-config');
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data = await res.json();

        document.getElementById('camera-enabled').checked = !!data.enabled;
        document.getElementById('camera-interval').value = data.interval_seconds || 10;
        document.getElementById('camera-provider').value = data.provider_name || '';
        document.getElementById('camera-confirm-count').value = data.confirm_count || 2;
        document.getElementById('camera-cooldown').value = data.cooldown_seconds || 60;
    } catch (e) {
        showToast('加载摄像头配置失败：' + e.message, 'error');
    }
}

async function saveCameraSettings() {
    const intervalValue = parseInt(document.getElementById('camera-interval').value, 10);
    if (isNaN(intervalValue) || intervalValue < 5) {
        showToast('检测间隔最小为 5 秒', 'error');
        return;
    }

    const confirmCount = parseInt(document.getElementById('camera-confirm-count').value, 10);
    if (isNaN(confirmCount) || confirmCount < 1) {
        showToast('连续确认次数最小为 1', 'error');
        return;
    }

    const cooldownSeconds = parseInt(document.getElementById('camera-cooldown').value, 10);
    if (isNaN(cooldownSeconds) || cooldownSeconds < 10) {
        showToast('告警冷却时间最小为 10 秒', 'error');
        return;
    }

    const payload = {
        enabled: document.getElementById('camera-enabled').checked,
        interval_seconds: intervalValue,
        provider_name: document.getElementById('camera-provider').value.trim(),
        confirm_count: confirmCount,
        cooldown_seconds: cooldownSeconds,
    };

    try {
        const res = await fetch('/api/camera-config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        showToast('摄像头设置已保存', 'ok');
    } catch (e) {
        showToast('保存失败：' + e.message, 'error');
    }
}
