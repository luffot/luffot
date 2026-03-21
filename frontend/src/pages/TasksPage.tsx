import { useState, useEffect } from 'react'
import { Clock, Play, RefreshCw, AlertCircle } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { ScheduledTask } from '../types'

export default function TasksPage() {
  const [tasks, setTasks] = useState<ScheduledTask[]>([])
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadTasks()
  }, [])

  const loadTasks = async () => {
    try {
      const data = await wailsAPI.getTasks()
      setTasks(data.tasks || [])
    } catch (error) {
      console.error('Failed to load tasks:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleTriggerTask = async (name: string) => {
    try {
      await wailsAPI.triggerTask(name)
      setMessage({ type: 'success', text: `任务 "${name}" 已触发` })
      loadTasks()
    } catch (error) {
      setMessage({ type: 'error', text: '触发失败：' + String(error) })
    }
    setTimeout(() => setMessage(null), 3000)
  }

  const getTaskTypeLabel = (type: string) => {
    switch (type) {
      case 'builtin':
        return '内置任务'
      case 'python':
        return 'Python 脚本'
      default:
        return type
    }
  }

  const getTaskTypeColor = (type: string) => {
    switch (type) {
      case 'builtin':
        return 'bg-blue-100 text-blue-700'
      case 'python':
        return 'bg-green-100 text-green-700'
      default:
        return 'bg-gray-100 text-gray-700'
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
        <h1 className="text-2xl font-bold text-gray-900">定时任务</h1>
        <p className="text-gray-500 mt-1">查看已注册的定时任务状态，支持手动触发</p>
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

      {/* Tasks List */}
      <div className="card">
        <div className="card-header flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Clock className="w-5 h-5 text-primary-600" />
            <h3 className="text-lg font-semibold text-gray-900">任务列表</h3>
          </div>
          <button
            onClick={loadTasks}
            className="btn-secondary btn-sm"
          >
            <RefreshCw className="w-4 h-4" />
            刷新
          </button>
        </div>
        <div className="divide-y divide-gray-100">
          {tasks.length === 0 ? (
            <div className="p-8 text-center text-gray-500">
              暂无定时任务
            </div>
          ) : (
            tasks.map((task) => (
              <div key={task.name} className="p-6">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-2">
                      <span className="font-medium text-gray-900">{task.name}</span>
                      <span
                        className={`px-2 py-0.5 text-xs rounded-full ${
                          task.enabled
                            ? 'bg-green-100 text-green-700'
                            : 'bg-gray-100 text-gray-600'
                        }`}
                      >
                        {task.enabled ? '已启用' : '已禁用'}
                      </span>
                      <span className={`px-2 py-0.5 text-xs rounded-full ${getTaskTypeColor(task.type)}`}>
                        {getTaskTypeLabel(task.type)}
                      </span>
                    </div>
                    {task.description && (
                      <p className="text-sm text-gray-500 mb-2">{task.description}</p>
                    )}
                    <div className="flex items-center gap-4 text-sm text-gray-500">
                      <span className="font-mono bg-gray-100 px-2 py-0.5 rounded">
                        {task.cron}
                      </span>
                      {task.last_run && (
                        <span>上次运行: {new Date(task.last_run).toLocaleString()}</span>
                      )}
                      {task.next_run && (
                        <span>下次运行: {new Date(task.next_run).toLocaleString()}</span>
                      )}
                      <span>运行次数: {task.run_count}</span>
                    </div>
                    {task.builtin_name && (
                      <p className="text-xs text-gray-400 mt-1">
                        内置任务: {task.builtin_name}
                      </p>
                    )}
                    {task.script_path && (
                      <p className="text-xs text-gray-400 mt-1">
                        脚本路径: {task.script_path}
                      </p>
                    )}
                  </div>
                  <button
                    onClick={() => handleTriggerTask(task.name)}
                    disabled={!task.enabled}
                    className="btn-primary btn-sm disabled:opacity-50"
                  >
                    <Play className="w-4 h-4" />
                    立即执行
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
