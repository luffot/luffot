import { useState, useEffect } from 'react'
import { Save, Clock, Bot, Shield, AlertCircle, FileText } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { CameraGuardConfig } from '../types'

export default function CameraSettingsPage() {
  const [config, setConfig] = useState<CameraGuardConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [promptContent, setPromptContent] = useState('')
  const [promptLoading, setPromptLoading] = useState(true)

  useEffect(() => {
    loadConfig()
    loadPrompt()
  }, [])

  const loadConfig = async () => {
    try {
      const data = await wailsAPI.getCameraGuardConfig()
      setConfig(data)
    } catch (error) {
      console.error('Failed to load camera config:', error)
    } finally {
      setLoading(false)
    }
  }

  const loadPrompt = async () => {
    try {
      const data = await wailsAPI.getPrompt('camera_guard')
      setPromptContent(typeof data === 'string' ? data : data?.content || '')
    } catch (error) {
      console.error('Failed to load camera_guard prompt:', error)
    } finally {
      setPromptLoading(false)
    }
  }

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    try {
      await wailsAPI.saveCameraGuardConfig(config)
      await wailsAPI.savePrompt('camera_guard', promptContent)
      setMessage({ type: 'success', text: '保存成功，配置已生效' })
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
        <h1 className="text-2xl font-bold text-gray-900">摄像头监测设置</h1>
        <p className="text-gray-500 mt-1">配置摄像头守卫的检测间隔、AI 分析提示词及使用的模型 Provider</p>
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

      {/* Detection Interval Card */}
      <div className="card">
        <div className="card-header">
          <Clock className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">检测间隔</h3>
        </div>
        <div className="card-body space-y-6">
          {/* Enable Toggle */}
          <div className="flex items-center justify-between">
            <div>
              <label className="form-label mb-0">启用摄像头监测</label>
              <p className="text-sm text-gray-500">开启后按设定间隔自动拍照并进行 AI 分析</p>
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

          {/* Interval */}
          <div className="form-group mb-0">
            <label className="form-label">检测间隔（秒）</label>
            <input
              type="number"
              value={config.interval_seconds}
              onChange={(e) => setConfig({ ...config, interval_seconds: parseInt(e.target.value) || 30 })}
              min={5}
              max={3600}
              className="form-input w-48"
            />
            <p className="text-xs text-gray-500 mt-1">每隔多少秒拍一次照进行检测，建议 15~60 秒，最小 5 秒</p>
          </div>
        </div>
      </div>

      {/* Model Provider Card */}
      <div className="card">
        <div className="card-header">
          <Bot className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">模型 Provider</h3>
        </div>
        <div className="card-body">
          <div className="form-group mb-0">
            <label className="form-label">使用的 Provider 名称</label>
            <input
              type="text"
              value={config.provider_name}
              onChange={(e) => setConfig({ ...config, provider_name: e.target.value })}
              placeholder="vision"
              className="form-input"
            />
            <p className="text-xs text-gray-500 mt-1">
              填写在「模型配置」中已定义的 Provider 名称，需支持图像输入（Vision）
            </p>
          </div>
        </div>
      </div>

      {/* Alert Strategy Card */}
      <div className="card">
        <div className="card-header">
          <Shield className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">告警策略</h3>
        </div>
        <div className="card-body">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="form-group mb-0">
              <label className="form-label">连续确认次数</label>
              <input
                type="number"
                value={config.confirm_count}
                onChange={(e) => setConfig({ ...config, confirm_count: parseInt(e.target.value) || 2 })}
                min={1}
                max={10}
                className="form-input"
              />
              <p className="text-xs text-gray-500 mt-1">连续检测到人几次才触发告警，避免误报，建议 2~3</p>
            </div>
            <div className="form-group mb-0">
              <label className="form-label">告警冷却时间（秒）</label>
              <input
                type="number"
                value={config.cooldown_seconds}
                onChange={(e) => setConfig({ ...config, cooldown_seconds: parseInt(e.target.value) || 60 })}
                min={10}
                max={3600}
                className="form-input"
              />
              <p className="text-xs text-gray-500 mt-1">触发告警后多少秒内不再重复告警，建议 60~300</p>
            </div>
          </div>
        </div>
      </div>

      {/* Prompt Card */}
      <div className="card">
        <div className="card-header">
          <FileText className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">检测提示词</h3>
        </div>
        <div className="card-body">
          <p className="text-sm text-gray-500 mb-4">
            自定义 AI 分析摄像头画面时使用的提示词，用于判断是否需要触发告警
          </p>
          <div className="form-group mb-0">
            {promptLoading ? (
              <div className="flex items-center justify-center h-32 bg-gray-50 rounded-lg">
                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary-600" />
              </div>
            ) : (
              <textarea
                value={promptContent}
                onChange={(e) => setPromptContent(e.target.value)}
                placeholder="请输入检测提示词..."
                className="form-input h-32 resize-y"
                rows={8}
              />
            )}
            <p className="text-xs text-gray-500 mt-1">
              提示词将用于 AI 分析摄像头截图，判断是否检测到需要告警的场景
            </p>
          </div>
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
          {saving ? '保存中...' : '保存设置'}
        </button>
      </div>
    </div>
  )
}