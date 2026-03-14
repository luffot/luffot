// ── 提示词管理 ──

let promptsLoaded = false;
const promptOriginalContents = {};

async function loadPrompts() {
    if (promptsLoaded) return;
    try {
        const res = await fetch('/api/prompts');
        const data = await res.json();

        const dirBadge = document.getElementById('prompt-dir-badge');
        if (data.dir) {
            dirBadge.textContent = data.dir;
            dirBadge.title = '提示词文件存储目录：' + data.dir;
        }

        const container = document.getElementById('prompt-list');
        container.innerHTML = '';
        (data.prompts || []).forEach((p, index) => {
            promptOriginalContents[p.name] = p.content;
            container.appendChild(buildPromptItem(p, index === 0));
        });
        promptsLoaded = true;
    } catch (e) {
        showToast('加载 Prompt 失败：' + e.message, 'error');
    }
}

function buildPromptItem(p, expanded) {
    const item = document.createElement('div');
    item.className = 'prompt-item' + (expanded ? ' expanded' : '');
    item.dataset.name = p.name;
    item.innerHTML = `
        <div class="prompt-item-header" onclick="togglePromptItem(this.parentElement)">
            <div class="prompt-item-meta">
                <span class="prompt-item-title">${escapeHtml(p.display_name)}</span>
                <span class="prompt-item-desc">${escapeHtml(p.description)}</span>
            </div>
            <span class="prompt-item-filename">${escapeHtml(p.name)}.md</span>
            <span class="prompt-collapse-icon">▼</span>
        </div>
        <div class="prompt-item-body">
            <textarea class="prompt-textarea" id="prompt-textarea-${escapeHtml(p.name)}" rows="12">${escapeHtml(p.content)}</textarea>
            <div class="prompt-item-actions">
                <button class="btn btn-primary btn-sm" onclick="savePrompt('${escapeHtml(p.name)}')">💾 保存</button>
                <button class="btn btn-secondary btn-sm" onclick="resetPrompt('${escapeHtml(p.name)}')">↩ 恢复</button>
                <span class="prompt-save-hint">修改后立即生效，无需重启</span>
            </div>
        </div>
    `;
    return item;
}

function togglePromptItem(item) {
    item.classList.toggle('expanded');
}

async function savePrompt(name) {
    const textarea = document.getElementById('prompt-textarea-' + name);
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
            promptOriginalContents[name] = content;
            showToast('✅ ' + data.message, 'success');
        } else {
            showToast('❌ 保存失败：' + data.error, 'error');
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
    }
}

async function resetPrompt(name) {
    if (!confirm('确定要恢复"' + name + '"到上次保存的内容吗？')) return;
    const textarea = document.getElementById('prompt-textarea-' + name);
    if (textarea && promptOriginalContents[name] !== undefined) {
        textarea.value = promptOriginalContents[name];
        showToast('已恢复到上次保存的内容', 'success');
    }
}
