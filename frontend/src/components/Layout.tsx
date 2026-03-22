import { useState } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import {
  Bot,
  Bell,
  MessageSquare,
  Palette,
  Monitor,
  Camera,
  Clock,
  BarChart3,
  ChevronRight,
  ChevronDown,
  Settings,
  Sparkles,
  Zap,
  Keyboard,
  Cpu,
  LayoutDashboard,
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

/** 子菜单项（用于 NavGroup 内部） */
function SubNavItem({ to, label }: { to: string; label: string }) {
  return (
    <NavLink
      to={to}
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
      {label}
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

/** 分组标题 */
function SectionTitle({ title }: { title: string }) {
  return (
    <div className="pt-4 pb-2 first:pt-0">
      <p className="px-4 text-xs font-semibold text-gray-400 uppercase tracking-wider">
        {title}
      </p>
    </div>
  )
}

interface LayoutProps {
  children: React.ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const pathname = location.pathname

  const [cameraOpen, setCameraOpen] = useState(
    pathname.startsWith('/camera')
  )

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="w-64 bg-white border-r border-gray-200 flex flex-col">
        {/* Draggable title bar area - for macOS traffic lights */}
        <div
          className="h-12 flex items-center px-6 border-b border-gray-200 wails-drag"
          style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
        >
          {/* 左侧留出红绿灯按钮空间 */}
          <div className="flex items-center gap-2 pl-16">
            <div className="w-7 h-7 bg-primary-600 rounded-lg flex items-center justify-center">
              <Settings className="w-4 h-4 text-white" />
            </div>
            <span className="text-base font-semibold text-gray-900">Luffot</span>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto p-4 space-y-1">
          {/* ==================== 智能设置 ==================== */}
          <SectionTitle title="智能设置" />

          <NavItem
            to="/agent-persona"
            icon={<Sparkles className="w-4 h-4" />}
            label="基础设置"
          />
          <NavItem
            to="/skin"
            icon={<Palette className="w-4 h-4" />}
            label="皮肤设置"
          />
          <NavItem
            to="/skill-center"
            icon={<Zap className="w-4 h-4" />}
            label="技能中心"
          />

          {/* ==================== 功能设置 ==================== */}
          <SectionTitle title="功能设置" />

          <NavItem
            to="/process-monitor"
            icon={<Monitor className="w-4 h-4" />}
            label="进程监控"
          />

          <NavGroup
            icon={<Camera className="w-4 h-4" />}
            label="环境监测"
            isOpen={cameraOpen}
            onToggle={() => setCameraOpen(!cameraOpen)}
            isActive={pathname.startsWith('/camera')}
          >
            <SubNavItem to="/camera-settings" label="监测设置" />
            <SubNavItem to="/camera-log" label="监测记录" />
          </NavGroup>

          <NavItem
            to="/barrage"
            icon={<MessageSquare className="w-4 h-4" />}
            label="弹幕设置"
          />

          <NavItem
            to="/alert"
            icon={<Bell className="w-4 h-4" />}
            label="告警配置"
          />

          <NavItem
            to="/messages"
            icon={<LayoutDashboard className="w-4 h-4" />}
            label="消息总览"
          />

          {/* ==================== 系统设置 ==================== */}
          <SectionTitle title="系统设置" />

          <NavItem
            to="/model"
            icon={<Bot className="w-4 h-4" />}
            label="模型服务设置"
          />
          <NavItem
            to="/langfuse"
            icon={<BarChart3 className="w-4 h-4" />}
            label="可观测配置"
          />
          <NavItem
            to="/adk"
            icon={<Cpu className="w-4 h-4" />}
            label="ADK 配置"
          />
          <NavItem
            to="/tasks"
            icon={<Clock className="w-4 h-4" />}
            label="定时任务"
          />
          <NavItem
            to="/hotkeys"
            icon={<Keyboard className="w-4 h-4" />}
            label="快捷键"
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
      <main className="flex-1 overflow-auto flex flex-col">
        {/* 右侧顶部拖拽区域 */}
        <div
          className="h-12 flex-shrink-0 wails-drag"
          style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
        />
        <div className="flex-1 overflow-auto p-8 max-w-7xl mx-auto w-full">
          {children}
        </div>
      </main>
    </div>
  )
}
