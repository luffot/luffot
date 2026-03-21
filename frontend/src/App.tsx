import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import MessagesPage from './pages/MessagesPage'
import ModelConfigPage from './pages/ModelConfigPage'
import AlertConfigPage from './pages/AlertConfigPage'
import BarrageFilterPage from './pages/BarrageFilterPage'
import BarrageHighlightPage from './pages/BarrageHighlightPage'
import PromptPage from './pages/PromptPage'
import SkinPage from './pages/SkinPage'
import ProcessMonitorPage from './pages/ProcessMonitorPage'
import CameraSettingsPage from './pages/CameraSettingsPage'
import CameraLogPage from './pages/CameraLogPage'
import TasksPage from './pages/TasksPage'
import ADKConfigPage from './pages/ADKConfigPage'
import LangfuseConfigPage from './pages/LangfuseConfigPage'

function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<MessagesPage />} />
        <Route path="/messages" element={<MessagesPage />} />
        <Route path="/model" element={<ModelConfigPage />} />
        <Route path="/alert" element={<AlertConfigPage />} />
        <Route path="/barrage-filter" element={<BarrageFilterPage />} />
        <Route path="/barrage-highlight" element={<BarrageHighlightPage />} />
        <Route path="/prompt" element={<PromptPage />} />
        <Route path="/skin" element={<SkinPage />} />
        <Route path="/process-monitor" element={<ProcessMonitorPage />} />
        <Route path="/camera-settings" element={<CameraSettingsPage />} />
        <Route path="/camera-log" element={<CameraLogPage />} />
        <Route path="/tasks" element={<TasksPage />} />
        <Route path="/adk" element={<ADKConfigPage />} />
        <Route path="/langfuse" element={<LangfuseConfigPage />} />
      </Routes>
    </Layout>
  )
}

export default App
