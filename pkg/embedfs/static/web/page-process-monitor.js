// ── 进程监控配置页 ──

let processMonitorApps = [];
let aiProviders = [];
let availablePrompts = [];

// 进程监控相关的提示词
const PROCESS_PROMPT_NAMES = [
    { name: 'analyzer_importance_system', displayName: '消息重要性分析（System）', description: '消息重要性分析助手的角色定义' },
    { name: 'analyzer_importance_user', displayName: '消息重要性分析（User Prompt 模板）', description: '判断消息是否重要的分析模板' },
    { name: 'analyzer_profile_system', displayName: '用户画像分析（System）', description: '用户画像分析助手的角色定义' },
    { name: 'analyzer_profile_user', displayName: '用户画像更新（User Prompt 模板）', description: '根据消息内容生成/更新用户画像的模板' },
    { name: 'vlmodel_message_extract', displayName: 'VLModel 消息识别指令', description: '用于进程监控的视觉模型消息识别 Prompt' },
];
let processPromptContents = {};

// 加载进程监控配置
async function loadProcessMonitorConfig() {
    try {
        // 并行获取应用列表、AI 配置和提示词列表
        const [appsRes, aiRes, promptsRes] = await Promise.all([
            fetch('/api/apps'),
            fetch('/api/config'),
            fetch('/api/prompts')
        ]);

        const appsData = await appsRes.json();
        const aiConfig = await aiRes.json();
        const promptsData = await promptsRes.json();

        processMonitorApps = appsData.apps || [];
        aiProviders = aiConfig.ai?.providers || [];
        availablePrompts = promptsData.prompts || [];

        // 存储进程相关提示词内容
        const allPrompts = promptsData.prompts || [];
        PROCESS_PROMPT_NAMES.forEach(meta => {
            const found = allPrompts.find(p => p.name === meta.name);
            processPromptContents[meta.name] = found ? found.content : '';
        });
        renderProcessPromptList();

        renderProcessMonitorList();
    } catch (error) {
        console.error('加载进程监控配置失败:', error);
        showToast('加载配置失败', 'error');
    }
}

// 渲染进程监控列表
function renderProcessMonitorList() {
    const container = document.getElementById('process-monitor-list');

    if (processMonitorApps.length === 0) {
        container.innerHTML = `
            <div style="padding:2rem;text-align:center;color:#9ca3af;font-size:0.875rem;">
                暂无监控进程，点击上方「＋ 添加」按钮添加
            </div>
        `;
        return;
    }

    let html = '';
    processMonitorApps.forEach(app => {
        const pmConfig = app.process_monitor || {};
        const useVLModel = pmConfig.use_vlmodel || false;
        const vlModelProvider = pmConfig.vlmodel_provider || '';

        html += `
            <div class="provider-item" style="border-bottom:1px solid #e5e7eb;">
                <div style="display:flex;justify-content:space-between;align-items:flex-start;padding:1rem;">
                    <div style="flex:1;min-width:0;">
                        <div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.5rem;">
                            <span style="font-weight:600;color:#1f2937;font-size:0.95rem;">${escapeHtml(app.display_name || app.name)}</span>
                            ${app.enabled ? '<span class="badge badge-success">启用</span>' : '<span class="badge badge-secondary">禁用</span>'}
                        </div>
                        <div style="font-size:0.8rem;color:#6b7280;margin-bottom:0.3rem;">
                            进程名: <code style="background:#f3f4f6;padding:0.1rem 0.4rem;border-radius:3px;">${escapeHtml(app.process_name)}</code>
                        </div>
                        <div style="font-size:0.8rem;color:#6b7280;margin-bottom:0.5rem;">
                            VLModel: ${useVLModel ? '<span style="color:#059669;">已启用</span>' : '<span style="color:#9ca3af;">未启用</span>'}
                            ${useVLModel && vlModelProvider ? ` (Provider: <code style="background:#f3f4f6;padding:0.1rem 0.4rem;border-radius:3px;">${escapeHtml(vlModelProvider)}</code>)` : ''}
                        </div>
                    </div>
                    <div style="display:flex;gap:0.5rem;flex-shrink:0;">
                        <button class="btn btn-secondary btn-sm" onclick="editProcessMonitor('${escapeHtml(app.name)}')">编辑</button>
                        <button class="btn btn-danger btn-sm" onclick="deleteProcessMonitor('${escapeHtml(app.name)}')">删除</button>
                    </div>
                </div>
            </div>
        `;
    });

    container.innerHTML = html;
}

// 添加进程监控
function addProcessMonitor() {
    showProcessMonitorEditDialog(null);
}

// 编辑进程监控
function editProcessMonitor(appName) {
    const app = processMonitorApps.find(a => a.name === appName);
    if (!app) {
        showToast('应用不存在', 'error');
        return;
    }
    showProcessMonitorEditDialog(app);
}

// 显示编辑对话框
function showProcessMonitorEditDialog(app) {
    const isEdit = app !== null;
    const pmConfig = app?.process_monitor || {};

    const dialog = document.createElement('div');
    dialog.className = 'modal-overlay';
    dialog.style.cssText = 'position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.5);display:flex;align-items:center;justify-content:center;z-index:1000;';
    dialog.innerHTML = `
        <div class="modal" style="background:white;border-radius:12px;width:90%;max-width:500px;max-height:90vh;overflow-y:auto;padding:1.5rem;box-shadow:0 20px 25px -5px rgba(0,0,0,0.1);">
            <h2 style="margin:0 0 1rem 0;font-size:1.25rem;font-weight:600;color:#1f2937;">
                ${isEdit ? '编辑进程监控' : '添加进程监控'}
            </h2>
            
            <div class="form-group" style="margin-bottom:1rem;">
                <label class="form-label">应用名称</label>
                <input type="text" id="pm-name" value="${isEdit ? escapeHtml(app.name) : ''}" placeholder="例如: wechat"
                    style="width:100%;padding:0.6rem 0.8rem;border:1.5px solid #e5e7eb;border-radius:8px;font-size:0.9rem;outline:none;"
                    onfocus="this.style.borderColor='#3b82f6';this.style.boxShadow='0 0 0 3px rgba(59,130,246,0.1)'"
                    onblur="this.style.borderColor='#e5e7eb';this.style.boxShadow='none'">
                <div class="form-hint">唯一标识符，只能包含字母、数字和下划线</div>
            </div>

            <div class="form-group" style="margin-bottom:1rem;">
                <label class="form-label">进程名称</label>
                <input type="text" id="pm-process-name" value="${isEdit ? escapeHtml(app.process_name) : ''}" placeholder="例如: WeChat"
                    style="width:100%;padding:0.6rem 0.8rem;border:1.5px solid #e5e7eb;border-radius:8px;font-size:0.9rem;outline:none;"
                    onfocus="this.style.borderColor='#3b82f6';this.style.boxShadow='0 0 0 3px rgba(59,130,246,0.1)'"
                    onblur="this.style.borderColor='#e5e7eb';this.style.boxShadow='none'">
                <div class="form-hint">用于查找进程的实际名称（使用 pgrep 命令）</div>
            </div>

            <div class="form-group" style="margin-bottom:1rem;">
                <label class="form-label">显示名称</label>
                <input type="text" id="pm-display-name" value="${isEdit ? escapeHtml(app.display_name) : ''}" placeholder="例如: 微信"
                    style="width:100%;padding:0.6rem 0.8rem;border:1.5px solid #e5e7eb;border-radius:8px;font-size:0.9rem;outline:none;"
                    onfocus="this.style.borderColor='#3b82f6';this.style.boxShadow='0 0 0 3px rgba(59,130,246,0.1)'"
                    onblur="this.style.borderColor='#e5e7eb';this.style.boxShadow='none'">
                <div class="form-hint">在界面上显示的友好名称</div>
            </div>

            <div class="form-group" style="margin-bottom:1rem;">
                <div class="toggle-row">
                    <div class="toggle-info">
                        <div class="toggle-label">启用监控</div>
                        <div class="toggle-desc">是否启用该进程的消息监控</div>
                    </div>
                    <label class="toggle">
                        <input type="checkbox" id="pm-enabled" ${isEdit && app.enabled ? 'checked' : ''}>
                        <span class="toggle-slider"></span>
                    </label>
                </div>
            </div>

            <div class="divider"></div>

            <div class="form-group" style="margin-bottom:1rem;">
                <div class="toggle-row">
                    <div class="toggle-info">
                        <div class="toggle-label">启用 VLModel 识别优化</div>
                        <div class="toggle-desc">使用视觉模型识别窗口截图，更准确但需要配置 AI Provider</div>
                    </div>
                    <label class="toggle">
                        <input type="checkbox" id="pm-use-vlmodel" ${pmConfig.use_vlmodel ? 'checked' : ''} onchange="toggleVLModelProvider()">
                        <span class="toggle-slider"></span>
                    </label>
                </div>
            </div>

            <div class="form-group" id="pm-vlmodel-provider-group" style="margin-bottom:1rem;display:${pmConfig.use_vlmodel ? 'block' : 'none'};">
                <label class="form-label">VLModel Provider</label>
                <select id="pm-vlmodel-provider" style="width:100%;padding:0.6rem 0.8rem;border:1.5px solid #e5e7eb;border-radius:8px;font-size:0.9rem;outline:none;background:white;"
                    onfocus="this.style.borderColor='#3b82f6';this.style.boxShadow='0 0 0 3px rgba(59,130,246,0.1)'"
                    onblur="this.style.borderColor='#e5e7eb';this.style.boxShadow='none'">
                    <option value="">-- 选择 Provider --</option>
                    ${aiProviders.map(p => `<option value="${escapeHtml(p.name)}" ${pmConfig.vlmodel_provider === p.name ? 'selected' : ''}>${escapeHtml(p.name)} (${escapeHtml(p.model)})</option>`).join('')}
                </select>
                <div class="form-hint">选择在「模型配置」中已定义的 Provider，需支持图像输入</div>
            </div>

            <div class="form-group" id="pm-vlmodel-prompt-group" style="margin-bottom:1.5rem;display:${pmConfig.use_vlmodel ? 'block' : 'none'};">
                <label class="form-label">VLModel 提示词</label>
                <select id="pm-vlmodel-prompt" style="width:100%;padding:0.6rem 0.8rem;border:1.5px solid #e5e7eb;border-radius:8px;font-size:0.9rem;outline:none;background:white;"
                    onfocus="this.style.borderColor='#3b82f6';this.style.boxShadow='0 0 0 3px rgba(59,130,246,0.1)'"
                    onblur="this.style.borderColor='#e5e7eb';this.style.boxShadow='none'">
                    <option value="">-- 使用默认提示词 --</option>
                    ${availablePrompts.map(p => `<option value="${escapeHtml(p.name)}" ${pmConfig.vlmodel_prompt === p.name ? 'selected' : ''}>${escapeHtml(p.display_name)}</option>`).join('')}
                </select>
                <div class="form-hint">选择在「提示词管理」中已定义的提示词模板，留空则使用默认提示词</div>
            </div>

            <div style="display:flex;gap:0.6rem;justify-content:flex-end;">
                <button class="btn btn-secondary" onclick="closeProcessMonitorDialog()">取消</button>
                <button class="btn btn-primary" onclick="saveProcessMonitor('${isEdit ? escapeHtml(app.name) : ''}')">保存</button>
            </div>
        </div>
    `;

    document.body.appendChild(dialog);
    window.currentProcessMonitorDialog = dialog;

    // 初始化 VLModel Provider 显示状态
    toggleVLModelProvider();
}

// 切换 VLModel Provider 输入框显示
function toggleVLModelProvider() {
    const useVLModel = document.getElementById('pm-use-vlmodel')?.checked;
    const providerGroup = document.getElementById('pm-vlmodel-provider-group');
    const promptGroup = document.getElementById('pm-vlmodel-prompt-group');
    if (providerGroup) {
        providerGroup.style.display = useVLModel ? 'block' : 'none';
    }
    if (promptGroup) {
        promptGroup.style.display = useVLModel ? 'block' : 'none';
    }
}

// 关闭对话框
function closeProcessMonitorDialog() {
    if (window.currentProcessMonitorDialog) {
        window.currentProcessMonitorDialog.remove();
        window.currentProcessMonitorDialog = null;
    }
}

// 保存进程监控配置
async function saveProcessMonitor(originalName) {
    const name = document.getElementById('pm-name').value.trim();
    const processName = document.getElementById('pm-process-name').value.trim();
    const displayName = document.getElementById('pm-display-name').value.trim();
    const enabled = document.getElementById('pm-enabled').checked;
    const useVLModel = document.getElementById('pm-use-vlmodel').checked;
    const vlModelProvider = document.getElementById('pm-vlmodel-provider').value;
    const vlModelPrompt = document.getElementById('pm-vlmodel-prompt').value;

    // 验证
    if (!name) {
        showToast('请输入应用名称', 'error');
        return;
    }
    if (!processName) {
        showToast('请输入进程名称', 'error');
        return;
    }
    if (!displayName) {
        showToast('请输入显示名称', 'error');
        return;
    }
    if (useVLModel && !vlModelProvider) {
        showToast('请选择 VLModel Provider', 'error');
        return;
    }

    // 检查名称冲突
    if (originalName !== name) {
        const exists = processMonitorApps.find(a => a.name === name);
        if (exists) {
            showToast('应用名称已存在', 'error');
            return;
        }
    }

    const appData = {
        name: name,
        process_name: processName,
        display_name: displayName,
        enabled: enabled,
        icon_path: '',
        parse_rules: {
            sender_pattern: '',
            time_pattern: '\\d{1,2}:\\d{2}',
            content_mode: 'after_time',
            dedup_enabled: true
        },
        session_config: {
            source: 'window_title',
            fixed_name: '',
            script: ''
        },
        process_monitor: {
            use_vlmodel: useVLModel,
            vlmodel_provider: vlModelProvider,
            vlmodel_prompt: vlModelPrompt
        }
    };

    try {
        const url = originalName ? `/api/apps/${encodeURIComponent(originalName)}` : '/api/apps';
        const method = originalName ? 'PUT' : 'POST';

        const res = await fetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(appData)
        });

        if (!res.ok) {
            const err = await res.json();
            throw new Error(err.error || '保存失败');
        }

        showToast('保存成功', 'success');
        closeProcessMonitorDialog();
        await loadProcessMonitorConfig();
    } catch (error) {
        console.error('保存进程监控配置失败:', error);
        showToast(error.message || '保存失败', 'error');
    }
}

// 删除进程监控
async function deleteProcessMonitor(appName) {
    if (!confirm(`确定要删除「${appName}」吗？`)) {
        return;
    }

    try {
        const res = await fetch(`/api/apps/${encodeURIComponent(appName)}`, {
            method: 'DELETE'
        });

        if (!res.ok) {
            const err = await res.json();
            throw new Error(err.error || '删除失败');
        }

        showToast('删除成功', 'success');
        await loadProcessMonitorConfig();
    } catch (error) {
        console.error('删除进程监控失败:', error);
        showToast(error.message || '删除失败', 'error');
    }
}

// 页面加载时自动加载
document.addEventListener('DOMContentLoaded', function() {
    // 延迟加载，等待其他页面初始化完成
    setTimeout(() => {
        if (document.getElementById('page-process-monitor')) {
            loadProcessMonitorConfig();
        }
    }, 100);
});

function renderProcessPromptList() {
    const container = document.getElementById('process-prompt-list');
    if (!container) return;

    let html = '';
    PROCESS_PROMPT_NAMES.forEach((meta, index) => {
        const content = processPromptContents[meta.name] || '';
        html += `
        <div class="prompt-item" style="border:1px solid #e5e7eb;border-radius:10px;margin-bottom:0.8rem;overflow:hidden;">
            <div class="prompt-item-header" style="display:flex;justify-content:space-between;align-items:center;padding:0.8rem 1rem;cursor:pointer;background:#fafafa;" onclick="toggleProcessPrompt('${meta.name}')">
                <div>
                    <span style="font-weight:600;font-size:0.9rem;color:#1f2937;">${escapeHtml(meta.displayName)}</span>
                    <span style="display:block;font-size:0.75rem;color:#6b7280;margin-top:0.2rem;">${escapeHtml(meta.description)}</span>
                </div>
                <span id="process-prompt-arrow-${meta.name}" style="font-size:0.75rem;color:#9ca3af;transition:transform 0.2s;">▶</span>
            </div>
            <div id="process-prompt-body-${meta.name}" style="display:none;padding:0.8rem 1rem;border-top:1px solid #e5e7eb;">
                <textarea id="process-prompt-textarea-${meta.name}" rows="10"
                    style="width:100%;padding:0.75rem 1rem;border:1.5px solid #e5e7eb;border-radius:9px;font-size:0.85rem;color:#1f2937;background:#fafafa;outline:none;resize:vertical;font-family:'Menlo','Monaco','Courier New',monospace;line-height:1.6;transition:border-color 0.2s,box-shadow 0.2s;"
                    onfocus="this.style.borderColor='#6366f1';this.style.boxShadow='0 0 0 3px rgba(99,102,241,0.12)';this.style.background='white'"
                    onblur="this.style.borderColor='#e5e7eb';this.style.boxShadow='none';this.style.background='#fafafa'">${escapeHtml(content)}</textarea>
                <div style="display:flex;justify-content:flex-end;margin-top:0.6rem;gap:0.5rem;">
                    <button class="btn btn-primary btn-sm" onclick="saveProcessPrompt('${meta.name}')">💾 保存</button>
                </div>
            </div>
        </div>`;
    });

    container.innerHTML = html;
}

function toggleProcessPrompt(name) {
    const body = document.getElementById('process-prompt-body-' + name);
    const arrow = document.getElementById('process-prompt-arrow-' + name);
    if (!body) return;
    const isHidden = body.style.display === 'none';
    body.style.display = isHidden ? 'block' : 'none';
    if (arrow) arrow.textContent = isHidden ? '▼' : '▶';
}

async function saveProcessPrompt(name) {
    const textarea = document.getElementById('process-prompt-textarea-' + name);
    if (!textarea) return;
    const content = textarea.value;
    try {
        const res = await fetch('/api/prompts/' + name, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ content }),
        });
        const data = await res.json();
        if (res.ok) {
            processPromptContents[name] = content;
            showToast('✅ 提示词保存成功', 'success');
        } else {
            showToast('❌ 保存失败：' + data.error, 'error');
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
    }
}