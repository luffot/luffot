# Luffot Wails 设置窗口

本项目已将原有的 Web 设置页面迁移到 Wails 框架，提供更原生的桌面应用体验。

## 功能特性

- **原生桌面窗口**：使用 Wails + React + TypeScript 构建，提供流畅的桌面应用体验
- **完整的设置功能**：保留了原有 Web 设置页面的所有功能
  - 消息总览：查看消息统计和列表
  - 模型配置：管理 AI Provider
  - 告警配置：设置关键词告警
  - 弹幕设置：过滤规则和特别关注
  - 提示词管理：编辑 AI 提示词
  - 皮肤配置：切换和导入皮肤
  - 进程监控：管理监听应用
  - 摄像头监测：设置和查看记录
  - 定时任务：查看和手动触发任务
  - ADK 配置：多 Agent 系统配置
  - Langfuse 配置：AI 调用追踪

## 构建和运行

### 前置要求

- Go 1.24+
- Node.js 18+
- Wails CLI (可选，用于开发)

### 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 开发模式

1. 进入前端目录并安装依赖：
```bash
cd frontend
npm install
```

2. 启动 Wails 开发服务器：
```bash
wails dev
```

### 生产构建

1. 构建前端：
```bash
cd frontend
npm install
npm run build
```

2. 构建完整应用：
```bash
wails build
```

## 使用方式

### 从托盘菜单打开

运行主应用后，点击系统托盘图标，选择「打开设置」即可启动 Wails 设置窗口。

### 独立运行（开发测试）

```bash
go run ./cmd/luffot
# 然后点击托盘菜单的「打开设置」
```

## 项目结构

```
.
├── cmd/luffot/           # 主应用入口
├── pkg/
│   ├── settings/         # Wails 后端 API
│   │   ├── app.go        # 应用逻辑和 API 绑定
│   │   └── wails.go      # Wails 启动配置
│   └── ...               # 其他包
├── frontend/             # Wails 前端项目
│   ├── src/
│   │   ├── components/   # React 组件
│   │   ├── pages/        # 页面组件
│   │   ├── lib/          # 工具库（Wails 桥接）
│   │   └── types/        # TypeScript 类型定义
│   └── ...
└── wails.json            # Wails 配置文件
```

## 技术栈

- **后端**: Go + Wails v2
- **前端**: React 18 + TypeScript + Tailwind CSS
- **图标**: Lucide React
- **路由**: React Router v6

## 迁移说明

原有的 Web 设置页面（`pkg/embedfs/static/web/`）仍然保留，可以通过浏览器访问：
- Web UI: http://127.0.0.1:8765
- 设置页面: http://127.0.0.1:8765/settings

Wails 设置窗口提供了更好的用户体验，建议优先使用。
