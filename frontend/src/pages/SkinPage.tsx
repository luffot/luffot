import { useState, useEffect } from 'react'
import { Palette, Check, Upload, AlertCircle, Save } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { SkinListResponse } from '../types'

export default function SkinPage() {
  const [skinData, setSkinData] = useState<SkinListResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [importing, setImporting] = useState(false)
  const [showImportPanel, setShowImportPanel] = useState(false)
  const [importDir, setImportDir] = useState('')
  const [importName, setImportName] = useState('')
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadSkins()
  }, [])

  const loadSkins = async () => {
    try {
      const data = await wailsAPI.getSkins()
      setSkinData(data)
    } catch (error) {
      console.error('Failed to load skins:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSelectSkin = async (skinName: string) => {
    setSaving(true)
    try {
      await wailsAPI.setSkin(skinName)
      setSkinData(prev => prev ? { ...prev, current_skin: skinName } : null)
      setMessage({ type: 'success', text: '皮肤已切换' })
    } catch (error) {
      setMessage({ type: 'error', text: '切换失败：' + String(error) })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const handleImport = async () => {
    if (!importDir.trim()) {
      setMessage({ type: 'error', text: '请输入皮肤目录路径' })
      return
    }
    setImporting(true)
    try {
      await wailsAPI.importSkin(importDir.trim(), importName.trim())
      setMessage({ type: 'success', text: '皮肤导入成功' })
      setShowImportPanel(false)
      setImportDir('')
      setImportName('')
      loadSkins()
    } catch (error) {
      setMessage({ type: 'error', text: '导入失败：' + String(error) })
    } finally {
      setImporting(false)
    }
  }

  const getSkinTypeLabel = (type: string) => {
    switch (type) {
      case 'vector':
        return '矢量皮肤'
      case 'image':
        return '图片皮肤'
      case 'lua':
        return 'Lua 皮肤'
      default:
        return type
    }
  }

  const getSkinTypeColor = (type: string) => {
    switch (type) {
      case 'vector':
        return 'bg-blue-100 text-blue-700'
      case 'image':
        return 'bg-green-100 text-green-700'
      case 'lua':
        return 'bg-purple-100 text-purple-700'
      default:
        return 'bg-gray-100 text-gray-700'
    }
  }

  if (loading || !skinData) {
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
        <h1 className="text-2xl font-bold text-gray-900">皮肤配置</h1>
        <p className="text-gray-500 mt-1">选择桌宠的外观皮肤，切换后立即生效，无需重启</p>
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

      {/* Skin List Card */}
      <div className="card">
        <div className="card-header flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Palette className="w-5 h-5 text-primary-600" />
            <h3 className="text-lg font-semibold text-gray-900">选择皮肤</h3>
          </div>
          <div className="flex items-center gap-3">
            <span className="badge badge-info">
              当前: {skinData.current_skin || '经典皮肤'}
            </span>
            <button
              onClick={() => setShowImportPanel(!showImportPanel)}
              className="btn-secondary btn-sm"
            >
              <Upload className="w-4 h-4" />
              导入皮肤
            </button>
          </div>
        </div>

        {/* Import Panel */}
        {showImportPanel && (
          <div className="mx-6 mt-4 p-4 bg-green-50 border border-green-200 rounded-lg">
            <h4 className="font-medium text-green-800 mb-2">导入 skin-builder 生成的皮肤</h4>
            <p className="text-sm text-green-700 mb-4">
              使用 <code className="px-1.5 py-0.5 bg-green-100 rounded">go run ./cmd/skin-builder</code> 生成皮肤素材后，
              填入生成的皮肤目录路径即可将皮肤注册到桌宠。
            </p>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-green-800 mb-1">
                  皮肤目录路径（绝对路径或相对路径）
                </label>
                <input
                  type="text"
                  value={importDir}
                  onChange={(e) => setImportDir(e.target.value)}
                  placeholder="例如：/Users/me/luffot/assets/skins/我的猫咪"
                  className="w-full px-3 py-2 border border-green-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-green-800 mb-1">
                  皮肤名称（可选，留空则自动读取 skin_meta.json 或使用目录名）
                </label>
                <input
                  type="text"
                  value={importName}
                  onChange={(e) => setImportName(e.target.value)}
                  placeholder="例如：我的猫咪"
                  className="w-full px-3 py-2 border border-green-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
                />
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={handleImport}
                  disabled={importing}
                  className="btn-primary btn-sm"
                >
                  <Save className="w-4 h-4" />
                  {importing ? '导入中...' : '确认导入'}
                </button>
                <button
                  onClick={() => setShowImportPanel(false)}
                  className="btn-secondary btn-sm"
                >
                  取消
                </button>
              </div>
            </div>
          </div>
        )}

        <div className="card-body">
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {skinData.skins.map((skin) => {
              const isSelected = skinData.current_skin === skin.internal
              return (
                <button
                  key={skin.internal}
                  onClick={() => handleSelectSkin(skin.internal)}
                  disabled={saving}
                  className={`relative p-4 rounded-xl border-2 text-left transition-all ${
                    isSelected
                      ? 'border-primary-500 bg-primary-50'
                      : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                  }`}
                >
                  {isSelected && (
                    <div className="absolute top-3 right-3 w-6 h-6 bg-primary-600 rounded-full flex items-center justify-center">
                      <Check className="w-4 h-4 text-white" />
                    </div>
                  )}
                  <span className={`inline-block px-2 py-0.5 text-xs rounded-full mb-2 ${getSkinTypeColor(skin.type)}`}>
                    {getSkinTypeLabel(skin.type)}
                  </span>
                  <h4 className="font-medium text-gray-900">{skin.name}</h4>
                  <p className="text-sm text-gray-500 mt-1">{skin.description}</p>
                </button>
              )
            })}
          </div>
        </div>
        <div className="card-footer">
          <span className="status-dot status-dot-success" />
          <span>共 {skinData.skins.length} 个皮肤可用</span>
        </div>
      </div>
    </div>
  )
}
