import { useState, useEffect } from 'react'
import { Plus, Trash2, Save, Bot, AlertCircle } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { AIConfig, AIProviderConfig } from '../types'

const PROVIDER_OPTIONS = [
  { value: 'openai', label: 'OpenAI / 兼容接口' },
  { value: 'bailian', label: '阿里云百炼' },
  { value: 'dashscope', label: '阿里云 DashScope' },
]

export default function ModelConfigPage() {
  const [config, setConfig] = useState<AIConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadConfig()
  }, [])

  const loadConfig = async () => {
    try {
      const data = await wailsAPI.getAIConfig()
      setConfig(data)
    } catch (error) {
      console.error('Failed to load AI config:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    try {
      await wailsAPI.saveAIConfig(config)
      setMessage({ type: 'success', text: '保存成功，配置已生效' })
    } catch (error) {
      setMessage({ type: 'error', text: '保存失败：' + String(error) })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const addProvider = () => {
    if (!config) return
    const newProvider: AIProviderConfig = {
      name: `provider_${config.providers.length + 1}`,
      provider: 'openai',
      api_key: '',
      model: 'gpt-3.5-turbo',
      base_url: '',
      timeout_seconds: 0,
    }
    setConfig({
      ...config,
      providers: [...config.providers, newProvider],
    })
  }

  const removeProvider = (index: number) => {
    if (!config) return
    const newProviders = [...config.providers]
    newProviders.splice(index, 1)
    setConfig({ ...config, providers: newProviders })
  }

  const updateProvider = (index: number, field: keyof AIProviderConfig, value: string | number) => {
    if (!config) return
    const newProviders = [...config.providers]
    newProviders[index] = { ...newProviders[index], [field]: value }
    setConfig({ ...config, providers: newProviders })
  }

  if (loading || !config) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600" />
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">模型配置</h1>
        <p className="text-gray-500 mt-1">管理所有 AI Provider，不同功能可指定不同模型</p>
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

      {/* Global Settings */}
      <div className="card">
        <div className="card-header">
          <Bot className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">全局设置</h3>
          <span
            className={`ml-auto badge ${config.enabled ? 'badge-success' : 'badge-danger'}`}
          >
            {config.enabled ? '已启用' : '已禁用'}
          </span>
        </div>
        <div className="card-body space-y-6">
          {/* Enable Toggle */}
          <div className="flex items-center justify-between">
            <div>
              <label className="form-label mb-0">启用 AI 功能</label>
              <p className="text-sm text-gray-500">开启后可与桌宠小钉进行 AI 对话</p>
            </div>
            <button
              onClick={() => setConfig({ ...config, enabled: !config.enabled })}
              className={`toggle ${config.enabled ? 'toggle-enabled' : 'toggle-disabled'}`}
            >
              <span
                className={`toggle-thumb ${
                  config.enabled ? 'toggle-thumb-enabled' : 'toggle-thumb-disabled'
                }`}
              />
            </button>
          </div>

          <div className="border-t border-gray-100" />

          {/* Default Provider & Context Rounds */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="form-group mb-0">
              <label className="form-label">默认 Provider</label>
              <input
                type="text"
                value={config.default_provider}
                onChange={(e) =>
                  setConfig({ ...config, default_provider: e.target.value })
                }
                placeholder="default"
                className="form-input"
              />
              <p className="text-xs text-gray-500 mt-1">未指定 provider 时使用此名称</p>
            </div>

            <div className="form-group mb-0">
              <label className="form-label">最大上下文轮数</label>
              <input
                type="number"
                value={config.max_context_rounds}
                onChange={(e) =>
                  setConfig({ ...config, max_context_rounds: parseInt(e.target.value) || 10 })
                }
                min={1}
                max={50}
                className="form-input"
              />
              <p className="text-xs text-gray-500 mt-1">AI 记忆的最近对话轮数，建议 5~20</p>
            </div>
          </div>
        </div>
        <div className="card-footer">
          <span
            className={`status-dot ${config.enabled ? 'status-dot-success' : 'status-dot-gray'}`}
          />
          <span>{config.enabled ? 'AI 功能已启用' : 'AI 功能已禁用'}</span>
        </div>
      </div>

      {/* Provider List */}
      <div className="card">
        <div className="card-header flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Bot className="w-5 h-5 text-primary-600" />
            <h3 className="text-lg font-semibold text-gray-900">Provider 列表</h3>
          </div>
          <button onClick={addProvider} className="btn-primary btn-sm">
            <Plus className="w-4 h-4" />
            添加
          </button>
        </div>
        <div className="divide-y divide-gray-100">
          {config.providers.length === 0 ? (
            <div className="p-8 text-center text-gray-500">
              暂无 Provider，点击「添加」创建
            </div>
          ) : (
            config.providers.map((provider, index) => (
              <div key={index} className="p-6">
                <div className="flex items-center justify-between mb-4">
                  <h4 className="font-medium text-gray-900">Provider #{index + 1}</h4>
                  <button
                    onClick={() => removeProvider(index)}
                    className="text-red-500 hover:text-red-700 transition-colors"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="form-group mb-0">
                    <label className="form-label">名称</label>
                    <input
                      type="text"
                      value={provider.name}
                      onChange={(e) => updateProvider(index, 'name', e.target.value)}
                      placeholder="例如：default"
                      className="form-input"
                    />
                  </div>
                  <div className="form-group mb-0">
                    <label className="form-label">服务商类型</label>
                    <select
                      value={provider.provider}
                      onChange={(e) => updateProvider(index, 'provider', e.target.value)}
                      className="form-select"
                    >
                      {PROVIDER_OPTIONS.map((opt) => (
                        <option key={opt.value} value={opt.value}>
                          {opt.label}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="form-group mb-0">
                    <label className="form-label">API Key</label>
                    <input
                      type="password"
                      value={provider.api_key}
                      onChange={(e) => updateProvider(index, 'api_key', e.target.value)}
                      placeholder="sk-..."
                      className="form-input"
                    />
                  </div>
                  <div className="form-group mb-0">
                    <label className="form-label">模型</label>
                    <input
                      type="text"
                      value={provider.model}
                      onChange={(e) => updateProvider(index, 'model', e.target.value)}
                      placeholder="例如：gpt-4o, qwen-plus"
                      className="form-input"
                    />
                  </div>
                  <div className="form-group mb-0 md:col-span-2">
                    <label className="form-label">Base URL（可选）</label>
                    <input
                      type="text"
                      value={provider.base_url || ''}
                      onChange={(e) => updateProvider(index, 'base_url', e.target.value)}
                      placeholder="留空使用默认值"
                      className="form-input"
                    />
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Save Button */}
      <div className="flex justify-end">
        <button
          onClick={handleSave}
          disabled={saving}
          className="btn-primary"
        >
          <Save className="w-4 h-4" />
          {saving ? '保存中...' : '保存配置'}
        </button>
      </div>
    </div>
  )
}
