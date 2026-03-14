package builder

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Session 皮肤制作会话，负责引导用户完成配置收集
type Session struct {
	imagePath   string
	skinName    string
	scanner     *bufio.Scanner
	config      *SkinBuildConfig
	hasRemoveBG bool // remove.bg API 是否已配置
}

// NewSession 创建新的皮肤制作会话
func NewSession(imagePath, skinName string, scanner *bufio.Scanner, hasRemoveBG bool) *Session {
	return &Session{
		imagePath:   imagePath,
		skinName:    skinName,
		scanner:     scanner,
		config:      NewSkinBuildConfig(skinName, imagePath),
		hasRemoveBG: hasRemoveBG,
	}
}

// RunGuidedDialog 运行引导对话，收集所有状态的特效配置
func (s *Session) RunGuidedDialog() (*SkinBuildConfig, error) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🎭 第三步：配置各状态下的表现特效")
	fmt.Println()
	fmt.Println("  桌宠有 5 种状态，每种状态可以有不同的动画特效。")
	fmt.Println("  我会逐一引导你配置，你也可以直接回车使用推荐设置。")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// 先询问是否去除背景（hasRemoveBG 由调用方通过 SetHasRemoveBG 设置）
	s.promptBackgroundRemoval(s.hasRemoveBG)

	// 逐状态引导配置
	for _, state := range AllStates {
		if err := s.configureState(state); err != nil {
			return nil, err
		}
	}

	// 询问 alert 全屏特效
	s.promptFullscreenEffect()

	// 展示配置摘要并确认
	s.showConfigSummary()
	if !s.promptConfirm("以上配置是否正确？确认后开始生成皮肤素材") {
		fmt.Println("\n  已取消，请重新运行程序进行配置。")
		os.Exit(0)
	}

	return s.config, nil
}

// promptBackgroundRemoval 询问背景去除方式
// hasRemoveBG 表示 remove.bg API 是否已配置
func (s *Session) promptBackgroundRemoval(hasRemoveBG bool) {
	fmt.Println("🖼️  图片背景处理")
	fmt.Println("  如果图片已经是透明背景（PNG），请选 [1] 跳过。")
	fmt.Println()

	if hasRemoveBG {
		fmt.Println("  [1] 不去除背景（图片已是透明背景）← 推荐")
		fmt.Println("  [2] 使用 remove.bg API 去除背景（效果最佳，消耗 API 额度）")
		fmt.Println("  [3] 使用本地算法去除背景（仅适合纯色背景，效果有限）")
		fmt.Println()

		for {
			fmt.Print("  请选择（直接回车使用推荐 [1]）：> ")
			line := s.readLine()
			switch line {
			case "", "1":
				s.config.BackgroundMode = ""
				fmt.Println("  ✅ 不去除背景")
				fmt.Println()
				return
			case "2":
				s.config.BackgroundMode = "removebg"
				fmt.Println("  ✅ 将使用 remove.bg API 去除背景")
				fmt.Println()
				return
			case "3":
				s.config.BackgroundMode = "local"
				fmt.Println("  ✅ 将使用本地算法去除背景")
				fmt.Println()
				return
			default:
				fmt.Println("  ⚠️  请输入 1、2 或 3。")
			}
		}
	} else {
		// remove.bg 未配置，只提供本地算法选项
		fmt.Println("  （提示：配置 remove.bg API Key 后可获得更好的抠图效果）")
		fmt.Println()
		fmt.Println("  [1] 不去除背景（图片已是透明背景）← 推荐")
		fmt.Println("  [2] 使用本地算法去除背景（仅适合纯色背景，效果有限）")
		fmt.Println()

		for {
			fmt.Print("  请选择（直接回车使用推荐 [1]）：> ")
			line := s.readLine()
			switch line {
			case "", "1":
				s.config.BackgroundMode = ""
				fmt.Println("  ✅ 不去除背景")
				fmt.Println()
				return
			case "2":
				s.config.BackgroundMode = "local"
				fmt.Println("  ✅ 将使用本地算法去除背景")
				fmt.Println()
				return
			default:
				fmt.Println("  ⚠️  请输入 1 或 2。")
			}
		}
	}
}

// configureState 引导用户配置单个状态
func (s *Session) configureState(state SkinState) error {
	stateCfg := s.config.States[state]
	displayName := StateDisplayName[state]
	description := StateDescription[state]

	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("🎬 状态：%s\n", displayName)
	fmt.Printf("   说明：%s\n", description)
	fmt.Println()

	// 展示可选特效列表
	fmt.Println("  可选特效：")
	effectOptions := getEffectOptionsForState(state)
	for i, effect := range effectOptions {
		defaultMark := ""
		if effect == stateCfg.Effect {
			defaultMark = " ← 推荐"
		}
		fmt.Printf("  [%d] %s%s\n", i+1, EffectDisplayName[effect], defaultMark)
	}
	fmt.Println()

	// 用户选择特效
	selectedEffect := s.promptEffectChoice(effectOptions, stateCfg.Effect)
	stateCfg.Effect = selectedEffect

	// 根据特效类型询问附加参数
	switch selectedEffect {
	case EffectNone:
		stateCfg.FrameCount = 1
		fmt.Println("  ✅ 已设置为静态图（无动画）")

	case EffectGlow:
		stateCfg.GlowColor = s.promptGlowColor()
		stateCfg.FrameCount = 8
		fmt.Printf("  ✅ 光晕颜色：RGB(%d, %d, %d)\n", stateCfg.GlowColor[0], stateCfg.GlowColor[1], stateCfg.GlowColor[2])

	case EffectParticle:
		stateCfg.ParticleType = s.promptParticleType()
		stateCfg.FrameCount = 12
		fmt.Printf("  ✅ 粒子类型：%s\n", stateCfg.ParticleType)

	default:
		// 询问特效强度
		stateCfg.EffectIntensity = s.promptEffectIntensity(selectedEffect)
		stateCfg.FrameCount = defaultFrameCount(selectedEffect)
		fmt.Printf("  ✅ 特效强度：%.0f%%，帧数：%d\n", stateCfg.EffectIntensity*100, stateCfg.FrameCount)
	}

	fmt.Println()
	return nil
}

// promptEffectChoice 让用户从列表中选择特效
func (s *Session) promptEffectChoice(options []EffectType, defaultEffect EffectType) EffectType {
	defaultIndex := 1
	for i, effect := range options {
		if effect == defaultEffect {
			defaultIndex = i + 1
			break
		}
	}

	for {
		fmt.Printf("  请选择特效编号（直接回车使用推荐 [%d]）：", defaultIndex)
		fmt.Print(" > ")

		line := s.readLine()
		if line == "" {
			return defaultEffect
		}

		num, err := strconv.Atoi(line)
		if err != nil || num < 1 || num > len(options) {
			fmt.Printf("  ⚠️  请输入 1~%d 之间的数字。\n", len(options))
			continue
		}
		return options[num-1]
	}
}

// promptEffectIntensity 询问特效强度（低/中/高）
func (s *Session) promptEffectIntensity(effect EffectType) float64 {
	intensityOptions := []struct {
		label string
		value float64
	}{
		{"低（轻微）", 0.3},
		{"中（适中）", 0.6},
		{"高（强烈）", 1.0},
	}

	fmt.Println("  特效强度：")
	for i, opt := range intensityOptions {
		defaultMark := ""
		if i == 1 {
			defaultMark = " ← 推荐"
		}
		fmt.Printf("  [%d] %s%s\n", i+1, opt.label, defaultMark)
	}

	for {
		fmt.Print("  请选择强度（直接回车使用推荐 [2]）：> ")
		line := s.readLine()
		if line == "" {
			return 0.6
		}
		num, err := strconv.Atoi(line)
		if err != nil || num < 1 || num > len(intensityOptions) {
			fmt.Printf("  ⚠️  请输入 1~%d 之间的数字。\n", len(intensityOptions))
			continue
		}
		return intensityOptions[num-1].value
	}
}

// promptGlowColor 询问光晕颜色
func (s *Session) promptGlowColor() [3]uint8 {
	colorOptions := []struct {
		name  string
		color [3]uint8
	}{
		{"白色（柔和）", [3]uint8{255, 255, 255}},
		{"蓝色（科技感）", [3]uint8{80, 160, 255}},
		{"金色（高贵）", [3]uint8{255, 200, 50}},
		{"粉色（可爱）", [3]uint8{255, 150, 200}},
		{"绿色（清新）", [3]uint8{100, 220, 100}},
		{"红色（热情）", [3]uint8{255, 80, 80}},
		{"紫色（神秘）", [3]uint8{180, 100, 255}},
	}

	fmt.Println("  光晕颜色：")
	for i, opt := range colorOptions {
		fmt.Printf("  [%d] %s\n", i+1, opt.name)
	}

	for {
		fmt.Print("  请选择颜色（直接回车使用白色 [1]）：> ")
		line := s.readLine()
		if line == "" {
			return colorOptions[0].color
		}
		num, err := strconv.Atoi(line)
		if err != nil || num < 1 || num > len(colorOptions) {
			fmt.Printf("  ⚠️  请输入 1~%d 之间的数字。\n", len(colorOptions))
			continue
		}
		return colorOptions[num-1].color
	}
}

// promptParticleType 询问粒子类型
func (s *Session) promptParticleType() string {
	particleOptions := []struct {
		name  string
		value string
	}{
		{"⭐ 星星", "star"},
		{"🎵 音符", "note"},
		{"❤️  爱心", "heart"},
		{"✨ 闪光", "sparkle"},
	}

	fmt.Println("  粒子类型：")
	for i, opt := range particleOptions {
		fmt.Printf("  [%d] %s\n", i+1, opt.name)
	}

	for {
		fmt.Print("  请选择粒子类型（直接回车使用星星 [1]）：> ")
		line := s.readLine()
		if line == "" {
			return "star"
		}
		num, err := strconv.Atoi(line)
		if err != nil || num < 1 || num > len(particleOptions) {
			fmt.Printf("  ⚠️  请输入 1~%d 之间的数字。\n", len(particleOptions))
			continue
		}
		return particleOptions[num-1].value
	}
}

// promptFullscreenEffect 询问 alert 状态的全屏特效
func (s *Session) promptFullscreenEffect() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🌟 全屏特效（告警状态专属）")
	fmt.Println("  收到紧急告警时，除了桌宠动画外，还可以触发全屏特效。")
	fmt.Println()
	fmt.Println("  [1] 无全屏特效（仅桌宠动画）← 推荐")
	fmt.Println("  [2] 篮球抛物线（图片从桌宠位置飞向屏幕中心）")
	fmt.Println()

	for {
		fmt.Print("  请选择（直接回车使用推荐 [1]）：> ")
		line := s.readLine()
		if line == "" || line == "1" {
			s.config.FullscreenEffect = FullscreenEffectNone
			fmt.Println("  ✅ 无全屏特效")
			break
		}
		if line == "2" {
			s.config.FullscreenEffect = FullscreenEffectBasketball
			fmt.Println("  ✅ 已启用篮球抛物线全屏特效")
			break
		}
		fmt.Println("  ⚠️  请输入 1 或 2。")
	}
	fmt.Println()
}

// showConfigSummary 展示配置摘要
func (s *Session) showConfigSummary() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📋 配置摘要")
	fmt.Printf("  皮肤名称：%s\n", s.config.SkinName)
	fmt.Printf("  图片路径：%s\n", s.config.ImagePath)
	fmt.Printf("  背景处理：%s\n", backgroundModeLabel(s.config.BackgroundMode))
	fmt.Printf("  输出尺寸：%dx%d px\n", s.config.SpriteSize, s.config.SpriteSize)
	fmt.Println()
	fmt.Println("  各状态特效：")
	for _, state := range AllStates {
		stateCfg := s.config.States[state]
		fmt.Printf("    %-12s → %s（%d 帧）\n",
			StateDisplayName[state],
			EffectDisplayName[stateCfg.Effect],
			stateCfg.FrameCount,
		)
	}
	if s.config.FullscreenEffect != FullscreenEffectNone {
		fmt.Printf("  全屏特效：篮球抛物线\n")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}

// promptYesNo 询问是/否，返回布尔值
func (s *Session) promptYesNo(question string, defaultYes bool) bool {
	defaultHint := "Y/n"
	if !defaultYes {
		defaultHint = "y/N"
	}
	for {
		fmt.Printf("  %s [%s]：> ", question, defaultHint)
		line := s.readLine()
		if line == "" {
			return defaultYes
		}
		lower := strings.ToLower(line)
		if lower == "y" || lower == "yes" || lower == "是" {
			return true
		}
		if lower == "n" || lower == "no" || lower == "否" {
			return false
		}
		fmt.Println("  ⚠️  请输入 y 或 n。")
	}
}

// promptConfirm 确认提示，返回是否确认
func (s *Session) promptConfirm(question string) bool {
	return s.promptYesNo(question, true)
}

// readLine 读取一行输入并去除首尾空白
func (s *Session) readLine() string {
	if !s.scanner.Scan() {
		os.Exit(0)
	}
	return strings.TrimSpace(s.scanner.Text())
}

// backgroundModeLabel 将背景模式转为可读标签
func backgroundModeLabel(mode string) string {
	switch mode {
	case "removebg":
		return "使用 remove.bg API 去除背景"
	case "local":
		return "使用本地算法去除背景"
	default:
		return "不去除背景（保留原图）"
	}
}

// getEffectOptionsForState 根据状态返回合适的特效选项列表
func getEffectOptionsForState(state SkinState) []EffectType {
	switch state {
	case StateIdle:
		return []EffectType{
			EffectBob,
			EffectWave,
			EffectPulse,
			EffectGlow,
			EffectNone,
		}
	case StateTyping:
		return []EffectType{
			EffectShake,
			EffectBounce,
			EffectParticle,
			EffectFlash,
			EffectBob,
		}
	case StateThinking:
		return []EffectType{
			EffectSpin,
			EffectPulse,
			EffectGlow,
			EffectWave,
			EffectBob,
		}
	case StateTalking:
		return []EffectType{
			EffectPulse,
			EffectGlow,
			EffectParticle,
			EffectBob,
			EffectWave,
		}
	case StateAlert:
		return []EffectType{
			EffectBounce,
			EffectFlash,
			EffectShake,
			EffectGlow,
			EffectSpin,
		}
	default:
		return []EffectType{EffectBob, EffectShake, EffectPulse, EffectGlow, EffectNone}
	}
}

// defaultFrameCount 根据特效类型返回默认帧数
func defaultFrameCount(effect EffectType) int {
	switch effect {
	case EffectNone:
		return 1
	case EffectSpin:
		return 12
	case EffectParticle:
		return 12
	case EffectBounce:
		return 10
	case EffectFlash:
		return 6
	default:
		return 8
	}
}
