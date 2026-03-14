// ── 弹幕设置（过滤 + 特别关注）──

let barrageConfig = { filter_keywords: [], highlight_rules: [] };

async function loadBarrageConfig() {
    try {
        const res = await fetch('/api/barrage-config');
        const cfg = await res.json();
        barrageConfig = {
            filter_keywords: Array.isArray(cfg.filter_keywords) ? cfg.filter_keywords : [],
            highlight_rules: Array.isArray(cfg.highlight_rules) ? cfg.highlight_rules : [],
        };
    } catch (e) {
        showToast('加载弹幕配置失败：' + e.message, 'error');
    }
}

async function loadBarrageFilterConfig() {
    await loadBarrageConfig();
    const keywords = barrageConfig.filter_keywords;
    document.getElementById('barrage-filter-keywords').value = keywords.join('\n');

    const badge = document.getElementById('barrage-filter-badge');
    const dot = document.getElementById('barrage-filter-status-dot');
    const statusText = document.getElementById('barrage-filter-status-text');
    if (keywords.length === 0) {
        badge.textContent = '未配置';
        badge.className = 'card-badge warn';
        dot.className = 'status-dot warn';
        statusText.textContent = '暂无过滤关键词，所有消息均会显示为弹幕';
    } else {
        badge.textContent = keywords.length + ' 个关键词';
        badge.className = 'card-badge ok';
        dot.className = 'status-dot ok';
        statusText.textContent = '✅ 已配置 ' + keywords.length + ' 个过滤关键词，修改立即生效';
    }
}

async function saveBarrageFilterConfig() {
    const keywords = document.getElementById('barrage-filter-keywords').value
        .split('\n').map(k => k.trim()).filter(k => k.length > 0);
    barrageConfig.filter_keywords = keywords;
    try {
        const res = await fetch('/api/barrage-config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(barrageConfig),
        });
        const data = await res.json();
        if (res.ok) {
            showToast('✅ ' + data.message, 'success');
            loadBarrageFilterConfig();
        } else {
            showToast('❌ 保存失败：' + data.error, 'error');
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
    }
}

async function loadBarrageHighlightConfig() {
    await loadBarrageConfig();
    renderHighlightRuleList(barrageConfig.highlight_rules);

    const badge = document.getElementById('barrage-highlight-badge');
    const dot = document.getElementById('barrage-highlight-status-dot');
    const statusText = document.getElementById('barrage-highlight-status-text');
    const rules = barrageConfig.highlight_rules;
    if (rules.length === 0) {
        badge.textContent = '未配置';
        badge.className = 'card-badge warn';
        dot.className = 'status-dot warn';
        statusText.textContent = '暂无特别关注规则';
    } else {
        badge.textContent = rules.length + ' 条规则';
        badge.className = 'card-badge ok';
        dot.className = 'status-dot ok';
        statusText.textContent = '✅ 已配置 ' + rules.length + ' 条特别关注规则，修改立即生效';
    }
}

function renderHighlightRuleList(rules) {
    const container = document.getElementById('highlight-rule-list');
    if (!rules || rules.length === 0) {
        container.innerHTML = '<div style="text-align:center;color:#9ca3af;padding:2rem;font-size:0.875rem;">暂无规则，点击「添加规则」创建</div>';
        return;
    }
    container.innerHTML = rules.map((rule, index) => buildHighlightRuleRow(rule, index)).join('');
}

function buildHighlightRuleRow(rule, index) {
    const displayColor = rule.color || '#FFD700';
    return `
    <div class="provider-row" id="highlight-rule-row-${index}">
        <div class="provider-row-header">
            <span class="provider-name-tag" style="background:#fef3c7;color:#92400e;">规则 ${index + 1}</span>
            <span style="font-size:0.75rem;color:#9ca3af;margin-left:0.5rem;">命中关键词时高亮显示</span>
            <button class="btn btn-danger-outline btn-sm" style="margin-left:auto;" onclick="removeHighlightRule(${index})">🗑 删除</button>
        </div>
        <div class="provider-row-body">
            <div style="display:grid;grid-template-columns:1fr auto;gap:0.8rem;align-items:end;">
                <div class="form-group" style="margin-bottom:0;">
                    <label class="form-label">关键词 <span style="color:#ef4444">*</span></label>
                    <input type="text" value="${escapeHtml(rule.keyword || '')}"
                        onchange="updateHighlightRule(${index},'keyword',this.value)"
                        placeholder="例如：@我、紧急、老板">
                    <div class="form-hint">消息内容包含此关键词时，弹幕以指定颜色高亮显示</div>
                </div>
                <div class="form-group" style="margin-bottom:0;">
                    <label class="form-label">高亮颜色</label>
                    <div style="display:flex;align-items:center;gap:0.5rem;">
                        <input type="color" value="${escapeHtml(displayColor)}"
                            id="highlight-color-picker-${index}"
                            onchange="updateHighlightRule(${index},'color',this.value)"
                            style="width:44px;height:38px;border:1.5px solid #e5e7eb;border-radius:9px;cursor:pointer;padding:2px;">
                        <input type="text" value="${escapeHtml(displayColor)}"
                            id="highlight-color-text-${index}"
                            onchange="syncColorFromText(${index},this.value)"
                            placeholder="#FFD700"
                            style="width:100px;font-family:monospace;">
                    </div>
                    <div class="form-hint">留空使用默认金色 #FFD700</div>
                </div>
            </div>
        </div>
    </div>`;
}

function syncColorFromText(index, value) {
    updateHighlightRule(index, 'color', value);
    const picker = document.getElementById('highlight-color-picker-' + index);
    if (picker && /^#[0-9a-fA-F]{6}$/.test(value)) {
        picker.value = value;
    }
}

function updateHighlightRule(index, field, value) {
    if (!barrageConfig.highlight_rules[index]) return;
    barrageConfig.highlight_rules[index][field] = value;
    if (field === 'color') {
        const textInput = document.getElementById('highlight-color-text-' + index);
        if (textInput) textInput.value = value;
    }
}

function addHighlightRule() {
    if (!barrageConfig.highlight_rules) barrageConfig.highlight_rules = [];
    barrageConfig.highlight_rules.push({ keyword: '', color: '#FFD700' });
    renderHighlightRuleList(barrageConfig.highlight_rules);
}

function removeHighlightRule(index) {
    if (!confirm('确定删除此规则？')) return;
    barrageConfig.highlight_rules.splice(index, 1);
    renderHighlightRuleList(barrageConfig.highlight_rules);
}

async function saveBarrageHighlightConfig() {
    barrageConfig.highlight_rules = barrageConfig.highlight_rules.filter(r => r.keyword && r.keyword.trim() !== '');
    try {
        const res = await fetch('/api/barrage-config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(barrageConfig),
        });
        const data = await res.json();
        if (res.ok) {
            showToast('✅ ' + data.message, 'success');
            loadBarrageHighlightConfig();
        } else {
            showToast('❌ 保存失败：' + data.error, 'error');
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
    }
}
