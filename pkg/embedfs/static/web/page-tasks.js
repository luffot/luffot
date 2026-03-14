// ── 定时任务 ──

let tasksLoaded = false;

async function loadTasks() {
    tasksLoaded = false;
    const container = document.getElementById('task-list');
    container.innerHTML = '<div style="padding:1.5rem;text-align:center;color:#9ca3af;font-size:0.875rem;">加载中...</div>';
    try {
        const res = await fetch('/api/tasks');
        const data = await res.json();
        const tasks = data.tasks || [];
        if (tasks.length === 0) {
            container.innerHTML = '<div style="padding:2rem;text-align:center;color:#9ca3af;font-size:0.875rem;">' +
                (data.message || '暂无已注册的定时任务') + '</div>';
            return;
        }
        container.innerHTML = tasks.map(t => buildTaskRow(t)).join('');
        tasksLoaded = true;
    } catch (e) {
        container.innerHTML = '<div style="padding:1.5rem;color:#ef4444;font-size:0.875rem;">加载失败：' + escapeHtml(e.message) + '</div>';
    }
}

function buildTaskRow(task) {
    const statusMap = {
        idle:    { cls: 'ok',   label: '空闲' },
        running: { cls: 'warn', label: '运行中' },
        success: { cls: 'ok',   label: '上次成功' },
        failed:  { cls: 'err',  label: '上次失败' },
    };
    const st = statusMap[task.status] || { cls: 'warn', label: task.status };
    const lastRun = task.last_run_at ? new Date(task.last_run_at).toLocaleString('zh-CN') : '从未执行';
    const nextRun = task.next_run_at ? new Date(task.next_run_at).toLocaleString('zh-CN') : '-';
    const errorHtml = task.last_error
        ? `<div style="margin-top:0.4rem;font-size:0.72rem;color:#ef4444;background:#fef2f2;padding:0.4rem 0.7rem;border-radius:6px;">❌ ${escapeHtml(task.last_error)}</div>`
        : '';
    return `
    <div class="task-row">
        <div class="task-row-left">
            <div class="task-name">${escapeHtml(task.name)}</div>
            <div class="task-desc">${escapeHtml(task.description || '')}</div>
            <div class="task-meta">
                <span class="task-cron">⏰ ${escapeHtml(task.cron)}</span>
                <span class="task-type">${escapeHtml(task.type)}</span>
                <span>上次：${lastRun}</span>
                <span>下次：${nextRun}</span>
            </div>
            ${errorHtml}
        </div>
        <div class="task-row-right">
            <span class="task-status-badge ${st.cls}">${st.label}</span>
            <button class="btn btn-secondary btn-sm" onclick="triggerTask('${escapeHtml(task.name)}', this)" ${task.status === 'running' ? 'disabled' : ''}>▶ 立即执行</button>
        </div>
    </div>`;
}

async function triggerTask(name, btn) {
    btn.disabled = true;
    btn.textContent = '执行中...';
    try {
        const res = await fetch('/api/tasks/' + name + '/run', { method: 'POST' });
        const data = await res.json();
        if (res.ok) {
            showToast('✅ ' + data.message, 'success');
            setTimeout(loadTasks, 1500);
        } else {
            showToast('❌ ' + data.error, 'error');
            btn.disabled = false;
            btn.textContent = '▶ 立即执行';
        }
    } catch (e) {
        showToast('❌ 网络错误：' + e.message, 'error');
        btn.disabled = false;
        btn.textContent = '▶ 立即执行';
    }
}
