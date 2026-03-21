import { useState, useEffect } from 'react'
import { Save, Bell, AlertCircle } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { AlertConfig } from '../types'

export default function AlertConfigPage() {
  const [config, setConfig] = useState<AlertConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadConfig()
  }, [])

  const loadConfig = async () => {
    try {
      const data = await wailsAPI.getAlertConfig()
      setConfig(data)
    } catch (error) {
      console.error('Failed to load alert config:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    try {
      await wailsAPI.saveAlertConfig(config)
      setMessage({ type: 'success', text: '保存成功，配置已生效' })
    } catch (error) {
      setMessage({ type: 'error', text: '保存失败：' + String(error) })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const parseKeywords = (text: string): string[] => {
    return text.split('\n').map(k => k.trim()).filter(k => k.length > 0)
  }

  const formatKeywords = (keywords: string[]): string => {
    return keywords.join('\n')
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
        <h1 className="text-2xl font-bold text-gray-900">告警配置</h1>
        <p className="text-gray-500 mt-1">设置关键词告警规则，消息命中时桌宠主动提醒</p>
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

      {/* Alert Config Card */}
      <div className="card">
        <div className="card-header">
          <Bell className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">告警规则</h3>
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
              <label className="form-label mb-0">启用告警检测</label>
              <p className="text-sm text-gray-500">消息包含关键词时，桌宠会主动弹出提醒气泡</p>
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

          {/* Alert Keywords */}
          <div className="form-group mb-0">
            <label className="form-label">告警关键词</label>
            <textarea
              value={formatKeywords(config.keywords)}
              onChange={(e) =>
                setConfig({ ...config, keywords: parseKeywords(e.target.value) })
              }
              rows={8}
              placeholder={`每行一个关键词，大小写不敏感
例如：
紧急
故障
线上问题`}
              className="form-textarea font-mono text-sm"
            />
            <p className="text-xs text-gray-500 mt-2">
              每行填写一个关键词，消息内容包含任意一个关键词即触发告警提醒。修改后立即生效。
            </p>
          </div>

          <div className="border-t border-gray-100" />

          {/* Filter Keywords */}
          <div className="form-group mb-0">
            <label className="form-label">过滤关键词</label>
            <textarea
              value={formatKeywords(config.filter_keywords)}
              onChange={(e) =>
                setConfig({ ...config, filter_keywords: parseKeywords(e.target.value) })
              }
              rows={5}
              placeholder={`每行一个过滤词，大小写不敏感
例如：
测试群
机器人
已解决`}
              className="form-textarea font-mono text-sm"
            />
            <p className="text-xs text-gray-500 mt-2">
              消息内容命中任意过滤词时，<strong>即使包含告警关键词也不会触发告警</strong>。
              适合屏蔽不关心的群组、机器人消息等。修改后立即生效。
            </p>
          </div>
        </div>
        <div className="card-footer">
          <span
            className={`status-dot ${config.enabled ? 'status-dot-success' : 'status-dot-gray'}`}
          />
          <span>
            {config.enabled
              ? `已启用 · ${config.keywords.length} 个关键词 · ${config.filter_keywords.length} 个过滤词`
              : '告警检测已禁用'}
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
          {saving ? '保存中...' : '保存告警规则'}
        </button>
      </div>
    </div>
  )
}
