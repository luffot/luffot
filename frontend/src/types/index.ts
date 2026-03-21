// 消息类型
export interface Message {
  id: number;
  app: string;
  sender: string;
  session: string;
  content: string;
  timestamp: string;
}

export interface MessageStats {
  total_messages: number;
  today_messages: number;
  app_counts: Record<string, number>;
}

// 应用配置
export interface AppConfigItem {
  name: string;
  process_name: string;
  display_name: string;
  enabled: boolean;
  icon_path?: string;
  parse_rules: ParseRules;
  session_config: SessionConfig;
  dingtalk?: DingTalkConfig;
  process_monitor?: ProcessMonitorConfig;
}

export interface ParseRules {
  sender_pattern: string;
  time_pattern: string;
  content_mode: string;
  dedup_enabled: boolean;
}

export interface SessionConfig {
  source: string;
  fixed_name?: string;
  script?: string;
}

export interface DingTalkConfig {
  source_mode: string;
}

export interface ProcessMonitorConfig {
  use_vlmodel: boolean;
  vlmodel_provider?: string;
  vlmodel_prompt?: string;
}

// AI 配置
export interface AIProviderConfig {
  name: string;
  provider: string;
  api_key: string;
  model: string;
  base_url?: string;
  timeout_seconds?: number;
}

export interface AIConfig {
  enabled: boolean;
  default_provider: string;
  max_context_rounds: number;
  timeout_seconds: number;
  providers: AIProviderConfig[];
}

// 告警配置
export interface AlertConfig {
  enabled: boolean;
  keywords: string[];
  filter_keywords: string[];
}

// 弹幕配置
export interface BarrageHighlightRule {
  keyword: string;
  color: string;
}

export interface BarrageConfig {
  filter_keywords: string[];
  highlight_rules: BarrageHighlightRule[];
}

// 摄像头守卫配置
export interface CameraGuardConfig {
  enabled: boolean;
  interval_seconds: number;
  provider_name: string;
  confirm_count: number;
  cooldown_seconds: number;
}

export interface CameraDetection {
  id: number;
  detected_at: string;
  image_url: string;
  ai_reason: string;
}

// 皮肤配置
export interface Skin {
  name: string;
  internal: string;
  description: string;
  type: 'vector' | 'image' | 'lua';
}

export interface SkinListResponse {
  current_skin: string;
  skins: Skin[];
}

// 提示词
export interface Prompt {
  name: string;
  content: string;
  updated_at?: string;
}

// 定时任务
export interface ScheduledTask {
  name: string;
  description?: string;
  enabled: boolean;
  cron: string;
  type: string;
  builtin_name?: string;
  script_path?: string;
  script_args?: string[];
  provider_name?: string;
  last_run?: string;
  next_run?: string;
  run_count: number;
}

// Langfuse 配置
export interface LangfuseConfig {
  enabled: boolean;
  public_key: string;
  secret_key: string;
  base_url: string;
  async_enabled: boolean;
  batch_size: number;
  flush_interval: number;
}

// ADK 配置
export interface ADKAgent {
  name: string;
  role?: string;
  model_provider?: string;
  system_prompt?: string;
}

export interface ADKConfig {
  system_name?: string;
  log_level?: string;
  agents?: ADKAgent[];
  skills_dir?: string;
  skills_autoload?: boolean;
  memory_type?: string;
  memory_max_history?: number;
}

// 导航项
export interface NavItem {
  id: string;
  label: string;
  icon: string;
  path: string;
  children?: NavItem[];
}
