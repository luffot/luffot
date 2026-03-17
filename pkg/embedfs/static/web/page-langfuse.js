
// ── Langfuse 配置页面 ──

let currentLangfuseConfig = {};

// 加载 Langfuse 配置
async function loadLangfuseConfig() {
    try {
        const response = await fetch('/api/langfuse-config');
        if (!response.ok) {
            throw new Error('获取配置失败');
        }
        const config = await response.json();
        currentLangfuseConfig = config;
        
        // 填充表单
        document.getElementById('langfuse-enabled').checked = config.enabled || false;
        document.getElementById('langfuse-public-key').value = config.public_key || '';
        document.getElementById('langfuse-secret-key').value = config.secret_key || '';
        document.getElementById('langfuse-base-url').value = config.base_url || '';
        document.getElementById('langfuse-async-enabled').checked = config.async_enabled !== false;
        document.getElementById('langfuse-batch-size').value = config.batch_size || 100;
        document.getElementById('langfuse-flush-interval').value = config.flush_interval || 5;
        
        // 更新状态标签
        updateLangfuseStatusBadge(config.enabled);
    } catch (error) {
        console.error('加载 Langfuse 配置失败:', error);
        showToast('加载配置失败: ' + error.message, 'error');
        updateLangfuseStatusBadge(false);
    }
}

// 更新状态标签
function updateLangfuseStatusBadge(enabled) {
    const badge = document.getElementById('langfuse-status-badge');
    if (enabled) {
        badge.textContent = '已启用';
        badge.className = 'card-badge success';
    } else {
        badge.textContent = '未启用';
        badge.className = 'card-badge';
    }
}

// 保存 Langfuse 配置
async function saveLangfuseConfig() {
    const config = {
        enabled: document.getElementById('langfuse-enabled').checked,
        public_key: document.getElementById('langfuse-public-key').value.trim(),
        secret_key: document.getElementById('langfuse-secret-key').value.trim(),
        base_url: document.getElementById('langfuse-base-url').value.trim(),
        async_enabled: document.getElementById('langfuse-async-enabled').checked,
        batch_size: parseInt(document.getElementById('langfuse-batch-size').value) || 100,
        flush_interval: parseInt(document.getElementById('langfuse-flush-interval').value) || 5
    };
    
    // 如果启用但未填写密钥，给出警告
    if (config.enabled && (!config.public_key || !config.secret_key)) {
        showToast('启用追踪需要填写 Public Key 和 Secret Key', 'warn');
        return;
    }
    
    try {
        const response = await fetch('/api/langfuse-config', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '保存失败');
        }
        
        const result = await response.json();
        showToast(result.message || '保存成功', 'success');
        currentLangfuseConfig = config;
        updateLangfuseStatusBadge(config.enabled);
    } catch (error) {
        console.error('保存 Langfuse 配置失败:', error);
        showToast('保存失败: ' + error.message, 'error');
    }
}

// 页面加载时自动加载配置
document.addEventListener('DOMContentLoaded', function() {
    // 如果当前页面是 Langfuse 配置页，加载配置
    if (document.getElementById('page-langfuse')?.classList.contains('active')) {
        loadLangfuseConfig();
    }
});
