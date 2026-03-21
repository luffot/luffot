// Wails 运行时桥接
// 在 Wails 环境中使用 window.go 对象，在开发环境中使用模拟数据

const isWails = typeof window !== 'undefined' && (window as any).go !== undefined;

// 模拟数据（开发环境使用）
const mockData = {
  aiConfig: {
    enabled: true,
    default_provider: 'default',
    max_context_rounds: 10,
    timeout_seconds: 30,
    providers: [
      {
        name: 'default',
        provider: 'bailian',
        api_key: '',
        model: 'qwen-plus',
        base_url: '',
        timeout_seconds: 0,
      },
    ],
  },
  alertConfig: {
    enabled: true,
    keywords: ['紧急', '故障', '告警', '线上问题'],
    filter_keywords: ['测试群', '机器人'],
  },
  barrageConfig: {
    filter_keywords: ['广告', '机器人'],
    highlight_rules: [
      { keyword: '重要', color: '#FFD700' },
    ],
  },
  cameraGuardConfig: {
    enabled: false,
    interval_seconds: 30,
    provider_name: 'vision',
    confirm_count: 2,
    cooldown_seconds: 60,
  },
  skins: {
    current_skin: '',
    skins: [
      { name: '经典皮肤', internal: '', description: '钉三多经典黑，原汁原味的矢量小蜜蜂', type: 'vector' },
      { name: '星空蓝', internal: '星空蓝', description: '深邃星空配色', type: 'vector' },
      { name: '暗金', internal: '暗金', description: '低调奢华暗金风', type: 'vector' },
      { name: '樱花粉', internal: '樱花粉', description: '清新樱花粉嫩风', type: 'vector' },
    ],
  },
  messageStats: {
    total_messages: 1234,
    today_messages: 56,
    app_counts: { dingtalk: 800, wechat: 434 },
  },
  apps: {
    apps: [
      { name: 'dingtalk', process_name: 'DingTalk', display_name: '钉钉', enabled: true, parse_rules: { sender_pattern: '(\\S+)\\s+\\d{1,2}:\\d{2}', time_pattern: '\\d{1,2}:\\d{2}', content_mode: 'after_time', dedup_enabled: true }, session_config: { source: 'window_title' } },
      { name: 'wechat', process_name: 'WeChat', display_name: '微信', enabled: true, parse_rules: { sender_pattern: '(\\S+)\\s+\\d{1,2}:\\d{2}', time_pattern: '\\d{1,2}:\\d{2}', content_mode: 'after_time', dedup_enabled: true }, session_config: { source: 'window_title' } },
    ],
  },
  tasks: {
    tasks: [],
  },
  langfuseConfig: {
    enabled: false,
    public_key: '',
    secret_key: '',
    base_url: 'https://cloud.langfuse.com',
    async_enabled: true,
    batch_size: 100,
    flush_interval: 5,
  },
  prompts: {
    prompts: [],
    dir: '~/.luffot/prompt/',
  },
  cameraDetections: {
    detections: [],
    total: 0,
  },
};

// API 封装
export const wailsAPI = {
  // AI 配置
  async getAIConfig() {
    if (isWails) {
      return (window as any).go.settings.App.GetAIConfig();
    }
    return mockData.aiConfig;
  },
  
  async saveAIConfig(config: any) {
    if (isWails) {
      return (window as any).go.settings.App.SaveAIConfig(config);
    }
    return { success: true };
  },
  
  // 告警配置
  async getAlertConfig() {
    if (isWails) {
      return (window as any).go.settings.App.GetAlertConfig();
    }
    return mockData.alertConfig;
  },
  
  async saveAlertConfig(config: any) {
    if (isWails) {
      return (window as any).go.settings.App.SaveAlertConfig(config);
    }
    return { success: true };
  },
  
  // 弹幕配置
  async getBarrageConfig() {
    if (isWails) {
      return (window as any).go.settings.App.GetBarrageConfig();
    }
    return mockData.barrageConfig;
  },
  
  async saveBarrageConfig(config: any) {
    if (isWails) {
      return (window as any).go.settings.App.SaveBarrageConfig(config);
    }
    return { success: true };
  },
  
  // 摄像头守卫配置
  async getCameraGuardConfig() {
    if (isWails) {
      return (window as any).go.settings.App.GetCameraGuardConfig();
    }
    return mockData.cameraGuardConfig;
  },
  
  async saveCameraGuardConfig(config: any) {
    if (isWails) {
      return (window as any).go.settings.App.SaveCameraGuardConfig(config);
    }
    return { success: true };
  },
  
  async getCameraDetections(limit: number = 20, offset: number = 0) {
    if (isWails) {
      return (window as any).go.settings.App.GetCameraDetections(limit, offset);
    }
    return mockData.cameraDetections;
  },
  
  // 皮肤配置
  async getSkins() {
    if (isWails) {
      return (window as any).go.settings.App.GetSkins();
    }
    return mockData.skins;
  },
  
  async setSkin(skinName: string) {
    if (isWails) {
      return (window as any).go.settings.App.SetSkin(skinName);
    }
    return { success: true };
  },
  
  async importSkin(dir: string, name: string) {
    if (isWails) {
      return (window as any).go.settings.App.ImportSkin(dir, name);
    }
    return { success: true, skin_name: name || '新皮肤' };
  },
  
  // 消息相关
  async getMessageStats() {
    if (isWails) {
      return (window as any).go.settings.App.GetMessageStats();
    }
    return mockData.messageStats;
  },
  
  async getMessages(app: string = '', limit: number = 50, offset: number = 0) {
    if (isWails) {
      return (window as any).go.settings.App.GetMessages(app, limit, offset);
    }
    return { messages: [], total: 0 };
  },
  
  async searchMessages(keyword: string, app: string = '', limit: number = 50) {
    if (isWails) {
      return (window as any).go.settings.App.SearchMessages(keyword, app, limit);
    }
    return { messages: [], total: 0 };
  },
  
  // 应用配置
  async getApps() {
    if (isWails) {
      return (window as any).go.settings.App.GetApps();
    }
    return mockData.apps;
  },
  
  async saveAppConfig(name: string, config: any) {
    if (isWails) {
      return (window as any).go.settings.App.SaveAppConfig(name, config);
    }
    return { success: true };
  },
  
  async addAppConfig(config: any) {
    if (isWails) {
      return (window as any).go.settings.App.AddAppConfig(config);
    }
    return { success: true };
  },
  
  async removeAppConfig(name: string) {
    if (isWails) {
      return (window as any).go.settings.App.RemoveAppConfig(name);
    }
    return { success: true };
  },
  
  // 提示词管理
  async getPrompts() {
    if (isWails) {
      return (window as any).go.settings.App.GetPrompts();
    }
    return mockData.prompts;
  },
  
  async getPrompt(name: string) {
    if (isWails) {
      return (window as any).go.settings.App.GetPrompt(name);
    }
    return { name, content: '' };
  },
  
  async savePrompt(name: string, content: string) {
    if (isWails) {
      return (window as any).go.settings.App.SavePrompt(name, content);
    }
    return { success: true };
  },
  
  // 定时任务
  async getTasks() {
    if (isWails) {
      return (window as any).go.settings.App.GetTasks();
    }
    return mockData.tasks;
  },
  
  async triggerTask(name: string) {
    if (isWails) {
      return (window as any).go.settings.App.TriggerTask(name);
    }
    return { success: true };
  },
  
  // Langfuse 配置
  async getLangfuseConfig() {
    if (isWails) {
      return (window as any).go.settings.App.GetLangfuseConfig();
    }
    return mockData.langfuseConfig;
  },
  
  async saveLangfuseConfig(config: any) {
    if (isWails) {
      return (window as any).go.settings.App.SaveLangfuseConfig(config);
    }
    return { success: true };
  },
  
  // ADK 配置
  async getADKConfig() {
    if (isWails) {
      return (window as any).go.settings.App.GetADKConfig();
    }
    return { enabled: false };
  },
  
  async saveADKConfig(config: any) {
    if (isWails) {
      return (window as any).go.settings.App.SaveADKConfig(config);
    }
    return { success: true };
  },
  
  // 通用配置
  async getFullConfig() {
    if (isWails) {
      return (window as any).go.settings.App.GetFullConfig();
    }
    return {};
  },
  
  async reloadConfig() {
    if (isWails) {
      return (window as any).go.settings.App.ReloadConfig();
    }
    return { success: true };
  },
};

export default wailsAPI;
