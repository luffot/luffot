import { useState, useEffect } from 'react'
import { Save, MessageSquare, Star, Plus, Trash2, AlertCircle } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { BarrageConfig, BarrageHighlightRule } from '../types'

const DEFAULT_COLOR = '#FFD700'

export default function BarrageSettingsPage() {
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

  const addHighlightRule = () => {
    if (!config) return
    const newRule: BarrageHighlightRule = {
      keyword: '',
      color: DEFAULT_COLOR,
    }
    setConfig({
      ...config,
      highlight_rules: [...config.highlight_rules, newRule],
    })
  }

  const removeHighlightRule = (index: number) => {
    if (!config) return
    const newRules = [...config.highlight_rules]
    newRules.splice(index, 1)
    setConfig({ ...config, highlight_rules: newRules })
  }

  const updateHighlightRule = (index: number, field: keyof BarrageHighlightRule, value: string) => {
    if (!config) return
    const newRules = [...config.highlight_rules]
    newRules[index] = { ...newRules[index], [field]: value }
    setConfig({ ...config, highlight_rules: newRules })
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
        <h1 className="text-2xl font-bold text-gray-900">弹幕设置</h1>
        <p className="text-gray-500 mt-1">管理弹幕过滤关键词和特别关注规则</p>
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

      {/* ==================== 过滤关键词 ==================== */}
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
              rows={8}
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

      {/* ==================== 特别关注规则 ==================== */}
      <div className="card">
        <div className="card-header flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Star className="w-5 h-5 text-primary-600" />
            <h3 className="text-lg font-semibold text-gray-900">特别关注规则</h3>
          </div>
          <div className="flex items-center gap-3">
            <span className="badge badge-info">{config.highlight_rules.length} 个规则</span>
            <button onClick={addHighlightRule} className="btn-primary btn-sm">
              <Plus className="w-4 h-4" />
              添加规则
            </button>
          </div>
        </div>
        <div className="card-body">
          {/* Info Box */}
          <div className="mb-6 p-4 bg-yellow-50 border border-yellow-200 rounded-lg flex items-start gap-3">
            <AlertCircle className="w-5 h-5 text-yellow-600 flex-shrink-0 mt-0.5" />
            <p className="text-sm text-yellow-800">
              消息内容命中关键词时，弹幕将以指定颜色高亮渲染，默认使用金色
              <span
                className="inline-block w-4 h-4 rounded ml-1 align-middle border border-gray-200"
                style={{ backgroundColor: DEFAULT_COLOR }}
              />
              <code className="ml-1 px-1.5 py-0.5 bg-yellow-100 rounded text-xs">{DEFAULT_COLOR}</code>
              。颜色可自定义，修改后立即生效。
            </p>
          </div>

          {/* Rules List */}
          {config.highlight_rules.length === 0 ? (
            <div className="text-center py-12 text-gray-500">
              暂无规则，点击「添加规则」创建
            </div>
          ) : (
            <div className="space-y-4">
              {config.highlight_rules.map((rule, index) => (
                <div
                  key={index}
                  className="flex items-center gap-4 p-4 bg-gray-50 rounded-lg"
                >
                  <div className="flex-1">
                    <label className="block text-xs font-medium text-gray-700 mb-1">
                      关键词
                    </label>
                    <input
                      type="text"
                      value={rule.keyword}
                      onChange={(e) => updateHighlightRule(index, 'keyword', e.target.value)}
                      placeholder="输入关键词"
                      className="form-input"
                    />
                  </div>
                  <div className="w-32">
                    <label className="block text-xs font-medium text-gray-700 mb-1">
                      颜色
                    </label>
                    <div className="flex items-center gap-2">
                      <input
                        type="color"
                        value={rule.color || DEFAULT_COLOR}
                        onChange={(e) => updateHighlightRule(index, 'color', e.target.value)}
                        className="w-10 h-9 rounded cursor-pointer border border-gray-300"
                      />
                      <input
                        type="text"
                        value={rule.color || DEFAULT_COLOR}
                        onChange={(e) => updateHighlightRule(index, 'color', e.target.value)}
                        placeholder="#FFD700"
                        className="form-input w-24 text-xs font-mono"
                      />
                    </div>
                  </div>
                  <button
                    onClick={() => removeHighlightRule(index)}
                    className="mt-5 text-red-500 hover:text-red-700 transition-colors"
                  >
                    <Trash2 className="w-5 h-5" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="card-footer">
          <span className="status-dot status-dot-success" />
          <span>已配置 {config.highlight_rules.length} 个特别关注规则</span>
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
          {saving ? '保存中...' : '保存弹幕设置'}
        </button>
      </div>
    </div>
  )
}
