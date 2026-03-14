// ── 皮肤配置 ──

// 皮肤类型对应的 emoji 图标
const SKIN_TYPE_ICONS = {
    vector: '🐦',
    image: '🖼️',
    lua: '🎮',
};

// 皮肤类型对应的标签文字
const SKIN_TYPE_LABELS = {
    vector: '矢量皮肤',
    image: '图片皮肤',
    lua: 'Lua 皮肤',
};

let currentSkinInternal = '';

async function loadSkins() {
    try {
        const res = await fetch('/api/skin');
        const data = await res.json();

        currentSkinInternal = data.current_skin || '';
        renderSkinList(data.skins || []);
        updateSkinStatusBar(currentSkinInternal, data.skins || []);
    } catch (e) {
        showToast('加载皮肤列表失败：' + e.message, 'error');
    }
}

function renderSkinList(skins) {
    const container = document.getElementById('skin-list');
    if (!skins || skins.length === 0) {
        container.innerHTML = '<div style="text-align:center;color:#9ca3af;padding:2rem;font-size:0.875rem;grid-column:1/-1;">暂无可用皮肤</div>';
        return;
    }

    container.innerHTML = skins.map(skin => buildSkinCard(skin)).join('');
}

function buildSkinCard(skin) {
    const isActive = skin.internal === currentSkinInternal;
    const icon = SKIN_TYPE_ICONS[skin.type] || '🎨';
    const typeLabel = SKIN_TYPE_LABELS[skin.type] || skin.type;

    return `
    <div class="skin-card ${isActive ? 'skin-card-active' : ''}" onclick="selectSkin('${escapeHtml(skin.internal)}', '${escapeHtml(skin.name)}')">
        <div class="skin-card-icon">${icon}</div>
        <div class="skin-card-name">${escapeHtml(skin.name)}</div>
        <div class="skin-card-desc">${escapeHtml(skin.description || '')}</div>
        <div class="skin-card-type">${typeLabel}</div>
        ${isActive ? '<div class="skin-card-check">✓ 当前使用</div>' : ''}
    </div>`;
}

async function selectSkin(internalName, displayName) {
    if (internalName === currentSkinInternal) {
        return;
    }

    try {
        const res = await fetch('/api/skin', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ skin_name: internalName }),
        });
        const data = await res.json();

        if (res.ok) {
            currentSkinInternal = internalName;
            showToast('✅ 已切换到「' + displayName + '」，立即生效', 'success');
            // 重新渲染皮肤列表以更新选中状态
            loadSkins();
        } else {
            showToast('❌ 切换失败：' + (data.error || '未知错误'), 'error');
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
    }
}

function updateSkinStatusBar(currentInternal, skins) {
    const dot = document.getElementById('skin-status-dot');
    const text = document.getElementById('skin-status-text');
    const badge = document.getElementById('skin-current-badge');

    const activeSkin = skins.find(s => s.internal === currentInternal);
    const activeName = activeSkin ? activeSkin.name : '经典皮肤';

    if (dot) dot.className = 'status-dot ok';
    if (text) text.textContent = '当前皮肤：' + activeName + '，切换后立即生效';
    if (badge) badge.textContent = activeName;
}

// ── 导入皮肤 ──

/**
 * 切换导入皮肤面板的显示/隐藏
 */
function toggleSkinImportPanel() {
    const panel = document.getElementById('skin-import-panel');
    if (!panel) return;
    const isHidden = panel.style.display === 'none' || panel.style.display === '';
    panel.style.display = isHidden ? 'block' : 'none';
    if (isHidden) {
        document.getElementById('skin-import-dir').focus();
    }
}

/**
 * 导入皮肤：将 skin-builder 生成的皮肤目录注册到桌宠
 */
async function importSkin() {
    const dirInput = document.getElementById('skin-import-dir');
    const nameInput = document.getElementById('skin-import-name');
    const btn = document.getElementById('skin-import-btn');

    const dir = (dirInput.value || '').trim();
    const name = (nameInput.value || '').trim();

    if (!dir) {
        showToast('⚠️ 请输入皮肤目录路径', 'error');
        dirInput.focus();
        return;
    }

    btn.disabled = true;
    btn.textContent = '导入中...';

    try {
        const res = await fetch('/api/skin/import', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ dir, name }),
        });
        const data = await res.json();

        if (res.ok) {
            showToast('✅ ' + data.message, 'success');
            // 清空输入框并隐藏面板
            dirInput.value = '';
            nameInput.value = '';
            document.getElementById('skin-import-panel').style.display = 'none';
            // 刷新皮肤列表
            await loadSkins();
        } else {
            showToast('❌ ' + (data.error || '导入失败'), 'error');
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = '确认导入';
    }
}
