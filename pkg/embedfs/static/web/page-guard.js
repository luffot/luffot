// ── 背后有人记录 ──

const guardLogPageSize = 10;
let guardLogCurrentPage = 0;
let guardLogTotal = 0;

async function loadGuardLog() {
    const list = document.getElementById('guard-log-list');
    const badge = document.getElementById('guard-log-total-badge');
    list.innerHTML = '<div style="padding:2rem;text-align:center;color:#9ca3af;font-size:0.875rem;">加载中...</div>';
    badge.textContent = '加载中...';

    try {
        const offset = guardLogCurrentPage * guardLogPageSize;
        const res = await fetch(`/api/camera-detections?limit=${guardLogPageSize}&offset=${offset}`);
        const data = await res.json();

        guardLogTotal = data.total || 0;
        badge.textContent = `共 ${guardLogTotal} 条`;

        const detections = data.detections || [];
        if (detections.length === 0) {
            list.innerHTML = '<div style="padding:3rem;text-align:center;color:#9ca3af;font-size:0.875rem;">暂无检测记录<br><span style="font-size:0.78rem;margin-top:0.4rem;display:block;">摄像头守卫检测到背后有人时，记录会自动出现在这里</span></div>';
            document.getElementById('guard-log-pagination').style.display = 'none';
            return;
        }

        list.innerHTML = detections.map(item => buildGuardLogItem(item)).join('');

        const totalPages = Math.ceil(guardLogTotal / guardLogPageSize);
        const pagination = document.getElementById('guard-log-pagination');
        if (totalPages > 1) {
            pagination.style.display = 'flex';
            document.getElementById('guard-log-page-info').textContent =
                `第 ${guardLogCurrentPage + 1} / ${totalPages} 页`;
            document.getElementById('guard-log-prev').disabled = guardLogCurrentPage === 0;
            document.getElementById('guard-log-next').disabled = guardLogCurrentPage >= totalPages - 1;
        } else {
            pagination.style.display = 'none';
        }
    } catch (e) {
        list.innerHTML = `<div style="padding:1.5rem;color:#ef4444;font-size:0.875rem;">加载失败：${escapeHtml(e.message)}</div>`;
        badge.textContent = '加载失败';
    }
}

function buildGuardLogItem(item) {
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

function guardLogPrevPage() {
    if (guardLogCurrentPage > 0) {
        guardLogCurrentPage--;
        loadGuardLog();
    }
}

function guardLogNextPage() {
    const totalPages = Math.ceil(guardLogTotal / guardLogPageSize);
    if (guardLogCurrentPage < totalPages - 1) {
        guardLogCurrentPage++;
        loadGuardLog();
    }
}

// ── 图片灯箱 ──
function openLightbox(imageUrl, caption) {
    document.getElementById('lightbox-img').src = imageUrl;
    document.getElementById('lightbox-caption').textContent = caption || '';
    document.getElementById('lightbox').classList.add('open');
    document.body.style.overflow = 'hidden';
}

function closeLightbox() {
    document.getElementById('lightbox').classList.remove('open');
    document.getElementById('lightbox-img').src = '';
    document.body.style.overflow = '';
}