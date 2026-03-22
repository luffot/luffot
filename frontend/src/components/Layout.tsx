import { useState } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  Bot,
  Bell,
  MessageSquare,
  Palette,
  Monitor,
  Camera,
  Clock,
  Brain,
  BarChart3,
  ChevronRight,
  ChevronDown,
  Settings,
  Sparkles,
} from 'lucide-react'
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

interface NavItemProps {
  to: string
  icon: React.ReactNode
  label: string
  end?: boolean
}

function NavItem({ to, icon, label, end }: NavItemProps) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cn(
          'flex items-center gap-3 px-4 py-2.5 text-sm font-medium rounded-lg transition-all duration-200',
          isActive
            ? 'bg-primary-50 text-primary-700 border-l-2 border-primary-600'
            : 'text-gray-600 hover:bg-gray-100'
        )
      }
    >
      {icon}
      <span>{label}</span>
    </NavLink>
  )
}

interface NavGroupProps {
  icon: React.ReactNode
  label: string
  children: React.ReactNode
  isOpen: boolean
  onToggle: () => void
  isActive: boolean
}

function NavGroup({ icon, label, children, isOpen, onToggle, isActive }: NavGroupProps) {
  return (
    <div>
      <button
        onClick={onToggle}
        className={cn(
          'w-full flex items-center justify-between px-4 py-2.5 text-sm font-medium rounded-lg transition-all duration-200',
          isActive ? 'text-primary-700 bg-primary-50/50' : 'text-gray-600 hover:bg-gray-100'
        )}
      >
        <div className="flex items-center gap-3">
          {icon}
          <span>{label}</span>
        </div>
        {isOpen ? (
          <ChevronDown className="w-4 h-4" />
        ) : (
          <ChevronRight className="w-4 h-4" />
        )}
      </button>
      {isOpen && (
        <div className="ml-4 mt-1 space-y-1 animate-fade-in">
          {children}
        </div>
      )}
    </div>
  )
}

interface LayoutProps {
  children: React.ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const [barrageOpen, setBarrageOpen] = useState(
    location.pathname.startsWith('/barrage')
  )
  const [cameraOpen, setCameraOpen] = useState(
    location.pathname.startsWith('/camera')
  )

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="w-64 bg-white border-r border-gray-200 flex flex-col">
        {/* Header */}
        <div className="h-16 flex items-center px-6 border-b border-gray-200">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-primary-600 rounded-lg flex items-center justify-center">
              <Settings className="w-5 h-5 text-white" />
            </div>
            <span className="text-lg font-semibold text-gray-900">Luffot</span>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto p-4 space-y-1">
          <div className="pb-2">
            <p className="px-4 text-xs font-semibold text-gray-400 uppercase tracking-wider">
              智能体
            </p>
          </div>

          <NavItem
            to="/agent-persona"
            icon={<Sparkles className="w-4 h-4" />}
            label="智能体人设"
          />
          <NavItem
            to="/model"
            icon={<Bot className="w-4 h-4" />}
            label="模型配置"
          />

          <div className="pt-4 pb-2">
            <p className="px-4 text-xs font-semibold text-gray-400 uppercase tracking-wider">
              数据
            </p>
          </div>

          <NavItem
            to="/messages"
            icon={<LayoutDashboard className="w-4 h-4" />}
            label="消息总览"
          />

          <div className="pt-4 pb-2">
            <p className="px-4 text-xs font-semibold text-gray-400 uppercase tracking-wider">
              配置管理
            </p>
          </div>

          <NavItem
            to="/alert"
            icon={<Bell className="w-4 h-4" />}
            label="告警配置"
          />

          <NavGroup
            icon={<MessageSquare className="w-4 h-4" />}
            label="弹幕设置"
            isOpen={barrageOpen}
            onToggle={() => setBarrageOpen(!barrageOpen)}
            isActive={location.pathname.startsWith('/barrage')}
          >
            <NavLink
              to="/barrage-filter"
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2 px-4 py-2 text-sm rounded-lg transition-colors',
                  isActive
                    ? 'text-primary-700 bg-primary-50'
                    : 'text-gray-600 hover:bg-gray-100'
                )
              }
            >
              <span className="w-1.5 h-1.5 rounded-full bg-current" />
              弹幕过滤
            </NavLink>
            <NavLink
              to="/barrage-highlight"
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2 px-4 py-2 text-sm rounded-lg transition-colors',
                  isActive
                    ? 'text-primary-700 bg-primary-50'
                    : 'text-gray-600 hover:bg-gray-100'
                )
              }
            >
              <span className="w-1.5 h-1.5 rounded-full bg-current" />
              特别关注
            </NavLink>
          </NavGroup>

          <NavItem
            to="/process-monitor"
            icon={<Monitor className="w-4 h-4" />}
            label="进程监控"
          />
          <NavItem
            to="/skin"
            icon={<Palette className="w-4 h-4" />}
            label="皮肤配置"
          />
          <NavItem
            to="/adk"
            icon={<Brain className="w-4 h-4" />}
            label="ADK 配置"
          />
          <NavItem
            to="/langfuse"
            icon={<BarChart3 className="w-4 h-4" />}
            label="Langfuse 配置"
          />

          <div className="pt-4 pb-2">
            <p className="px-4 text-xs font-semibold text-gray-400 uppercase tracking-wider">
              环境监测
            </p>
          </div>

          <NavGroup
            icon={<Camera className="w-4 h-4" />}
            label="摄像头监测"
            isOpen={cameraOpen}
            onToggle={() => setCameraOpen(!cameraOpen)}
            isActive={location.pathname.startsWith('/camera')}
          >
            <NavLink
              to="/camera-settings"
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2 px-4 py-2 text-sm rounded-lg transition-colors',
                  isActive
                    ? 'text-primary-700 bg-primary-50'
                    : 'text-gray-600 hover:bg-gray-100'
                )
              }
            >
              <span className="w-1.5 h-1.5 rounded-full bg-current" />
              监测设置
            </NavLink>
            <NavLink
              to="/camera-log"
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2 px-4 py-2 text-sm rounded-lg transition-colors',
                  isActive
                    ? 'text-primary-700 bg-primary-50'
                    : 'text-gray-600 hover:bg-gray-100'
                )
              }
            >
              <span className="w-1.5 h-1.5 rounded-full bg-current" />
              监测记录
            </NavLink>
          </NavGroup>

          <div className="pt-4 pb-2">
            <p className="px-4 text-xs font-semibold text-gray-400 uppercase tracking-wider">
              系统
            </p>
          </div>

          <NavItem
            to="/tasks"
            icon={<Clock className="w-4 h-4" />}
            label="定时任务"
          />
        </nav>

        {/* Footer */}
        <div className="p-4 border-t border-gray-200">
          <div className="flex items-center justify-between text-xs text-gray-500">
            <span>v1.0.0</span>
            <span className="flex items-center gap-1">
              <span className="w-2 h-2 rounded-full bg-green-500" />
              运行中
            </span>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-auto">
        <div className="p-8 max-w-7xl mx-auto">
          {children}
        </div>
      </main>
    </div>
  )
}
