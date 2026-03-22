import { Keyboard } from 'lucide-react'

export default function HotkeySettingsPage() {
  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">快捷键设置</h1>
        <p className="text-gray-500 mt-1">自定义快捷键绑定，提升操作效率</p>
      </div>

      {/* Placeholder */}
      <div className="card">
        <div className="card-body text-center py-16">
          <Keyboard className="w-16 h-16 text-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">快捷键设置即将上线</h3>
          <p className="text-gray-500 max-w-md mx-auto">
            快捷键设置将允许你重新配置已支持的快捷键，打造个性化的操作体验。
            敬请期待！
          </p>
        </div>
      </div>
    </div>
  )
}
