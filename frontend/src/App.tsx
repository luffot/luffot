import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import AgentPersonaPage from './pages/AgentPersonaPage'
import SkinPage from './pages/SkinPage'
import SkillCenterPage from './pages/SkillCenterPage'
import ProcessMonitorPage from './pages/ProcessMonitorPage'
import CameraSettingsPage from './pages/CameraSettingsPage'
import CameraLogPage from './pages/CameraLogPage'
import BarrageSettingsPage from './pages/BarrageSettingsPage'
import AlertConfigPage from './pages/AlertConfigPage'
import MessagesPage from './pages/MessagesPage'
import ModelConfigPage from './pages/ModelConfigPage'
import LangfuseConfigPage from './pages/LangfuseConfigPage'
import ADKConfigPage from './pages/ADKConfigPage'
import TasksPage from './pages/TasksPage'
import HotkeySettingsPage from './pages/HotkeySettingsPage'

function App() {
  return (
    <Layout>
      <Routes>
        {/* 默认首页 → 基础设置 */}
        <Route path="/" element={<AgentPersonaPage />} />

        {/* 智能设置 */}
        <Route path="/agent-persona" element={<AgentPersonaPage />} />
        <Route path="/skin" element={<SkinPage />} />
        <Route path="/skill-center" element={<SkillCenterPage />} />

        {/* 功能设置 */}
        <Route path="/process-monitor" element={<ProcessMonitorPage />} />
        <Route path="/camera-settings" element={<CameraSettingsPage />} />
        <Route path="/camera-log" element={<CameraLogPage />} />
        <Route path="/barrage" element={<BarrageSettingsPage />} />
        <Route path="/alert" element={<AlertConfigPage />} />
        <Route path="/messages" element={<MessagesPage />} />

        {/* 系统设置 */}
        <Route path="/model" element={<ModelConfigPage />} />
        <Route path="/langfuse" element={<LangfuseConfigPage />} />
        <Route path="/adk" element={<ADKConfigPage />} />
        <Route path="/tasks" element={<TasksPage />} />
        <Route path="/hotkeys" element={<HotkeySettingsPage />} />
      </Routes>
    </Layout>
  )
}

export default App
