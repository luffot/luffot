import { useState, useEffect } from 'react'
import { FileText, Save, AlertCircle, Folder } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { Prompt } from '../types'

export default function PromptPage() {
  const [prompts, setPrompts] = useState<Prompt[]>([])
  const [promptDir, setPromptDir] = useState('')
  const [selectedPrompt, setSelectedPrompt] = useState<Prompt | null>(null)
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadPrompts()
  }, [])

  const loadPrompts = async () => {
    try {
      const data = await wailsAPI.getPrompts()
      setPrompts(data.prompts || [])
      setPromptDir(data.dir || '')
    } catch (error) {
      console.error('Failed to load prompts:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSelectPrompt = async (prompt: Prompt) => {
    try {
      const data = await wailsAPI.getPrompt(prompt.name)
      setSelectedPrompt(prompt)
      setContent(data.content)
    } catch (error) {
      console.error('Failed to load prompt:', error)
    }
  }

  const handleSave = async () => {
    if (!selectedPrompt) return
    setSaving(true)
    try {
      await wailsAPI.savePrompt(selectedPrompt.name, content)
      setMessage({ type: 'success', text: '保存成功，配置已生效' })
    } catch (error) {
      setMessage({ type: 'error', text: '保存失败：' + String(error) })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
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
        <h1 className="text-2xl font-bold text-gray-900">提示词管理</h1>
        <p className="text-gray-500 mt-1">管理 AI 各场景的 Prompt 模板，修改后立即生效，无需重启</p>
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

      {/* Info Box */}
      <div className="p-4 bg-primary-50 border border-primary-200 rounded-lg flex items-start gap-3">
        <Folder className="w-5 h-5 text-primary-600 flex-shrink-0 mt-0.5" />
        <div className="text-sm text-primary-800">
          <p>
            每个提示词对应 <code className="px-1.5 py-0.5 bg-primary-100 rounded text-xs">{promptDir}</code> 目录下的一个 Markdown 文件。
          </p>
          <p className="mt-1">
            User Prompt 模板中可使用 <code className="px-1.5 py-0.5 bg-primary-100 rounded text-xs">{'{{profile}}'}</code>、
            <code className="px-1.5 py-0.5 bg-primary-100 rounded text-xs">{'{{messages}}'}</code>、
            <code className="px-1.5 py-0.5 bg-primary-100 rounded text-xs">{'{{old_profile}}'}</code> 占位符。
          </p>
        </div>
      </div>

      {/* Prompt Editor */}
      <div className="card">
        <div className="card-header">
          <FileText className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">Prompt 文件</h3>
          <span className="ml-auto text-xs font-mono text-gray-500">{promptDir}</span>
        </div>
        <div className="card-body">
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
            {/* Prompt List */}
            <div className="lg:col-span-1">
              <label className="form-label">选择提示词</label>
              <div className="border border-gray-200 rounded-lg overflow-hidden">
                {prompts.length === 0 ? (
                  <div className="p-4 text-center text-gray-500 text-sm">
                    暂无提示词文件
                  </div>
                ) : (
                  <div className="divide-y divide-gray-100 max-h-[400px] overflow-y-auto">
                    {prompts.map((prompt) => (
                      <button
                        key={prompt.name}
                        onClick={() => handleSelectPrompt(prompt)}
                        className={`w-full text-left px-4 py-3 text-sm transition-colors ${
                          selectedPrompt?.name === prompt.name
                            ? 'bg-primary-50 text-primary-700'
                            : 'hover:bg-gray-50 text-gray-700'
                        }`}
                      >
                        <div className="font-medium">{prompt.name}</div>
                        {prompt.updated_at && (
                          <div className="text-xs text-gray-400 mt-0.5">
                            更新于 {new Date(prompt.updated_at).toLocaleString()}
                          </div>
                        )}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {/* Editor */}
            <div className="lg:col-span-3">
              {selectedPrompt ? (
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <h4 className="font-medium text-gray-900">{selectedPrompt.name}</h4>
                    <button
                      onClick={handleSave}
                      disabled={saving}
                      className="btn-primary btn-sm"
                    >
                      <Save className="w-4 h-4" />
                      {saving ? '保存中...' : '保存'}
                    </button>
                  </div>
                  <textarea
                    value={content}
                    onChange={(e) => setContent(e.target.value)}
                    rows={20}
                    className="form-textarea font-mono text-sm"
                    placeholder="输入提示词内容..."
                  />
                </div>
              ) : (
                <div className="h-[400px] flex items-center justify-center text-gray-400">
                  请从左侧选择一个提示词文件进行编辑
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
