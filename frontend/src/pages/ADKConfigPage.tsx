import { useState, useEffect } from 'react'
import { Brain, Save, Plus, Trash2, AlertCircle, RefreshCw } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { ADKConfig, ADKAgent } from '../types'

const LOG_LEVELS = ['debug', 'info', 'warn', 'error']
const MEMORY_TYPES = ['sqlite', 'memory']

export default function ADKConfigPage() {
  const [config, setConfig] = useState<ADKConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadConfig()
  }, [])

  const loadConfig = async () => {
    try {
      const data = await wailsAPI.getADKConfig()
      setConfig(data)
    } catch (error) {
      console.error('Failed to load ADK config:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    try {
      await wailsAPI.saveADKConfig(config as any)
      setMessage({ type: 'success', text: '保存成功' })
    } catch (error) {
      setMessage({ type: 'error', text: '保存失败：' + String(error) })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const handleInitDefault = () => {
    setConfig({
      system_name: 'luffot-adk-system',
      log_level: 'info',
      agents: [],
      skills_dir: '~/.luffot/adk/skills',
      skills_autoload: true,
      memory_type: 'sqlite',
      memory_max_history: 100,
    })
  }

  const addAgent = () => {
    if (!config) return
    const newAgent: ADKAgent = {
      name: `agent_${(config.agents?.length || 0) + 1}`,
      role: '',
      model_provider: '',
      system_prompt: '',
    }
    setConfig({
      ...config,
      agents: [...(config.agents || []), newAgent],
    })
  }

  const removeAgent = (index: number) => {
    if (!config) return
    const newAgents = [...(config.agents || [])]
    newAgents.splice(index, 1)
    setConfig({ ...config, agents: newAgents })
  }

  const updateAgent = (index: number, field: keyof ADKAgent, value: string) => {
    if (!config) return
    const newAgents = [...(config.agents || [])]
    newAgents[index] = { ...newAgents[index], [field]: value }
    setConfig({ ...config, agents: newAgents })
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600" />
      </div>
    )
  }

  // 如果配置不存在，显示初始化提示
  if (!config || (config as any).enabled === false) {
    return (
      <div className="space-y-6 animate-fade-in">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">ADK 配置</h1>
          <p className="text-gray-500 mt-1">配置多 Agent 系统（Agent Development Kit），支持智能任务规划和执行</p>
        </div>

        <div className="card">
          <div className="card-body text-center py-12">
            <Brain className="w-16 h-16 text-gray-300 mx-auto mb-4" />
            <h3 className="text-lg font-medium text-gray-900 mb-2">ADK 配置不存在</h3>
            <p className="text-gray-500 mb-6">请先初始化默认配置</p>
            <button onClick={handleInitDefault} className="btn-primary">
              <RefreshCw className="w-4 h-4" />
              初始化默认配置
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">ADK 配置</h1>
        <p className="text-gray-500 mt-1">配置多 Agent 系统（Agent Development Kit），支持智能任务规划和执行</p>
      </div>

      {/* Message */}
      {message && (
        <div
          className={`p-4 rounded-lg flex items-center gap-2 ${
            message.type === 'success'
              ? 'bg-green-50 text-green-700 border border-green-200'
              : 'bg-red-50 text-red-700 border border-red-200'
          }`}
        >
          <AlertCircle className="w-5 h-5" />
          {message.text}
        </div>
      )}

      {/* System Config */}
      <div className="card">
        <div className="card-header">
          <Brain className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">系统配置</h3>
        </div>
        <div className="card-body">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="form-group mb-0">
              <label className="form-label">系统名称</label>
              <input
                type="text"
                value={config.system_name || ''}
                onChange={(e) => setConfig({ ...config, system_name: e.target.value })}
                placeholder="luffot-adk-system"
                className="form-input"
              />
            </div>
            <div className="form-group mb-0">
              <label className="form-label">日志级别</label>
              <select
                value={config.log_level || 'info'}
                onChange={(e) => setConfig({ ...config, log_level: e.target.value })}
                className="form-select"
              >
                {LOG_LEVELS.map((level) => (
                  <option key={level} value={level}>
                    {level}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>
      </div>

      {/* Agent Team */}
      <div className="card">
        <div className="card-header flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Brain className="w-5 h-5 text-primary-600" />
            <h3 className="text-lg font-semibold text-gray-900">Agent 团队</h3>
          </div>
          <button onClick={addAgent} className="btn-primary btn-sm">
            <Plus className="w-4 h-4" />
            添加 Agent
          </button>
        </div>
        <div className="divide-y divide-gray-100">
          {(config.agents || []).length === 0 ? (
            <div className="p-8 text-center text-gray-500">
              暂无 Agent，点击「添加 Agent」创建
            </div>
          ) : (
            (config.agents || []).map((agent, index) => (
              <div key={index} className="p-6">
                <div className="flex items-center justify-between mb-4">
                  <h4 className="font-medium text-gray-900">Agent #{index + 1}</h4>
                  <button
                    onClick={() => removeAgent(index)}
                    className="text-red-500 hover:text-red-700"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="form-group mb-0">
                    <label className="form-label">名称</label>
                    <input
                      type="text"
                      value={agent.name}
                      onChange={(e) => updateAgent(index, 'name', e.target.value)}
                      placeholder="agent_name"
                      className="form-input"
                    />
                  </div>
                  <div className="form-group mb-0">
                    <label className="form-label">角色</label>
                    <input
                      type="text"
                      value={agent.role || ''}
                      onChange={(e) => updateAgent(index, 'role', e.target.value)}
                      placeholder="例如：助手、分析师"
                      className="form-input"
                    />
                  </div>
                  <div className="form-group mb-0">
                    <label className="form-label">模型 Provider</label>
                    <input
                      type="text"
                      value={agent.model_provider || ''}
                      onChange={(e) => updateAgent(index, 'model_provider', e.target.value)}
                      placeholder="例如：default"
                      className="form-input"
                    />
                  </div>
                  <div className="form-group mb-0 md:col-span-2">
                    <label className="form-label">系统提示词</label>
                    <textarea
                      value={agent.system_prompt || ''}
                      onChange={(e) => updateAgent(index, 'system_prompt', e.target.value)}
                      rows={3}
                      placeholder="输入系统提示词..."
                      className="form-textarea"
                    />
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Skills Config */}
      <div className="card">
        <div className="card-header">
          <Brain className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">技能配置</h3>
        </div>
        <div className="card-body space-y-4">
          <div className="form-group mb-0">
            <label className="form-label">技能目录</label>
            <input
              type="text"
              value={config.skills_dir || ''}
              onChange={(e) => setConfig({ ...config, skills_dir: e.target.value })}
              placeholder="~/.luffot/adk/skills"
              className="form-input"
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <label className="form-label mb-0">自动加载技能</label>
              <p className="text-sm text-gray-500">启动时自动从技能目录加载所有技能</p>
            </div>
            <button
              onClick={() => setConfig({ ...config, skills_autoload: !config.skills_autoload })}
              className={`toggle ${config.skills_autoload ? 'toggle-enabled' : 'toggle-disabled'}`}
            >
              <span
                className={`toggle-thumb ${
                  config.skills_autoload ? 'toggle-thumb-enabled' : 'toggle-thumb-disabled'
                }`}
              />
            </button>
          </div>
        </div>
      </div>

      {/* Memory Config */}
      <div className="card">
        <div className="card-header">
          <Brain className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">内存配置</h3>
        </div>
        <div className="card-body">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="form-group mb-0">
              <label className="form-label">存储类型</label>
              <select
                value={config.memory_type || 'sqlite'}
                onChange={(e) => setConfig({ ...config, memory_type: e.target.value })}
                className="form-select"
              >
                {MEMORY_TYPES.map((type) => (
                  <option key={type} value={type}>
                    {type}
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group mb-0">
              <label className="form-label">最大历史记录</label>
              <input
                type="number"
                value={config.memory_max_history || 100}
                onChange={(e) => setConfig({ ...config, memory_max_history: parseInt(e.target.value) || 100 })}
                min={1}
                className="form-input"
              />
            </div>
          </div>
        </div>
      </div>

      {/* Save Button */}
      <div className="flex justify-end gap-3">
        <button onClick={handleInitDefault} className="btn-secondary">
          <RefreshCw className="w-4 h-4" />
          重置默认配置
        </button>
        <button onClick={handleSave} disabled={saving} className="btn-primary">
          <Save className="w-4 h-4" />
          {saving ? '保存中...' : '保存 ADK 配置'}
        </button>
      </div>
    </div>
  )
}
