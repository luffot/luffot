import { useState, useEffect } from 'react'
import { Search, RefreshCw, MessageSquare, Users, Calendar } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { Message, MessageStats, AppConfigItem } from '../types'

export default function MessagesPage() {
  const [stats, setStats] = useState<MessageStats | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [apps, setApps] = useState<AppConfigItem[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [loading, setLoading] = useState(true)

  const loadData = async () => {
    try {
      const [statsData, messagesData, appsData] = await Promise.all([
        wailsAPI.getMessageStats(),
        wailsAPI.getMessages('', 50, 0),
        wailsAPI.getApps(),
      ])
      setStats(statsData)
      setMessages(messagesData.messages || [])
      setApps(appsData.apps || [])
    } catch (error) {
      console.error('Failed to load data:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSearch = async () => {
    if (!searchQuery.trim()) {
      loadData()
      return
    }
    try {
      const result = await wailsAPI.searchMessages(searchQuery, '', 50)
      setMessages(result.messages || [])
    } catch (error) {
      console.error('Search failed:', error)
    }
  }

  useEffect(() => {
    loadData()
    const interval = setInterval(loadData, 5000)
    return () => clearInterval(interval)
  }, [])

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
        <h1 className="text-2xl font-bold text-gray-900">消息总览</h1>
        <p className="text-gray-500 mt-1">查看所有监听到的消息，每 5 秒自动刷新</p>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="card">
          <div className="card-body flex items-center gap-4">
            <div className="w-12 h-12 bg-blue-100 rounded-xl flex items-center justify-center">
              <MessageSquare className="w-6 h-6 text-blue-600" />
            </div>
            <div>
              <p className="text-sm text-gray-500">总消息数</p>
              <p className="text-2xl font-bold text-gray-900">
                {stats?.total_messages?.toLocaleString() || 0}
              </p>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="card-body flex items-center gap-4">
            <div className="w-12 h-12 bg-green-100 rounded-xl flex items-center justify-center">
              <Calendar className="w-6 h-6 text-green-600" />
            </div>
            <div>
              <p className="text-sm text-gray-500">今日消息</p>
              <p className="text-2xl font-bold text-gray-900">
                {stats?.today_messages?.toLocaleString() || 0}
              </p>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="card-body flex items-center gap-4">
            <div className="w-12 h-12 bg-purple-100 rounded-xl flex items-center justify-center">
              <Users className="w-6 h-6 text-purple-600" />
            </div>
            <div>
              <p className="text-sm text-gray-500">监听应用</p>
              <p className="text-2xl font-bold text-gray-900">
                {Object.keys(stats?.app_counts || {}).length}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Messages List */}
        <div className="lg:col-span-2 card">
          <div className="card-header flex items-center justify-between">
            <h3 className="text-lg font-semibold text-gray-900">消息列表</h3>
            <div className="flex items-center gap-3">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="搜索消息..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                  className="pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 w-64"
                />
              </div>
              <button
                onClick={handleSearch}
                className="btn-primary btn-sm"
              >
                搜索
              </button>
              <button
                onClick={loadData}
                className="btn-secondary btn-sm"
              >
                <RefreshCw className="w-4 h-4" />
              </button>
            </div>
          </div>
          <div className="divide-y divide-gray-100 max-h-[600px] overflow-y-auto">
            {messages.length === 0 ? (
              <div className="p-8 text-center text-gray-500">
                暂无消息
              </div>
            ) : (
              messages.map((msg) => (
                <div key={msg.id} className="p-4 hover:bg-gray-50 transition-colors">
                  <div className="flex items-center gap-3 mb-2">
                    <span className="font-medium text-gray-900">{msg.sender}</span>
                    <span className="px-2 py-0.5 bg-gray-100 text-gray-600 text-xs rounded">
                      {msg.session}
                    </span>
                    <span className="text-xs text-gray-400">
                      {new Date(msg.timestamp).toLocaleString()}
                    </span>
                  </div>
                  <p className="text-gray-700 text-sm whitespace-pre-wrap">{msg.content}</p>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Apps List */}
        <div className="card">
          <div className="card-header">
            <h3 className="text-lg font-semibold text-gray-900">监听应用</h3>
          </div>
          <div className="divide-y divide-gray-100">
            {apps.length === 0 ? (
              <div className="p-8 text-center text-gray-500">
                暂无应用
              </div>
            ) : (
              apps.map((app) => (
                <div
                  key={app.name}
                  className="p-4 flex items-center justify-between hover:bg-gray-50 transition-colors"
                >
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
                </div>
              ))
            )}
          </div>
          <div className="card-footer">
            <span className="text-xs text-gray-400">每 5 秒自动刷新</span>
          </div>
        </div>
      </div>
    </div>
  )
}
