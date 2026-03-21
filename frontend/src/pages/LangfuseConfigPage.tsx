import { useState, useEffect } from 'react'
import { BarChart3, Save, ExternalLink, AlertCircle } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { LangfuseConfig } from '../types'

export default function LangfuseConfigPage() {
  const [config, setConfig] = useState<LangfuseConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadConfig()
  }, [])

  const loadConfig = async () => {
    try {
      const data = await wailsAPI.getLangfuseConfig()
      setConfig(data)
    } catch (error) {
      console.error('Failed to load Langfuse config:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    try {
      await wailsAPI.saveLangfuseConfig(config)
      setMessage({ type: 'success', text: '保存成功，重启后生效' })
    } catch (error) {
      setMessage({ type: 'error', text: '保存失败：' + String(error) })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
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
        <h1 className="text-2xl font-bold text-gray-900">Langfuse 配置</h1>
        <p className="text-gray-500 mt-1">配置 Langfuse 追踪系统，用于监控和分析 AI 调用性能</p>
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

      {/* Basic Config */}
      <div className="card">
        <div className="card-header">
          <BarChart3 className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">基础配置</h3>
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
              <label className="form-label mb-0">启用 Langfuse 追踪</label>
              <p className="text-sm text-gray-500">开启后将自动追踪所有 AI 调用，包括输入输出、Token 消耗、耗时等</p>
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

          {/* Keys */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="form-group mb-0">
              <label className="form-label">Public Key</label>
              <input
                type="text"
                value={config.public_key}
                onChange={(e) => setConfig({ ...config, public_key: e.target.value })}
                placeholder="pk-lf-..."
                className="form-input"
              />
              <p className="text-xs text-gray-500 mt-1">从 Langfuse 控制台获取的 Public Key</p>
            </div>
            <div className="form-group mb-0">
              <label className="form-label">Secret Key</label>
              <input
                type="password"
                value={config.secret_key}
                onChange={(e) => setConfig({ ...config, secret_key: e.target.value })}
                placeholder="sk-lf-..."
                className="form-input"
              />
              <p className="text-xs text-gray-500 mt-1">从 Langfuse 控制台获取的 Secret Key</p>
            </div>
          </div>

          <div className="form-group mb-0">
            <label className="form-label">Base URL</label>
            <input
              type="text"
              value={config.base_url}
              onChange={(e) => setConfig({ ...config, base_url: e.target.value })}
              placeholder="https://cloud.langfuse.com"
              className="form-input"
            />
            <p className="text-xs text-gray-500 mt-1">Langfuse 服务端点，使用云服务时保持默认即可</p>
          </div>
        </div>
      </div>

      {/* Advanced Config */}
      <div className="card">
        <div className="card-header">
          <BarChart3 className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">高级配置</h3>
        </div>
        <div className="card-body space-y-6">
          {/* Async Toggle */}
          <div className="flex items-center justify-between">
            <div>
              <label className="form-label mb-0">启用异步批量处理</label>
              <p className="text-sm text-gray-500">开启后可提高性能，但会有短暂的延迟（推荐开启）</p>
            </div>
            <button
              onClick={() => setConfig({ ...config, async_enabled: !config.async_enabled })}
              className={`toggle ${config.async_enabled ? 'toggle-enabled' : 'toggle-disabled'}`}
            >
              <span
                className={`toggle-thumb ${
                  config.async_enabled ? 'toggle-thumb-enabled' : 'toggle-thumb-disabled'
                }`}
              />
            </button>
          </div>

          <div className="border-t border-gray-100" />

          {/* Batch Settings */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="form-group mb-0">
              <label className="form-label">批量大小</label>
              <input
                type="number"
                value={config.batch_size}
                onChange={(e) => setConfig({ ...config, batch_size: parseInt(e.target.value) || 100 })}
                min={10}
                max={1000}
                className="form-input"
              />
              <p className="text-xs text-gray-500 mt-1">每批发送的事件数量</p>
            </div>
            <div className="form-group mb-0">
              <label className="form-label">刷新间隔（秒）</label>
              <input
                type="number"
                value={config.flush_interval}
                onChange={(e) => setConfig({ ...config, flush_interval: parseInt(e.target.value) || 5 })}
                min={1}
                max={60}
                className="form-input"
              />
              <p className="text-xs text-gray-500 mt-1">批量数据发送间隔</p>
            </div>
          </div>
        </div>
      </div>

      {/* Quick Links */}
      <div className="card">
        <div className="card-header">
          <ExternalLink className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">快速链接</h3>
        </div>
        <div className="card-body">
          <a
            href="https://cloud.langfuse.com"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 btn-secondary"
          >
            <ExternalLink className="w-4 h-4" />
            打开 Langfuse 控制台
          </a>
          <span className="ml-4 text-sm text-gray-500">
            查看追踪数据、分析性能和成本
          </span>
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
