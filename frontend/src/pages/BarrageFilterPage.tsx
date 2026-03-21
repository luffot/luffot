import { useState, useEffect } from 'react'
import { Save, MessageSquare, AlertCircle } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { BarrageConfig } from '../types'

export default function BarrageFilterPage() {
  const [config, setConfig] = useState<BarrageConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadConfig()
  }, [])

  const loadConfig = async () => {
    try {
      const data = await wailsAPI.getBarrageConfig()
      setConfig(data)
    } catch (error) {
      console.error('Failed to load barrage config:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    try {
      await wailsAPI.saveBarrageConfig(config)
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
        <h1 className="text-2xl font-bold text-gray-900">弹幕过滤设置</h1>
        <p className="text-gray-500 mt-1">配置过滤关键词，包含关键词的消息将不会出现在弹幕中</p>
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

      {/* Filter Config Card */}
      <div className="card">
        <div className="card-header">
          <MessageSquare className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">过滤关键词</h3>
          <span className="ml-auto badge badge-info">
            {config.filter_keywords.length} 个关键词
          </span>
        </div>
        <div className="card-body">
          <div className="form-group mb-0">
            <label className="form-label">过滤关键词列表</label>
            <textarea
              value={formatKeywords(config.filter_keywords)}
              onChange={(e) =>
                setConfig({ ...config, filter_keywords: parseKeywords(e.target.value) })
              }
              rows={10}
              placeholder={`每行一个关键词，大小写不敏感
例如：
广告
机器人
测试消息`}
              className="form-textarea font-mono text-sm"
            />
            <p className="text-xs text-gray-500 mt-2">
              每行填写一个关键词，消息内容包含任意一个关键词时，该消息将被静默过滤，不在弹幕中显示。
              大小写不敏感，修改后立即生效。
            </p>
          </div>
        </div>
        <div className="card-footer">
          <span className="status-dot status-dot-success" />
          <span>已配置 {config.filter_keywords.length} 个过滤关键词</span>
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
          {saving ? '保存中...' : '保存过滤设置'}
        </button>
      </div>
    </div>
  )
}
