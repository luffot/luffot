// ADK 配置页面逻辑

let adkConfig = null;

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', function() {
    // 如果当前是 ADK 配置页面，加载配置
    if (document.getElementById('page-adk').classList.contains('active')) {
        loadADKConfig();
    }
});

// 加载 ADK 配置
async function loadADKConfig() {
    try {
        const response = await fetch('/api/adk/config');
        const data = await response.json();
        
        if (data.enabled === false) {
            // 配置不存在，显示初始化按钮
            document.getElementById('adk-status-badge').textContent = '未初始化';
            document.getElementById('adk-status-badge').className = 'card-badge warn';
            showToast('ADK 配置未初始化，请点击"初始化默认配置"按钮');
            return;
        }
        
        adkConfig = data;
        
        // 更新状态徽章
        document.getElementById('adk-status-badge').textContent = '已加载';
        document.getElementById('adk-status-badge').className = 'card-badge success';
        
        // 填充系统配置
        if (data.system) {
            document.getElementById('adk-system-name').value = data.system.name || '';
            document.getElementById('adk-log-level').value = data.system.log_level || 'info';
        }
        
        // 注意：ADK 使用全局模型配置，不再单独配置 LLM
        // LLM 配置请在「模型配置」页面进行设置
        
        // 填充 Agent 列表
        if (data.agents && Array.isArray(data.agents)) {
            renderADKAgents(data.agents);
        }
        
        // 填充技能配置
        if (data.skills) {
            document.getElementById('adk-skills-dir').value = data.skills.directory || '';
            document.getElementById('adk-skills-autoload').checked = data.skills.auto_load !== false;
        }
        
        // 填充内存配置
        if (data.memory) {
            document.getElementById('adk-memory-type').value = data.memory.type || 'sqlite';
            document.getElementById('adk-memory-maxhistory').value = data.memory.max_history || 100;
        }
        
    } catch (error) {
        console.error('加载 ADK 配置失败:', error);
        showToast('加载 ADK 配置失败: ' + error.message);
        document.getElementById('adk-status-badge').textContent = '加载失败';
        document.getElementById('adk-status-badge').className = 'card-badge error';
    }
}

// 渲染 Agent 列表
function renderADKAgents(agents) {
    const container = document.getElementById('adk-agents-list');
    container.innerHTML = '';
    
    agents.forEach((agent, index) => {
        const agentDiv = document.createElement('div');
        agentDiv.className = 'provider-item';
        agentDiv.innerHTML = `
            <div class="provider-header">
                <div class="provider-title">
                    <span class="provider-name">${agent.name || '未命名'}</span>
                    <span class="provider-type">${agent.type || 'unknown'}</span>
                </div>
                <button class="btn btn-danger btn-sm" onclick="removeADKAgent(${index})">删除</button>
            </div>
            <div class="provider-body">
                <div class="form-group">
                    <label class="form-label">名称</label>
                    <input type="text" class="adk-agent-name" data-index="${index}" value="${agent.name || ''}" placeholder="Agent 名称">
                </div>
                <div class="form-group">
                    <label class="form-label">类型</label>
                    <select class="adk-agent-type form-select" data-index="${index}">
                        <option value="coordinator" ${agent.type === 'coordinator' ? 'selected' : ''}>coordinator - 协调Agent</option>
                        <option value="planner" ${agent.type === 'planner' ? 'selected' : ''}>planner - 规划Agent</option>
                        <option value="executor" ${agent.type === 'executor' ? 'selected' : ''}>executor - 执行Agent</option>
                        <option value="reviewer" ${agent.type === 'reviewer' ? 'selected' : ''}>reviewer - 审查Agent</option>
                        <option value="specialist" ${agent.type === 'specialist' ? 'selected' : ''}>specialist - 专家Agent</option>
                    </select>
                </div>
                <div class="form-group">
                    <label class="form-label">描述</label>
                    <input type="text" class="adk-agent-description" data-index="${index}" value="${agent.description || ''}" placeholder="Agent 描述">
                </div>
                <div class="form-group">
                    <label class="form-label">指令 (Instruction)</label>
                    <textarea class="adk-agent-instruction" data-index="${index}" rows="3" placeholder="Agent 的系统指令">${agent.instruction || ''}</textarea>
                </div>
                <div class="form-group">
                    <label class="form-label">技能 (逗号分隔)</label>
                    <input type="text" class="adk-agent-skills" data-index="${index}" value="${(agent.skills || []).join(', ')}" placeholder="skill1, skill2, skill3">
                </div>
            </div>
        `;
        container.appendChild(agentDiv);
    });
}

// 添加 Agent
function addADKAgent() {
    const agents = adkConfig && adkConfig.agents ? adkConfig.agents : [];
    agents.push({
        name: 'new-agent',
        type: 'executor',
        description: '',
        instruction: '',
        skills: []
    });
    
    if (!adkConfig) {
        adkConfig = { agents: agents };
    } else {
        adkConfig.agents = agents;
    }
    
    renderADKAgents(agents);
}

// 删除 Agent
function removeADKAgent(index) {
    if (!adkConfig || !adkConfig.agents) return;
    
    adkConfig.agents.splice(index, 1);
    renderADKAgents(adkConfig.agents);
}

// 保存 ADK 配置
async function saveADKConfig() {
    try {
        // 收集表单数据
        const config = {
            system: {
                name: document.getElementById('adk-system-name').value || 'luffot-adk-system',
                version: '1.0.0',
                log_level: document.getElementById('adk-log-level').value || 'info'
            },
            // 注意：ADK 使用全局模型配置，不再单独配置 LLM
            agents: [],
            skills: {
                directory: document.getElementById('adk-skills-dir').value || '~/.luffot/adk/skills',
                auto_load: document.getElementById('adk-skills-autoload').checked,
                installed: []
            },
            memory: {
                type: document.getElementById('adk-memory-type').value || 'sqlite',
                path: '~/.luffot/adk/data/memory.db',
                max_history: parseInt(document.getElementById('adk-memory-maxhistory').value) || 100
            },
            events: {
                enabled: true,
                buffer_size: 1000
            }
        };
        
        // 收集 Agent 配置
        const agentItems = document.querySelectorAll('.provider-item');
        agentItems.forEach((item, index) => {
            const name = item.querySelector('.adk-agent-name').value;
            const type = item.querySelector('.adk-agent-type').value;
            const description = item.querySelector('.adk-agent-description').value;
            const instruction = item.querySelector('.adk-agent-instruction').value;
            const skillsStr = item.querySelector('.adk-agent-skills').value;
            const skills = skillsStr ? skillsStr.split(',').map(s => s.trim()).filter(s => s) : [];
            
            config.agents.push({
                name: name || `agent-${index}`,
                type: type || 'executor',
                description: description || '',
                instruction: instruction || '',
                skills: skills
            });
        });
        
        // 发送保存请求
        const response = await fetch('/api/adk/config', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        const result = await response.json();
        
        if (response.ok) {
            showToast('ADK 配置保存成功');
            document.getElementById('adk-status-badge').textContent = '已保存';
            document.getElementById('adk-status-badge').className = 'card-badge success';
        } else {
            throw new Error(result.error || '保存失败');
        }
        
    } catch (error) {
        console.error('保存 ADK 配置失败:', error);
        showToast('保存 ADK 配置失败: ' + error.message);
    }
}

// 初始化默认 ADK 配置
async function initADKConfig() {
    const defaultConfig = {
        system: {
            name: 'luffot-adk-system',
            version: '1.0.0',
            log_level: 'info'
        },
        // 注意：ADK 使用全局模型配置，不再单独配置 LLM
        agents: [
            {
                name: 'coordinator',
                type: 'coordinator',
                description: '负责任务协调和Agent调度',
                instruction: '',
                skills: []
            },
            {
                name: 'planner',
                type: 'planner',
                description: '负责任务规划和策略制定',
                instruction: '你是一个智能规划Agent。\n你的职责:\n1. 分析复杂任务，理解目标和约束\n2. 将任务分解为可执行的步骤\n3. 为每个步骤分配合适的Agent和技能\n4. 识别步骤间的依赖关系\n5. 生成详细的执行计划',
                skills: []
            },
            {
                name: 'executor',
                type: 'executor',
                description: '负责任务执行',
                instruction: '',
                skills: ['example-skill']
            },
            {
                name: 'reviewer',
                type: 'reviewer',
                description: '负责结果审查和质量控制',
                instruction: '',
                skills: []
            }
        ],
        skills: {
            directory: '~/.luffot/adk/skills',
            auto_load: true,
            installed: [
                { name: 'example-skill', version: '1.0.0', source: 'builtin' }
            ]
        },
        memory: {
            type: 'sqlite',
            path: '~/.luffot/adk/data/memory.db',
            max_history: 100
        },
        events: {
            enabled: true,
            buffer_size: 1000
        }
    };
    
    // 填充表单
    document.getElementById('adk-system-name').value = defaultConfig.system.name;
    document.getElementById('adk-log-level').value = defaultConfig.system.log_level;
    
    // 注意：ADK 使用全局模型配置，不再单独配置 LLM
    
    renderADKAgents(defaultConfig.agents);
    
    document.getElementById('adk-skills-dir').value = defaultConfig.skills.directory;
    document.getElementById('adk-skills-autoload').checked = defaultConfig.skills.auto_load;
    
    document.getElementById('adk-memory-type').value = defaultConfig.memory.type;
    document.getElementById('adk-memory-maxhistory').value = defaultConfig.memory.max_history;
    
    adkConfig = defaultConfig;
    
    showToast('已加载默认配置，请点击"保存 ADK 配置"按钮保存');
    document.getElementById('adk-status-badge').textContent = '待保存';
    document.getElementById('adk-status-badge').className = 'card-badge warn';
}

// 页面切换时重新加载
function onPageADKShow() {
    loadADKConfig();
}
