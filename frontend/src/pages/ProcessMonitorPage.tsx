import { useState, useEffect } from 'react'
import { Monitor, Plus, Trash2, Save, AlertCircle } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { AppConfigItem } from '../types'

export default function ProcessMonitorPage() {
  const [apps, setApps] = useState<AppConfigItem[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [editingApp, setEditingApp] = useState<AppConfigItem | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)
  const [newApp, setNewApp] = useState<Partial<AppConfigItem>>({
    name: '',
    process_name: '',
    display_name: '',
    enabled: true,
    parse_rules: {
      sender_pattern: '(\\S+)\\s+\\d{1,2}:\\d{2}',
      time_pattern: '\\d{1,2}:\\d{2}',
      content_mode: 'after_time',
      dedup_enabled: true,
    },
    session_config: {
      source: 'window_title',
    },
    process_monitor: {
      use_vlmodel: false,
      vlmodel_provider: '',
      vlmodel_prompt: '',
    },
  })

  useEffect(() => {
    loadApps()
  }, [])

  const loadApps = async () => {
    try {
      const data = await wailsAPI.getApps()
      setApps(data.apps || [])
    } catch (error) {
      console.error('Failed to load apps:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSaveApp = async (app: AppConfigItem) => {
    setSaving(true)
    try {
      await wailsAPI.saveAppConfig(app.name, app)
      setMessage({ type: 'success', text: '保存成功' })
      loadApps()
      setEditingApp(null)
    } catch (error) {
      setMessage({ type: 'error', text: '保存失败：' + String(error) })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const handleAddApp = async () => {
    if (!newApp.name || !newApp.process_name) {
      setMessage({ type: 'error', text: '请填写应用名称和进程名' })
      return
    }
    setSaving(true)
    try {
      await wailsAPI.addAppConfig(newApp as AppConfigItem)
      setMessage({ type: 'success', text: '添加成功' })
      loadApps()
      setShowAddForm(false)
      setNewApp({
        name: '',
        process_name: '',
        display_name: '',
        enabled: true,
        parse_rules: {
          sender_pattern: '(\\S+)\\s+\\d{1,2}:\\d{2}',
          time_pattern: '\\d{1,2}:\\d{2}',
          content_mode: 'after_time',
          dedup_enabled: true,
        },
        session_config: {
          source: 'window_title',
        },
        process_monitor: {
          use_vlmodel: false,
          vlmodel_provider: '',
          vlmodel_prompt: '',
        },
      })
    } catch (error) {
      setMessage({ type: 'error', text: '添加失败：' + String(error) })
    } finally {
      setSaving(false)
    }
  }

  const handleDeleteApp = async (name: string) => {
    if (!confirm(`确定要删除应用 "${name}" 吗？`)) return
    try {
      await wailsAPI.removeAppConfig(name)
      setMessage({ type: 'success', text: '删除成功' })
      loadApps()
    } catch (error) {
      setMessage({ type: 'error', text: '删除失败：' + String(error) })
    }
    setTimeout(() => setMessage(null), 3000)
  }

  if (loading) {
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
        <h1 className="text-2xl font-bold text-gray-900">进程监控</h1>
        <p className="text-gray-500 mt-1">添加和管理要监控的应用进程，配置 VLModel 识别优化</p>
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

      {/* Add Button */}
      <div className="flex justify-end">
        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="btn-primary"
        >
          <Plus className="w-4 h-4" />
          {showAddForm ? '取消' : '添加应用'}
        </button>
      </div>

      {/* Add Form */}
      {showAddForm && (
        <div className="card">
          <div className="card-header">
            <Monitor className="w-5 h-5 text-primary-600" />
            <h3 className="text-lg font-semibold text-gray-900">添加新应用</h3>
          </div>
          <div className="card-body">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="form-group mb-0">
                <label className="form-label">应用标识名 *</label>
                <input
                  type="text"
                  value={newApp.name}
                  onChange={(e) => setNewApp({ ...newApp, name: e.target.value })}
                  placeholder="例如：dingtalk"
                  className="form-input"
                />
              </div>
              <div className="form-group mb-0">
                <label className="form-label">进程名 *</label>
                <input
                  type="text"
                  value={newApp.process_name}
                  onChange={(e) => setNewApp({ ...newApp, process_name: e.target.value })}
                  placeholder="例如：DingTalk"
                  className="form-input"
                />
              </div>
              <div className="form-group mb-0">
                <label className="form-label">显示名称</label>
                <input
                  type="text"
                  value={newApp.display_name}
                  onChange={(e) => setNewApp({ ...newApp, display_name: e.target.value })}
                  placeholder="例如：钉钉"
                  className="form-input"
                />
              </div>
              <div className="form-group mb-0 flex items-end">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={newApp.enabled}
                    onChange={(e) => setNewApp({ ...newApp, enabled: e.target.checked })}
                    className="w-4 h-4 text-primary-600 rounded"
                  />
                  <span className="text-sm">启用监控</span>
                </label>
              </div>
            </div>
          </div>
          <div className="card-footer flex justify-end">
            <button
              onClick={handleAddApp}
              disabled={saving}
              className="btn-primary"
            >
              <Save className="w-4 h-4" />
              {saving ? '保存中...' : '确认添加'}
            </button>
          </div>
        </div>
      )}

      {/* App List */}
      <div className="card">
        <div className="card-header">
          <Monitor className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">监控进程列表</h3>
        </div>
        <div className="divide-y divide-gray-100">
          {apps.length === 0 ? (
            <div className="p-8 text-center text-gray-500">
              暂无监控应用
            </div>
          ) : (
            apps.map((app) => (
              <div key={app.name} className="p-6">
                {editingApp?.name === app.name ? (
                  // Edit Mode
                  <div className="space-y-4">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div className="form-group mb-0">
                        <label className="form-label">应用标识名</label>
                        <input
                          type="text"
                          value={editingApp.name}
                          disabled
                          className="form-input bg-gray-100"
                        />
                      </div>
                      <div className="form-group mb-0">
                        <label className="form-label">进程名</label>
                        <input
                          type="text"
                          value={editingApp.process_name}
                          onChange={(e) => setEditingApp({ ...editingApp, process_name: e.target.value })}
                          className="form-input"
                        />
                      </div>
                      <div className="form-group mb-0">
                        <label className="form-label">显示名称</label>
                        <input
                          type="text"
                          value={editingApp.display_name}
                          onChange={(e) => setEditingApp({ ...editingApp, display_name: e.target.value })}
                          className="form-input"
                        />
                      </div>
                      <div className="form-group mb-0 flex items-end">
                        <label className="flex items-center gap-2 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={editingApp.enabled}
                            onChange={(e) => setEditingApp({ ...editingApp, enabled: e.target.checked })}
                            className="w-4 h-4 text-primary-600 rounded"
                          />
                          <span className="text-sm">启用监控</span>
                        </label>
                      </div>
                    </div>
                    {/* VLModel Settings */}
                    <div className="border-t border-gray-100 pt-4">
                      <h4 className="font-medium text-gray-900 mb-3">VLModel 设置</h4>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div className="form-group mb-0">
                          <label className="flex items-center gap-2 cursor-pointer">
                            <input
                              type="checkbox"
                              checked={editingApp.process_monitor?.use_vlmodel || false}
                              onChange={(e) => setEditingApp({
                                ...editingApp,
                                process_monitor: {
                                  ...editingApp.process_monitor,
                                  use_vlmodel: e.target.checked,
                                },
                              })}
                              className="w-4 h-4 text-primary-600 rounded"
                            />
                            <span className="text-sm">启用 VLModel</span>
                          </label>
                        </div>
                        <div className="form-group mb-0">
                          <label className="form-label">VLModel Provider</label>
                          <input
                            type="text"
                            value={editingApp.process_monitor?.vlmodel_provider || ''}
                            onChange={(e) => setEditingApp({
                              ...editingApp,
                              process_monitor: {
                                use_vlmodel: editingApp.process_monitor?.use_vlmodel ?? false,
                                vlmodel_provider: e.target.value,
                                vlmodel_prompt: editingApp.process_monitor?.vlmodel_prompt,
                              },
                            })}
                            placeholder="例如：vision"
                            className="form-input"
                          />
                        </div>
                        <div className="form-group mb-0">
                          <label className="form-label">提示词名称</label>
                          <input
                            type="text"
                            value={editingApp.process_monitor?.vlmodel_prompt || ''}
                            onChange={(e) => setEditingApp({
                              ...editingApp,
                              process_monitor: {
                                use_vlmodel: editingApp.process_monitor?.use_vlmodel ?? false,
                                vlmodel_provider: editingApp.process_monitor?.vlmodel_provider,
                                vlmodel_prompt: e.target.value,
                              },
                            })}
                            placeholder="留空使用默认"
                            className="form-input"
                          />
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 pt-2">
                      <button
                        onClick={() => handleSaveApp(editingApp)}
                        disabled={saving}
                        className="btn-primary btn-sm"
                      >
                        <Save className="w-4 h-4" />
                        {saving ? '保存中...' : '保存'}
                      </button>
                      <button
                        onClick={() => setEditingApp(null)}
                        className="btn-secondary btn-sm"
                      >
                        取消
                      </button>
                    </div>
                  </div>
                ) : (
                  // View Mode
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-gray-900">
                            {app.display_name || app.name}
                          </span>
                          <span
                            className={`px-2 py-0.5 text-xs rounded-full ${
                              app.enabled
                                ? 'bg-green-100 text-green-700'
                                : 'bg-gray-100 text-gray-600'
                            }`}
                          >
                            {app.enabled ? '运行中' : '已禁用'}
                          </span>
                          {app.process_monitor?.use_vlmodel && (
                            <span className="px-2 py-0.5 text-xs rounded-full bg-purple-100 text-purple-700">
                              VLModel
                            </span>
                          )}
                        </div>
                        <div className="text-sm text-gray-500 mt-1">
                          进程: {app.process_name} | 标识: {app.name}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setEditingApp(app)}
                        className="btn-secondary btn-sm"
                      >
                        编辑
                      </button>
                      <button
                        onClick={() => handleDeleteApp(app.name)}
                        className="text-red-500 hover:text-red-700 p-2"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
