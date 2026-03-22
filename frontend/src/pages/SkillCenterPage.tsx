import { Zap } from 'lucide-react'

export default function SkillCenterPage() {
  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">技能中心</h1>
        <p className="text-gray-500 mt-1">创建和管理智能体技能，扩展 Luffot 的能力边界</p>
      </div>

      {/* Placeholder */}
      <div className="card">
        <div className="card-body text-center py-16">
          <Zap className="w-16 h-16 text-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">技能中心即将上线</h3>
          <p className="text-gray-500 max-w-md mx-auto">
            技能中心将支持创建、管理和分享自定义技能，让你的桌宠拥有更多能力。
            敬请期待！
          </p>
        </div>
      </div>
    </div>
  )
}
