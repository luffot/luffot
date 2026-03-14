// ── 模型配置 ──

let currentConfig = {};

const PROVIDER_MODELS = {
    chat: ['qwen-plus', 'qwen-turbo', 'qwen-max', 'qwen3-235b-a22b'],
    camera: ['qwen-vl-plus', 'qwen-vl-max', 'gpt-4o'],
    default: ['qwen-plus', 'qwen-turbo', 'qwen-max', 'qwen-vl-plus'],
};

function updateAiStatusBar(cfg) {
    const dot = document.getElementById('ai-status-dot');
    const text = document.getElementById('ai-status-text');
    const badge = document.getElementById('ai-status-badge');
    const providers = cfg.providers || [];
    const hasKey = providers.some(p => p.api_key && p.api_key.trim() !== '');
    if (!cfg.enabled) {
        dot.className = 'status-dot warn';
        text.textContent = 'AI 功能已禁用';
        badge.textContent = '已禁用';
        badge.className = 'card-badge warn';
    } else if (!hasKey) {
        dot.className = 'status-dot err';
        text.textContent = '⚠️ 尚未配置任何 API Key';
        badge.textContent = '未配置';
        badge.className = 'card-badge err';
    } else {
        dot.className = 'status-dot ok';
        text.textContent = '✅ 已配置 ' + providers.length + ' 个 Provider，重启后生效';
        badge.textContent = providers.length + ' 个 Provider';
        badge.className = 'card-badge ok';
    }
}

async function loadSettings() {
    try {
        const res = await fetch('/api/settings');
        const cfg = await res.json();
        currentConfig = cfg;

        document.getElementById('ai-enabled').checked = cfg.enabled || false;
        document.getElementById('default-provider').value = cfg.default_provider || '';
        document.getElementById('max-context-rounds').value = cfg.max_context_rounds || 10;

        renderProviderList(cfg.providers || []);
        updateAiStatusBar(cfg);
    } catch (e) {
        showToast('加载配置失败：' + e.message, 'error');
    }
}

function renderProviderList(providers) {
    const container = document.getElementById('provider-list');
    if (!providers || providers.length === 0) {
        container.innerHTML = '<div style="padding:2rem;text-align:center;color:#9ca3af;font-size:0.875rem;">暂无 Provider，点击「添加」创建</div>';
        return;
    }
    container.innerHTML = providers.map((p, index) => buildProviderRow(p, index)).join('');
}

function buildProviderRow(p, index) {
    const modelSuggestions = (PROVIDER_MODELS[p.name] || PROVIDER_MODELS.default)
        .map(m => `<span class="chip" onclick="selectProviderModel(${index},'${m}')">${m}</span>`).join('');
    return `
    <div class="provider-row" id="provider-row-${index}">
        <div class="provider-row-header">
            <span class="provider-name-tag">${escapeHtml(p.name || 'provider-' + index)}</span>
            <span style="font-size:0.75rem;color:#9ca3af;margin-left:0.5rem;">${escapeHtml(p.provider || 'bailian')}</span>
            <button class="btn btn-danger-outline btn-sm" style="margin-left:auto;" onclick="removeProvider(${index})">🗑 删除</button>
        </div>
        <div class="provider-row-body">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem;margin-bottom:0.8rem;">
                <div class="form-group" style="margin-bottom:0;">
                    <label class="form-label">名称（唯一标识）</label>
                    <input type="text" value="${escapeHtml(p.name || '')}" onchange="updateProvider(${index},'name',this.value)" placeholder="chat">
                </div>
                <div class="form-group" style="margin-bottom:0;">
                    <label class="form-label">服务商类型</label>
                    <select style="width:100%;padding:0.6rem 0.95rem;border:1.5px solid #e5e7eb;border-radius:9px;font-size:0.88rem;color:#1f2937;background:#fafafa;outline:none;font-family:inherit;" onchange="updateProvider(${index},'provider',this.value)">
                        <option value="bailian" ${p.provider==='bailian'?'selected':''}>bailian（阿里云百炼）</option>
                        <option value="openai" ${p.provider==='openai'?'selected':''}>openai（标准 OpenAI）</option>
                        <option value="dashscope" ${p.provider==='dashscope'?'selected':''}>dashscope（DashScope 原生）</option>
                    </select>
                </div>
            </div>
            <div class="form-group" style="margin-bottom:0.8rem;">
                <label class="form-label">API Key <span style="color:#ef4444">*</span></label>
                <div class="input-row">
                    <input type="password" id="provider-key-${index}" value="${escapeHtml(p.api_key || '')}" onchange="updateProvider(${index},'api_key',this.value)" placeholder="sk-xxxxxxxxxxxxxxxxxxxxxxxx">
                    <button class="btn-eye" onclick="toggleProviderKey(${index})" title="显示/隐藏">👁</button>
                </div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem;margin-bottom:0.8rem;">
                <div class="form-group" style="margin-bottom:0;">
                    <label class="form-label">模型</label>
                    <input type="text" id="provider-model-${index}" value="${escapeHtml(p.model || '')}" onchange="updateProvider(${index},'model',this.value)" placeholder="qwen-plus">
                    <div class="chip-group" style="margin-top:0.5rem;">${modelSuggestions}</div>
                </div>
                <div class="form-group" style="margin-bottom:0;">
                    <label class="form-label">API 地址</label>
                    <input type="text" value="${escapeHtml(p.base_url || '')}" onchange="updateProvider(${index},'base_url',this.value)" placeholder="https://dashscope.aliyuncs.com/compatible-mode/v1">
                </div>
            </div>
        </div>
    </div>`;
}

function updateProvider(index, field, value) {
    if (!currentConfig.providers) currentConfig.providers = [];
    if (!currentConfig.providers[index]) currentConfig.providers[index] = {};
    currentConfig.providers[index][field] = value;
}

function selectProviderModel(index, model) {
    const input = document.getElementById('provider-model-' + index);
    if (input) { input.value = model; updateProvider(index, 'model', model); }
    const row = document.getElementById('provider-row-' + index);
    if (row) row.querySelectorAll('.chip').forEach(c => c.classList.toggle('active', c.textContent === model));
}

function toggleProviderKey(index) {
    const input = document.getElementById('provider-key-' + index);
    if (input) input.type = input.type === 'password' ? 'text' : 'password';
}

function addProvider() {
    if (!currentConfig.providers) currentConfig.providers = [];
    currentConfig.providers.push({ name: 'new-provider', provider: 'bailian', api_key: '', model: 'qwen-plus', base_url: '' });
    renderProviderList(currentConfig.providers);
}

function removeProvider(index) {
    if (!confirm('确定删除此 Provider？')) return;
    currentConfig.providers.splice(index, 1);
    renderProviderList(currentConfig.providers);
}

async function saveSettings() {
    const cfg = {
        enabled: document.getElementById('ai-enabled').checked,
        default_provider: document.getElementById('default-provider').value.trim() || 'chat',
        max_context_rounds: parseInt(document.getElementById('max-context-rounds').value) || 10,
        timeout_seconds: currentConfig.timeout_seconds || 30,
        providers: currentConfig.providers || [],
    };

    try {
        const res = await fetch('/api/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(cfg),
        });
        const data = await res.json();
        if (res.ok) {
            showToast('✅ ' + data.message, 'success');
            currentConfig = { ...currentConfig, ...cfg };
            updateAiStatusBar(cfg);
        } else {
            showToast('❌ 保存失败：' + data.error, 'error');
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
    }
}
