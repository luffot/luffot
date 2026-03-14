// Package builder 皮肤制作核心逻辑
package builder

// SkinState 皮肤状态枚举
type SkinState string

const (
	StateIdle     SkinState = "idle"     // 摸鱼/待机
	StateTyping   SkinState = "typing"   // 敲键盘
	StateThinking SkinState = "thinking" // AI 思考中
	StateTalking  SkinState = "talking"  // AI 说话中
	StateAlert    SkinState = "alert"    // 告警/惊讶
)

// AllStates 所有需要生成帧的状态（有序）
var AllStates = []SkinState{
	StateIdle,
	StateTyping,
	StateThinking,
	StateTalking,
	StateAlert,
}

// StateDisplayName 状态的中文显示名
var StateDisplayName = map[SkinState]string{
	StateIdle:     "待机（摸鱼）",
	StateTyping:   "敲键盘",
	StateThinking: "AI 思考中",
	StateTalking:  "AI 说话中",
	StateAlert:    "告警惊讶",
}

// StateDescription 状态的详细说明（用于引导用户）
var StateDescription = map[SkinState]string{
	StateIdle:     "桌宠平时待机时的表现，通常是轻微摇摆或呼吸感",
	StateTyping:   "检测到你在敲键盘时的表现，通常是兴奋、抖动或跳舞",
	StateThinking: "你向 AI 提问、AI 正在思考时的表现，通常是若有所思或转圈",
	StateTalking:  "AI 正在回复你时的表现，通常是嘴巴动或发光",
	StateAlert:    "收到紧急消息告警时的表现，通常是惊讶、跳起或发光",
}

// EffectType 特效类型
type EffectType string

const (
	EffectNone     EffectType = "none"     // 无特效，仅静态图
	EffectBob      EffectType = "bob"      // 上下浮动（呼吸感）
	EffectShake    EffectType = "shake"    // 左右抖动
	EffectSpin     EffectType = "spin"     // 旋转
	EffectPulse    EffectType = "pulse"    // 缩放脉冲
	EffectGlow     EffectType = "glow"     // 发光光晕
	EffectParticle EffectType = "particle" // 粒子特效（星星/音符）
	EffectRainbow  EffectType = "rainbow"  // 彩虹色调变换
	EffectFlash    EffectType = "flash"    // 闪烁
	EffectBounce   EffectType = "bounce"   // 弹跳
	EffectWave     EffectType = "wave"     // 波浪摆动
)

// EffectDisplayName 特效的中文显示名
var EffectDisplayName = map[EffectType]string{
	EffectNone:     "无特效（静态）",
	EffectBob:      "上下浮动（呼吸感）",
	EffectShake:    "左右抖动（兴奋）",
	EffectSpin:     "旋转",
	EffectPulse:    "缩放脉冲（心跳感）",
	EffectGlow:     "发光光晕",
	EffectParticle: "粒子特效（星星/音符飘散）",
	EffectRainbow:  "彩虹色调变换",
	EffectFlash:    "闪烁",
	EffectBounce:   "弹跳",
	EffectWave:     "波浪摆动",
}

// FullscreenEffectType 全屏特效类型（alert 状态专属）
type FullscreenEffectType string

const (
	FullscreenEffectNone       FullscreenEffectType = ""
	FullscreenEffectBasketball FullscreenEffectType = "basketball"
)

// StateConfig 单个状态的配置
type StateConfig struct {
	// Effect 主特效类型
	Effect EffectType

	// FrameCount 生成的帧数（1 = 静态，>1 = 动画）
	FrameCount int

	// EffectIntensity 特效强度 0.0~1.0
	EffectIntensity float64

	// ParticleType 粒子类型（仅 EffectParticle 时有效）
	ParticleType string // "star" / "note" / "heart" / "sparkle"

	// GlowColor 光晕颜色（仅 EffectGlow 时有效）
	GlowColor [3]uint8 // RGB

	// UserDescription 用户对该状态的原始描述（供 AI 解析）
	UserDescription string
}

// SkinBuildConfig 完整的皮肤制作配置
type SkinBuildConfig struct {
	// SkinName 皮肤名称
	SkinName string

	// ImagePath 原始图片路径
	ImagePath string

	// SpriteSize 输出精灵尺寸（宽=高，正方形）
	SpriteSize int

	// States 各状态配置
	States map[SkinState]*StateConfig

	// FullscreenEffect alert 状态的全屏特效
	FullscreenEffect FullscreenEffectType

	// BackgroundMode 背景去除模式
	// "removebg"：使用 remove.bg API（效果最佳）
	// "local"：使用本地洪水填充算法（效果有限）
	// ""：不去除背景
	BackgroundMode string
}

// NewSkinBuildConfig 创建默认皮肤制作配置
func NewSkinBuildConfig(skinName, imagePath string) *SkinBuildConfig {
	cfg := &SkinBuildConfig{
		SkinName:         skinName,
		ImagePath:        imagePath,
		SpriteSize:       160,
		States:           make(map[SkinState]*StateConfig),
		FullscreenEffect: FullscreenEffectNone,
		BackgroundMode:   "",
	}

	// 为每个状态设置默认配置
	for _, state := range AllStates {
		cfg.States[state] = &StateConfig{
			Effect:          EffectBob,
			FrameCount:      8,
			EffectIntensity: 0.5,
			ParticleType:    "star",
			GlowColor:       [3]uint8{255, 255, 255},
		}
	}

	// 各状态的合理默认值
	cfg.States[StateIdle].Effect = EffectBob
	cfg.States[StateIdle].FrameCount = 8

	cfg.States[StateTyping].Effect = EffectShake
	cfg.States[StateTyping].FrameCount = 6

	cfg.States[StateThinking].Effect = EffectSpin
	cfg.States[StateThinking].FrameCount = 12

	cfg.States[StateTalking].Effect = EffectPulse
	cfg.States[StateTalking].FrameCount = 8

	cfg.States[StateAlert].Effect = EffectBounce
	cfg.States[StateAlert].FrameCount = 10

	return cfg
}

// SkinMeta 皮肤元信息，写入 skin_meta.json
type SkinMeta struct {
	Name             string               `json:"name"`
	Version          string               `json:"version"`
	FullscreenEffect FullscreenEffectType `json:"fullscreen_effect,omitempty"`
	States           map[string]StateMeta `json:"states"`
}

// StateMeta 单个状态的元信息
type StateMeta struct {
	Effect     EffectType `json:"effect"`
	FrameCount int        `json:"frame_count"`
}
