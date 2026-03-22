import { useState, useEffect } from 'react'
import { Save, AlertCircle, User, Bot, ChevronDown, ChevronRight, RotateCcw } from 'lucide-react'
import { wailsAPI } from '../lib/wails'

/** 智能体分组定义：将 KnownPrompts 按业务语义分组展示 */
interface AgentGroup {
  label: string
  description: string
  prompts: AgentPromptMeta[]
}

interface AgentPromptMeta {
  name: string
  displayName: string
  description: string
}

const AGENT_GROUPS: AgentGroup[] = [
  {
    label: '🐦 主对话智能体',
    description: '桌宠小钉的核心人设，决定了它和你聊天时的性格与风格',
    prompts: [
      { name: 'agent_system', displayName: '小钉人设（System Prompt）', description: '定义 AI 桌宠「小钉」的性格特点和说话风格' },
    ],
  },
  {
    label: '📊 消息分析智能体',
    description: '负责从消息流中筛选重要信息并推送通知',
    prompts: [
      { name: 'analyzer_importance_system', displayName: '消息重要性分析（System）', description: '消息重要性分析助手的角色定义' },
      { name: 'analyzer_importance_user', displayName: '消息重要性分析（User 模板）', description: '判断消息是否重要的分析模板，支持 {{profile}}、{{messages}} 占位符' },
    ],
  },
  {
    label: '🧠 用户画像智能体',
    description: '从消息中提炼用户特征，维护长期记忆与个人画像',
    prompts: [
      { name: 'analyzer_profile_system', displayName: '用户画像分析（System）', description: '用户画像分析助手的角色定义' },
      { name: 'analyzer_profile_user', displayName: '用户画像与记忆更新（User 模板）', description: '根据消息内容更新画像和记忆的模板，支持多种占位符' },
    ],
  },
  {
    label: '👁️ 环境感知智能体',
    description: '通过摄像头和进程监控感知用户所处环境',
    prompts: [
      { name: 'camera_guard', displayName: '摄像头守卫检测指令', description: '发给视觉 AI 的背后有人检测 Prompt' },
      { name: 'vlmodel_message_extract', displayName: 'VLModel 消息识别指令', description: '用于进程监控的视觉模型消息识别 Prompt' },
    ],
  },
]

export default function AgentPersonaPage() {
  // 用户画像
  const [userProfile, setUserProfile] = useState('')
  const [profileLoading, setProfileLoading] = useState(true)

  // 智能体提示词
  const [promptContents, setPromptContents] = useState<Record<string, string>>({})
  const [promptsLoading, setPromptsLoading] = useState(true)

  // 展开状态
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({
    '🐦 主对话智能体': true,
  })
  const [expandedPrompts, setExpandedPrompts] = useState<Record<string, boolean>>({
    'agent_system': true,
  })

  // 保存状态
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  useEffect(() => {
    loadUserProfile()
    loadAllPrompts()
  }, [])

  const loadUserProfile = async () => {
    setProfileLoading(true)
    try {
      const data = await wailsAPI.getUserProfile()
      setUserProfile(data?.content || '')
    } catch (error) {
      console.error('Failed to load user profile:', error)
    } finally {
      setProfileLoading(false)
    }
  }

  const loadAllPrompts = async () => {
    setPromptsLoading(true)
    try {
      const allPromptNames = AGENT_GROUPS.flatMap(group => group.prompts.map(p => p.name))
      const contents: Record<string, string> = {}
      for (const name of allPromptNames) {
        try {
          const data = await wailsAPI.getPrompt(name)
          contents[name] = typeof data === 'string' ? data : data?.content || ''
        } catch {
          contents[name] = ''
        }
      }
      setPromptContents(contents)
    } catch (error) {
      console.error('Failed to load prompts:', error)
    } finally {
      setPromptsLoading(false)
    }
  }

  const toggleGroup = (label: string) => {
    setExpandedGroups(prev => ({ ...prev, [label]: !prev[label] }))
  }

  const togglePrompt = (name: string) => {
    setExpandedPrompts(prev => ({ ...prev, [name]: !prev[name] }))
  }

  const updatePromptContent = (name: string, content: string) => {
    setPromptContents(prev => ({ ...prev, [name]: content }))
  }

  const handleSaveAll = async () => {
    setSaving(true)
    try {
      // 保存用户画像
      await wailsAPI.saveUserProfile(userProfile)

      // 保存所有提示词
      for (const [name, content] of Object.entries(promptContents)) {
        await wailsAPI.savePrompt(name, content)
      }

      setMessage({ type: 'success', text: '所有设置已保存，配置立即生效 ✨' })
    } catch (error) {
      setMessage({ type: 'error', text: '保存失败：' + String(error) })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const handleResetPrompt = async (name: string) => {
    try {
      // 删除自定义文件后重新加载，prompt.Load 会自动回退到内置默认值
      await wailsAPI.savePrompt(name, '')
      const data = await wailsAPI.getPrompt(name)
      const content = typeof data === 'string' ? data : data?.content || ''
      updatePromptContent(name, content)
      setMessage({ type: 'success', text: `已重置「${name}」为默认值` })
      setTimeout(() => setMessage(null), 3000)
    } catch (error) {
      setMessage({ type: 'error', text: '重置失败：' + String(error) })
      setTimeout(() => setMessage(null), 3000)
    }
  }

  if (profileLoading || promptsLoading) {
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
        <h1 className="text-2xl font-bold text-gray-900">智能体人设</h1>
        <p className="text-gray-500 mt-1">
          配置你的个人信息和各智能体的系统提示词，让 Luffot 更懂你
        </p>
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

      {/* ==================== 用户基础信息 ==================== */}
      <div className="card">
        <div className="card-header">
          <User className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">我的基础信息</h3>
        </div>
        <div className="card-body space-y-4">
          <div className="p-4 bg-amber-50 border border-amber-200 rounded-lg">
            <p className="text-sm text-amber-800">
              💡 以第一人称介绍你自己，这些信息会注入到所有智能体的上下文中，让它们更了解你。
              例如：我叫张三，是一名前端工程师，在 XX 团队负责 XX 项目……
            </p>
          </div>
          <div className="form-group mb-0">
            <label className="form-label">个人介绍</label>
            <textarea
              value={userProfile}
              onChange={(e) => setUserProfile(e.target.value)}
              placeholder={"我叫……，是一名……\n我目前在……团队，主要负责……\n我的工作习惯是……\n我比较关注……领域"}
              className="form-textarea min-h-[160px]"
              rows={7}
            />
            <p className="text-xs text-gray-500 mt-1">
              此信息存储在本地 <code className="px-1 py-0.5 bg-gray-100 rounded text-xs">~/.luffot/.my_profile</code>，仅用于 AI 上下文注入，不会上传到任何服务器
            </p>
          </div>
        </div>
      </div>

      {/* ==================== 多智能体提示词设置 ==================== */}
      <div className="card">
        <div className="card-header">
          <Bot className="w-5 h-5 text-primary-600" />
          <h3 className="text-lg font-semibold text-gray-900">多智能体提示词</h3>
          <span className="ml-auto text-xs text-gray-500">
            共 {AGENT_GROUPS.reduce((sum, g) => sum + g.prompts.length, 0)} 个提示词
          </span>
        </div>
        <div className="card-body space-y-2">
          <p className="text-sm text-gray-500 mb-4">
            Luffot 由多个专职智能体协作运行，每个智能体有独立的系统提示词。修改后立即生效，无需重启。
          </p>

          {AGENT_GROUPS.map((group) => {
            const isGroupExpanded = expandedGroups[group.label] ?? false
            return (
              <div key={group.label} className="border border-gray-200 rounded-lg overflow-hidden">
                {/* 分组标题 */}
                <button
                  onClick={() => toggleGroup(group.label)}
                  className="w-full flex items-center justify-between px-4 py-3 bg-gray-50 hover:bg-gray-100 transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-gray-900">{group.label}</span>
                    <span className="text-xs text-gray-500">({group.prompts.length} 个提示词)</span>
                  </div>
                  {isGroupExpanded ? (
                    <ChevronDown className="w-4 h-4 text-gray-500" />
                  ) : (
                    <ChevronRight className="w-4 h-4 text-gray-500" />
                  )}
                </button>

                {isGroupExpanded && (
                  <div className="p-4 space-y-4">
                    <p className="text-sm text-gray-500">{group.description}</p>

                    {group.prompts.map((promptMeta) => {
                      const isPromptExpanded = expandedPrompts[promptMeta.name] ?? false
                      return (
                        <div key={promptMeta.name} className="border border-gray-100 rounded-lg">
                          {/* 提示词标题 */}
                          <button
                            onClick={() => togglePrompt(promptMeta.name)}
                            className="w-full flex items-center justify-between px-4 py-2.5 hover:bg-gray-50 transition-colors"
                          >
                            <div className="text-left">
                              <div className="font-medium text-sm text-gray-800">{promptMeta.displayName}</div>
                              <div className="text-xs text-gray-500 mt-0.5">{promptMeta.description}</div>
                            </div>
                            {isPromptExpanded ? (
                              <ChevronDown className="w-4 h-4 text-gray-400 flex-shrink-0" />
                            ) : (
                              <ChevronRight className="w-4 h-4 text-gray-400 flex-shrink-0" />
                            )}
                          </button>

                          {isPromptExpanded && (
                            <div className="px-4 pb-4 space-y-2">
                              <div className="flex items-center justify-end">
                                <button
                                  onClick={() => handleResetPrompt(promptMeta.name)}
                                  className="text-xs text-gray-500 hover:text-primary-600 flex items-center gap-1 transition-colors"
                                >
                                  <RotateCcw className="w-3 h-3" />
                                  恢复默认
                                </button>
                              </div>
                              <textarea
                                value={promptContents[promptMeta.name] || ''}
                                onChange={(e) => updatePromptContent(promptMeta.name, e.target.value)}
                                className="form-textarea font-mono text-sm min-h-[200px]"
                                rows={10}
                                placeholder="输入提示词内容..."
                              />
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>

      {/* Save Button */}
      <div className="flex justify-end">
        <button
          onClick={handleSaveAll}
          disabled={saving}
          className="btn-primary"
        >
          <Save className="w-4 h-4" />
          {saving ? '保存中...' : '保存所有设置'}
        </button>
      </div>
    </div>
  )
}
