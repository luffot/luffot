package pet

import (
	"image/color"
	"math"
	"os"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"strings"
)

// PetState 宠物动画状态
type PetState int

const (
	StateIdle     PetState = iota // 摸鱼状态
	StateTyping                   // 敲键盘状态
	StateThinking                 // AI 思考中
	StateTalking                  // AI 正在说话
	StateAlert                    // 告警惊讶状态
)

// alertStateDuration 告警惊讶状态持续时长（与气泡同步）
const alertStateDuration = 8 * time.Second

// alertBubble 对话气泡（秘书汇报消息）
type alertBubble struct {
	lines     []string  // 换行后的文字行
	createdAt time.Time // 创建时间
	duration  time.Duration
}

// chatDialogMessage 对话框中的一条消息记录
type chatDialogMessage struct {
	isUser  bool   // true=用户消息，false=AI 回复
	content string // 消息内容（AI 回复可能含 Markdown）
}

// mdLineType Markdown 行类型，用于对话框富文本渲染
type mdLineType int

const (
	mdLineNormal    mdLineType = iota // 普通文本
	mdLineHeading                     // 标题（# / ## / ###）
	mdLineCode                        // 代码块内容
	mdLineCodeFence                   // 代码块围栏行（```）
	mdLineListItem                    // 列表项（- / * / 数字.）
	mdLineQuote                       // 引用（>）
	mdLineDivider                     // 分割线（---）
)

// mdSpanType 行内片段类型
type mdSpanType int

const (
	mdSpanText   mdSpanType = iota // 普通文本
	mdSpanBold                     // 加粗（**text**）
	mdSpanInline                   // 行内代码（`code`）
)

// mdSpan 行内文本片段（带样式）
type mdSpan struct {
	text     string
	spanType mdSpanType
}

// mdLine 解析后的 Markdown 行（带行类型和行内片段）
type mdLine struct {
	lineType    mdLineType
	spans       []mdSpan // 行内片段（普通行、标题行等使用）
	rawText     string   // 原始文本（代码块等直接使用）
	headLevel   int      // 标题级别（1/2/3）
	listPrefix  string   // 列表前缀（"• " 或 "1. "）
	inCodeFence bool     // 是否在代码块内（由 parseMdLines 填充）
}

// chatDialog 沉浸式对话框（在宠物旁边渲染）
type chatDialog struct {
	visible bool // 是否可见

	inputText string // 当前输入框内容

	messages []chatDialogMessage // 历史消息

	isThinking   bool // AI 是否正在思考中
	thinkingTick int  // 思考动画帧计数

	// 打字机效果：AI 回复逐字展示（非流式模式使用）
	typingTarget  string // 目标文字（完整 AI 回复）
	typingCurrent string // 当前已展示的文字
	typingTick    int    // 打字机帧计数

	// 流式模式：AI 回复逐 token 实时追加
	streamingText string // 流式正在接收的累积文本（流式完成后清空）
	isStreaming   bool   // 是否正在接收流式 token

	// 消息区滚动偏移（行数，0 表示显示最新内容，正数表示向上滚动查看历史）
	scrollOffset int

	// 对话框当前渲染边界（由 drawChatDialog 写入，Update 读取用于点击检测）
	boundsLeft   float32
	boundsTop    float32
	boundsRight  float32
	boundsBottom float32

	// 清空按钮渲染边界（由 drawChatDialog 写入，Update 读取用于点击检测）
	clearBtnLeft   float32
	clearBtnTop    float32
	clearBtnRight  float32
	clearBtnBottom float32

	// 上一帧鼠标左键状态（用于单击检测）
	prevMousePressed bool

	// 输入框光标闪烁帧计数
	cursorTick int

	// 按键帧间状态（用于实现"刚按下"检测，防止长按重复触发）
	prevBackspacePressed bool
	prevEnterPressed     bool
	prevEscapePressed    bool
}

// maxDisplayMessages 对话框最多展示的历史消息条数
const maxDisplayMessages = 6

// chatDialogWidth 对话框宽度
const chatDialogWidth = 280

// chatDialogInputHeight 输入框高度
const chatDialogInputHeight = 32

// chatDialogLineHeight 消息行高
const chatDialogLineHeight = 18

// chatDialogPaddingH 水平内边距
const chatDialogPaddingH = 10

// chatDialogPaddingV 垂直内边距
const chatDialogPaddingV = 8

// typingSpeedFrames 打字机效果每隔多少帧输出一个字符
const typingSpeedFrames = 2

// bubbleFontSize 气泡字体大小
const bubbleFontSize = 14

// bubbleMaxWidth 气泡最大文字宽度（像素）
const bubbleMaxWidth = 220

// bubbleDisplayDuration 气泡显示时长
const bubbleDisplayDuration = 7 * time.Second

// bubbleFadeInDuration 淡入时长
const bubbleFadeInDuration = 300 * time.Millisecond

// bubbleFadeOutDuration 淡出时长
const bubbleFadeOutDuration = 800 * time.Millisecond

// typingCooldown 检测到键盘输入后保持 typing 状态的时长
const typingCooldown = 2 * time.Second

// allKeys 需要监听的所有按键（覆盖常用键位）
var allKeys = []ebiten.Key{
	ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE,
	ebiten.KeyF, ebiten.KeyG, ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ,
	ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN, ebiten.KeyO,
	ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT,
	ebiten.KeyU, ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY,
	ebiten.KeyZ,
	ebiten.KeyDigit0, ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3,
	ebiten.KeyDigit4, ebiten.KeyDigit5, ebiten.KeyDigit6, ebiten.KeyDigit7,
	ebiten.KeyDigit8, ebiten.KeyDigit9,
	ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyBackspace, ebiten.KeyTab,
	ebiten.KeyEscape, ebiten.KeyDelete,
	ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown,
	ebiten.KeyShiftLeft, ebiten.KeyShiftRight,
	ebiten.KeyControlLeft, ebiten.KeyControlRight,
	ebiten.KeyAltLeft, ebiten.KeyAltRight,
	ebiten.KeyMeta,
	ebiten.KeyComma, ebiten.KeyPeriod, ebiten.KeySlash, ebiten.KeySemicolon,
	ebiten.KeyQuote, ebiten.KeyBracketLeft, ebiten.KeyBracketRight,
	ebiten.KeyBackslash, ebiten.KeyMinus, ebiten.KeyEqual, ebiten.KeyBackquote,
	ebiten.KeyF1, ebiten.KeyF2, ebiten.KeyF3, ebiten.KeyF4, ebiten.KeyF5,
	ebiten.KeyF6, ebiten.KeyF7, ebiten.KeyF8, ebiten.KeyF9, ebiten.KeyF10,
	ebiten.KeyF11, ebiten.KeyF12,
}

// PetSprite 桌面宠物精灵，嵌入到 BarrageDisplay 中渲染
type PetSprite struct {
	mu sync.Mutex

	state          PetState
	lastTypingTime time.Time

	tick       int           // 全局帧计数（用于动画）
	frameImage *ebiten.Image // 当前帧缓冲（避免每帧重新分配）

	screenWidth  int
	screenHeight int

	// 宠物当前位置（左上角坐标），支持拖拽移动
	posX float64
	posY float64

	// 拖拽状态
	isDragging  bool
	dragOffsetX float64 // 鼠标按下时相对于宠物左上角的偏移
	dragOffsetY float64
	isHovered   bool    // 鼠标是否悬停在宠物上
	pressStartX float64 // 鼠标按下时的起始 X（用于区分单击和拖拽）
	pressStartY float64 // 鼠标按下时的起始 Y
	pressInPet  bool    // 鼠标按下时是否在宠物区域内

	// 对话气泡（秘书汇报）
	bubble     *alertBubble
	bubbleFont font.Face

	// 告警状态：记录进入 StateAlert 的时间，用于自动恢复
	alertStartTime time.Time

	// 沉浸式 AI 对话框
	dialog *chatDialog

	// AI 对话回调：用户提交输入时调用（由 barrage.go 注入）
	onChatSubmit func(input string)
}

// NewPetSprite 创建宠物精灵，初始位置在屏幕右下角
func NewPetSprite(screenWidth, screenHeight int) *PetSprite {
	p := &PetSprite{
		state:        StateIdle,
		frameImage:   ebiten.NewImage(spriteWidth, spriteHeight),
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
		bubbleFont:   loadBubbleFontFace(bubbleFontSize),
		dialog:       &chatDialog{},
	}
	p.resetToDefaultPosition()
	// 自动扫描并加载 assets/skins 目录下的图片皮肤
	AutoLoadImageSkins()
	// 启动全局键盘监听（macOS CGEventTap，需要辅助功能权限）
	startGlobalKeyboardListener()
	return p
}

// SetChatSubmitCallback 注入 AI 对话回调（由 barrage.go 在初始化时调用）
func (p *PetSprite) SetChatSubmitCallback(callback func(input string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onChatSubmit = callback
}

// IsChatDialogVisible 返回对话框是否可见（供 barrage.go 控制鼠标穿透）
func (p *PetSprite) IsChatDialogVisible() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dialog.visible
}

// IsAlertActive 返回当前是否处于告警惊讶状态（供 barrage.go 绘制屏幕边缘红色渐变遮罩）
func (p *PetSprite) IsAlertActive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state == StateAlert
}

// GetPosition 返回宠物当前左上角坐标（供 barrage.go 计算全屏精灵起点）
func (p *PetSprite) GetPosition() (x, y float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.posX, p.posY
}

// ShowChatReply 展示 AI 回复（流式完成后调用，将完整回复写入消息列表），同时切换宠物状态（线程安全）
func (p *PetSprite) ShowChatReply(reply string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dialog.isThinking = false

	if p.dialog.isStreaming {
		// 流式模式：流已经结束，将完整回复写入消息列表，清空流式缓冲
		p.dialog.isStreaming = false
		p.dialog.streamingText = ""
		p.dialog.messages = append(p.dialog.messages, chatDialogMessage{
			isUser:  false,
			content: reply,
		})
		if len(p.dialog.messages) > maxDisplayMessages {
			p.dialog.messages = p.dialog.messages[len(p.dialog.messages)-maxDisplayMessages:]
		}
		p.state = StateIdle
		return
	}

	// 非流式模式：走打字机效果
	p.dialog.typingTarget = reply
	p.dialog.typingCurrent = ""
	p.dialog.typingTick = 0
	p.state = StateTalking
}

// AppendStreamToken 追加一个流式 token 到对话框（线程安全，由 barrage 的流式回调调用）
func (p *PetSprite) AppendStreamToken(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dialog.isThinking = false
	p.dialog.isStreaming = true
	p.dialog.streamingText += token
	p.state = StateTalking
}

// SetThinking 设置 AI 思考中状态（线程安全）
func (p *PetSprite) SetThinking(thinking bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dialog.isThinking = thinking
	if thinking {
		p.state = StateThinking
	}
}

// loadBubbleFontFace 加载气泡字体，优先使用系统中文字体，回退到内嵌英文字体
func loadBubbleFontFace(size int) font.Face {
	systemFontPaths := []string{
		"/System/Library/Fonts/STHeiti Medium.ttc",
		"/System/Library/Fonts/Hiragino Sans GB.ttc",
		"/System/Library/Fonts/STHeiti Light.ttc",
	}
	for _, fontPath := range systemFontPaths {
		data, err := os.ReadFile(fontPath)
		if err != nil {
			continue
		}
		// 尝试作为字体集合解析（.ttc 格式）
		if collection, err := opentype.ParseCollection(data); err == nil {
			if tt, err := collection.Font(0); err == nil {
				if face, err := opentype.NewFace(tt, &opentype.FaceOptions{
					Size:    float64(size),
					DPI:     72,
					Hinting: font.HintingFull,
				}); err == nil {
					return face
				}
			}
		}
		// 尝试作为单个字体解析（.ttf 格式）
		if tt, err := opentype.Parse(data); err == nil {
			if face, err := opentype.NewFace(tt, &opentype.FaceOptions{
				Size:    float64(size),
				DPI:     72,
				Hinting: font.HintingFull,
			}); err == nil {
				return face
			}
		}
	}
	// 回退到内嵌英文字体
	tt, _ := opentype.Parse(goregular.TTF)
	face, _ := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	return face
}

// ShowAlert 触发桌宠对话气泡，展示秘书汇报消息，同时切换到惊讶告警状态（线程安全）
func (p *PetSprite) ShowAlert(message string) {
	lines := wrapBubbleText(message, p.bubbleFont, bubbleMaxWidth)
	bubble := &alertBubble{
		lines:     lines,
		createdAt: time.Now(),
		duration:  bubbleDisplayDuration,
	}
	p.mu.Lock()
	p.bubble = bubble
	p.state = StateAlert
	p.alertStartTime = time.Now()
	p.mu.Unlock()
}

// safeBoundString 安全地调用 font.BoundString，捕获底层字体库可能抛出的 panic。
// 某些特殊字符（如 emoji、罕见 Unicode 字形）会触发 golang.org/x/image 的越界 bug，
// 发生 panic 时回退到按字符数估算宽度。
func safeBoundString(face font.Face, text string) (width int) {
	defer func() {
		if r := recover(); r != nil {
			// 估算：每个 rune 按字体 size 的一半计算宽度
			metrics := face.Metrics()
			charWidth := metrics.Height.Ceil()
			width = len([]rune(text)) * charWidth
		}
	}()
	bounds, _ := font.BoundString(face, text)
	return (bounds.Max.X - bounds.Min.X).Ceil()
}

// wrapBubbleText 将消息按最大宽度换行，返回行列表
func wrapBubbleText(message string, face font.Face, maxWidth int) []string {
	if face == nil || message == "" {
		return []string{message}
	}

	// 预处理：移除 emoji 及字体不支持的字符，防止 text.Draw 渲染出乱码方块
	message = sanitizeForFont(message, face)

	var lines []string
	runes := []rune(message)
	lineStart := 0

	for lineStart < len(runes) {
		lineEnd := len(runes)
		// 二分查找当前行能容纳的最大字符数
		for lineEnd > lineStart+1 {
			candidate := string(runes[lineStart:lineEnd])
			width := safeBoundString(face, candidate)
			if width <= maxWidth {
				break
			}
			// 按字符数缩减（中文每字约等宽，效率足够）
			lineEnd--
		}
		// 检查是否有换行符
		for i := lineStart; i < lineEnd; i++ {
			if runes[i] == '\n' {
				lineEnd = i
				break
			}
		}
		line := string(runes[lineStart:lineEnd])
		lines = append(lines, line)
		lineStart = lineEnd
		// 跳过换行符
		if lineStart < len(runes) && runes[lineStart] == '\n' {
			lineStart++
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// bubbleAlpha 根据气泡生命周期计算当前透明度（0~255）
func bubbleAlpha(bubble *alertBubble) uint8 {
	elapsed := time.Since(bubble.createdAt)
	total := bubble.duration

	if elapsed >= total {
		return 0
	}

	// 淡入阶段
	if elapsed < bubbleFadeInDuration {
		ratio := float64(elapsed) / float64(bubbleFadeInDuration)
		return uint8(ratio * 255)
	}

	// 淡出阶段
	fadeOutStart := total - bubbleFadeOutDuration
	if elapsed >= fadeOutStart {
		ratio := float64(total-elapsed) / float64(bubbleFadeOutDuration)
		return uint8(ratio * 255)
	}

	return 255
}

// resetToDefaultPosition 将宠物位置重置到屏幕右下角（距底部 10% 高度处）
func (p *PetSprite) resetToDefaultPosition() {
	p.posX = float64(p.screenWidth-spriteWidth) - 10
	p.posY = float64(p.screenHeight)*0.9 - float64(spriteHeight)
}

// SetScreenSize 更新屏幕尺寸并重置宠物到右下角（在 RunBarrage 中调用）
func (p *PetSprite) SetScreenSize(screenWidth, screenHeight int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.screenWidth = screenWidth
	p.screenHeight = screenHeight
	p.resetToDefaultPosition()
}

// IsHovered 返回鼠标是否悬停在宠物上（供 barrage.go 控制鼠标穿透）
func (p *PetSprite) IsHovered() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isHovered
}

// Update 每帧更新宠物状态（在 BarrageDisplay.Update 中调用，运行于主 goroutine）
// 返回值：是否正在拖拽或对话框可见（供 barrage.go 控制鼠标穿透）
func (p *PetSprite) Update() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.tick++

	// 更新对话框打字机效果和思考动画
	p.updateDialog()

	// 告警状态超时后自动恢复到 idle
	if p.state == StateAlert && time.Since(p.alertStartTime) >= alertStateDuration {
		p.state = StateIdle
	}

	// 对话框可见时：优先处理对话框键盘输入，不切换 typing 状态
	if p.dialog.visible {
		p.updateDialogInput()
	} else {
		// 实时检测是否有键被按住：按下立刻切 typing，松开后延迟 10ms 再切 idle
		// AI 思考/说话/告警状态不被键盘覆盖
		if p.state != StateThinking && p.state != StateTalking && p.state != StateAlert {
			if isGlobalKeyHeld() || isAnyKeyPressed() {
				p.state = StateTyping
				p.lastTypingTime = time.Now()
			} else if p.state == StateTyping {
				// typing 结束后延迟 10ms 再切换，避免状态切换过快导致闪屏
				if time.Since(p.lastTypingTime) >= 100*time.Millisecond {
					p.state = StateIdle
				}
			}
		}
	}

	// 鼠标拖拽处理
	mouseX, mouseY := ebiten.CursorPosition()
	mx, my := float64(mouseX), float64(mouseY)

	// 判断鼠标是否在宠物区域内
	p.isHovered = mx >= p.posX && mx <= p.posX+float64(spriteWidth) &&
		my >= p.posY && my <= p.posY+float64(spriteHeight)

	currentMousePressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	// 拖拽阈值：移动超过 5px 才算拖拽，否则算单击
	const dragThreshold = 5.0

	if currentMousePressed {
		if !p.dialog.prevMousePressed {
			// 刚按下：记录起始位置和是否在宠物内
			p.pressStartX = mx
			p.pressStartY = my
			p.pressInPet = p.isHovered
			p.dragOffsetX = mx - p.posX
			p.dragOffsetY = my - p.posY
		}

		// 判断是否超过拖拽阈值
		movedDistance := math.Sqrt(math.Pow(mx-p.pressStartX, 2) + math.Pow(my-p.pressStartY, 2))
		if p.pressInPet && movedDistance > dragThreshold {
			p.isDragging = true
		}

		if p.isDragging {
			newX := mx - p.dragOffsetX
			newY := my - p.dragOffsetY
			p.posX = clamp(newX, 0, float64(p.screenWidth-spriteWidth))
			p.posY = clamp(newY, 0, float64(p.screenHeight-spriteHeight))
		}
	} else {
		// 松开鼠标：判断是单击还是拖拽结束
		if p.dialog.prevMousePressed && !p.isDragging {
			if p.pressInPet {
				// 单击宠物：切换对话框
				p.dialog.visible = !p.dialog.visible
				if p.dialog.visible && len(p.dialog.messages) == 0 {
					// 首次打开，添加欢迎消息
					p.dialog.messages = append(p.dialog.messages, chatDialogMessage{
						isUser:  false,
						content: "嗨嗨主人！我是小钉，点我就能聊天哦～有什么我能帮你的吗？",
					})
				}
			} else if p.dialog.visible {
				// 单击了非宠物区域：先检查是否点击了清空按钮
				clickOnClearBtn := float32(mx) >= p.dialog.clearBtnLeft &&
					float32(mx) <= p.dialog.clearBtnRight &&
					float32(my) >= p.dialog.clearBtnTop &&
					float32(my) <= p.dialog.clearBtnBottom
				if clickOnClearBtn {
					// 清空所有消息并重置滚动
					p.dialog.messages = nil
					p.dialog.scrollOffset = 0
					p.dialog.typingTarget = ""
					p.dialog.typingCurrent = ""
					p.dialog.streamingText = ""
					p.dialog.isStreaming = false
					p.dialog.isThinking = false
					p.state = StateIdle
				} else {
					// 检查是否在对话框内
					clickInDialog := float32(mx) >= p.dialog.boundsLeft &&
						float32(mx) <= p.dialog.boundsRight &&
						float32(my) >= p.dialog.boundsTop &&
						float32(my) <= p.dialog.boundsBottom
					if !clickInDialog {
						// 点击了对话框外部，关闭对话框
						p.dialog.visible = false
						p.dialog.inputText = ""
					}
				}
			}
		}
		p.isDragging = false
		p.pressInPet = false
	}

	// 每帧末尾更新上一帧鼠标状态（必须在所有判断之后）
	p.dialog.prevMousePressed = currentMousePressed

	return p.isDragging || p.dialog.visible
}

// updateDialog 每帧更新对话框内部状态（打字机效果、思考动画）
// 调用方必须持有 p.mu 锁
func (p *PetSprite) updateDialog() {
	if !p.dialog.visible {
		return
	}

	// 思考动画帧计数
	if p.dialog.isThinking {
		p.dialog.thinkingTick++
	}

	// 光标闪烁帧计数
	p.dialog.cursorTick++

	// 打字机效果：逐字展示 AI 回复
	if p.dialog.typingTarget != "" {
		p.dialog.typingTick++
		if p.dialog.typingTick%typingSpeedFrames == 0 {
			targetRunes := []rune(p.dialog.typingTarget)
			currentRunes := []rune(p.dialog.typingCurrent)
			if len(currentRunes) < len(targetRunes) {
				p.dialog.typingCurrent = string(targetRunes[:len(currentRunes)+1])
			} else {
				// 打字完成：将完整回复写入消息列表
				p.dialog.messages = append(p.dialog.messages, chatDialogMessage{
					isUser:  false,
					content: p.dialog.typingTarget,
				})
				// 超出最大展示条数时裁剪
				if len(p.dialog.messages) > maxDisplayMessages {
					p.dialog.messages = p.dialog.messages[len(p.dialog.messages)-maxDisplayMessages:]
				}
				p.dialog.typingTarget = ""
				p.dialog.typingCurrent = ""
				// 打字完成后切回 idle
				p.state = StateIdle
			}
		}
	}
}

// updateDialogInput 处理对话框键盘输入（必须在主 goroutine 中调用）
// 调用方必须持有 p.mu 锁
func (p *PetSprite) updateDialogInput() {
	escPressed := ebiten.IsKeyPressed(ebiten.KeyEscape)
	backspacePressed := ebiten.IsKeyPressed(ebiten.KeyBackspace)
	enterPressed := ebiten.IsKeyPressed(ebiten.KeyEnter)

	// ESC 关闭对话框（刚按下时触发一次）
	if escPressed && !p.dialog.prevEscapePressed {
		p.dialog.visible = false
		p.dialog.inputText = ""
		p.dialog.prevEscapePressed = true
		p.dialog.prevBackspacePressed = backspacePressed
		p.dialog.prevEnterPressed = enterPressed
		return
	}
	p.dialog.prevEscapePressed = escPressed

	// Backspace 删除最后一个字符（刚按下时触发一次）
	if backspacePressed && !p.dialog.prevBackspacePressed {
		runes := []rune(p.dialog.inputText)
		if len(runes) > 0 {
			p.dialog.inputText = string(runes[:len(runes)-1])
		}
	}
	p.dialog.prevBackspacePressed = backspacePressed

	// Enter 提交输入（刚按下时触发一次）
	if enterPressed && !p.dialog.prevEnterPressed {
		input := p.dialog.inputText
		if input != "" && !p.dialog.isThinking {
			// 将用户消息加入对话框
			p.dialog.messages = append(p.dialog.messages, chatDialogMessage{
				isUser:  true,
				content: input,
			})
			if len(p.dialog.messages) > maxDisplayMessages {
				p.dialog.messages = p.dialog.messages[len(p.dialog.messages)-maxDisplayMessages:]
			}
			p.dialog.inputText = ""
			p.dialog.isThinking = true
			p.state = StateThinking

			// 调用 AI 回调（在锁外执行，避免死锁）
			callback := p.onChatSubmit
			go func() {
				if callback != nil {
					callback(input)
				}
			}()
		}
	}
	p.dialog.prevEnterPressed = enterPressed

	// 鼠标滚轮：滚动消息区（向上滚动减小 scrollOffset，向下滚动增大）
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		// 每次滚动 1 行，向上滚（wheelY > 0）减小偏移（看更早的消息）
		delta := -int(wheelY)
		p.dialog.scrollOffset += delta
		if p.dialog.scrollOffset < 0 {
			p.dialog.scrollOffset = 0
		}
	}

	// 接收普通字符输入（ebiten.AppendInputChars 本身已处理重复，无需帧间去重）
	inputChars := ebiten.AppendInputChars(nil)
	for _, ch := range inputChars {
		p.dialog.inputText += string(ch)
	}
}

// Draw 将宠物绘制到当前位置（在 BarrageDisplay.Draw 中调用）
func (p *PetSprite) Draw(screen *ebiten.Image) {
	p.mu.Lock()
	state := p.state
	tick := p.tick
	posX := p.posX
	posY := p.posY
	bubble := p.bubble
	bubbleFont := p.bubbleFont
	// 气泡过期则清除
	if bubble != nil && time.Since(bubble.createdAt) >= bubble.duration {
		p.bubble = nil
		bubble = nil
	}
	// 快照对话框状态（避免持锁期间调用 ebiten 绘制函数）
	dialogVisible := p.dialog.visible
	dialogMessages := make([]chatDialogMessage, len(p.dialog.messages))
	copy(dialogMessages, p.dialog.messages)
	dialogInputText := p.dialog.inputText
	dialogIsThinking := p.dialog.isThinking
	dialogThinkingTick := p.dialog.thinkingTick
	dialogTypingCurrent := p.dialog.typingCurrent
	dialogTypingTarget := p.dialog.typingTarget
	dialogStreamingText := p.dialog.streamingText
	dialogIsStreaming := p.dialog.isStreaming
	dialogCursorTick := p.dialog.cursorTick
	screenWidth := p.screenWidth
	p.mu.Unlock()

	// 渲染当前帧到 frameImage
	frame := (tick / 6) % 6
	if IsLuaSkinActive() {
		// Lua 皮肤模式
		p.frameImage.Fill(color.RGBA{0, 0, 0, 0})
		// 同步 Lua 皮肤状态
		SetLuaPetState(state)
		// 更新 Lua 皮肤动画
		UpdateLuaSkin()
		// 绘制 Lua 皮肤到 frameImage
		DrawLuaSkin(p.frameImage, 0, 0)
	} else if imageSkin := GetActiveImageSkin(); imageSkin != nil {
		// 图片皮肤模式：直接绘制对应状态的帧图片
		p.frameImage.Fill(color.RGBA{0, 0, 0, 0})
		if skinFrame := imageSkin.GetFrame(state, tick); skinFrame != nil {
			op := &ebiten.DrawImageOptions{}
			// 将图片缩放到 spriteWidth x spriteHeight
			srcW := skinFrame.Bounds().Dx()
			srcH := skinFrame.Bounds().Dy()
			if srcW > 0 && srcH > 0 {
				op.GeoM.Scale(float64(spriteWidth)/float64(srcW), float64(spriteHeight)/float64(srcH))
			}
			p.frameImage.DrawImage(skinFrame, op)
		}
	} else {
		// 矢量绘制模式（默认）
		switch state {
		case StateIdle:
			DrawIdleFrame(p.frameImage, frame, tick)
		case StateTyping:
			DrawTypingFrame(p.frameImage, tick)
		case StateThinking:
			// 思考状态：复用 idle 帧，但眼睛会有特殊效果（在 drawStateLabel 中体现）
			DrawIdleFrame(p.frameImage, frame, tick)
		case StateTalking:
			// 说话状态：复用 idle 帧
			DrawIdleFrame(p.frameImage, frame, tick)
		case StateAlert:
			// 告警状态：惊讶急切动画
			DrawAlertFrame(p.frameImage, tick)
		}
	}

	// 绘制宠物底座光晕（科幻感平台）
	drawPetPlatform(screen, posX+float64(spriteWidth)/2, posY+float64(spriteHeight)-8)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(posX, posY)
	screen.DrawImage(p.frameImage, op)

	// 绘制状态标签
	drawStateLabel(screen, state, posX+float64(spriteWidth)/2, posY+float64(spriteHeight)+4)

	// 绘制对话气泡（秘书汇报，对话框未打开时才显示）
	if bubble != nil && !dialogVisible {
		beakX := posX + float64(spriteWidth)/2
		beakY := posY + float64(spriteHeight)/2 - 13
		drawAlertBubble(screen, bubble, bubbleFont, beakX, beakY)
	}

	// 绘制沉浸式 AI 对话框，并将渲染边界写回（供 Update 做点击检测）
	if dialogVisible {
		p.mu.Lock()
		dialogScrollOffset := p.dialog.scrollOffset
		p.mu.Unlock()

		left, top, right, bottom, clearLeft, clearTop, clearRight, clearBottom := drawChatDialog(screen, bubbleFont,
			posX, posY, float64(screenWidth),
			dialogMessages, dialogInputText,
			dialogIsThinking, dialogThinkingTick,
			dialogTypingCurrent, dialogTypingTarget,
			dialogStreamingText, dialogIsStreaming,
			dialogCursorTick, dialogScrollOffset,
		)
		p.mu.Lock()
		p.dialog.boundsLeft = left
		p.dialog.boundsTop = top
		p.dialog.boundsRight = right
		p.dialog.boundsBottom = bottom
		p.dialog.clearBtnLeft = clearLeft
		p.dialog.clearBtnTop = clearTop
		p.dialog.clearBtnRight = clearRight
		p.dialog.clearBtnBottom = clearBottom
		p.mu.Unlock()
	}
}

// clamp 将值限制在 [min, max] 范围内
func clamp(value, minVal, maxVal float64) float64 {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

// isAnyKeyPressed 检测是否有任意按键被按下
func isAnyKeyPressed() bool {
	for _, key := range allKeys {
		if ebiten.IsKeyPressed(key) {
			return true
		}
	}
	return false
}

// drawPetPlatform 绘制宠物站立的科幻感椭圆光晕平台
func drawPetPlatform(screen *ebiten.Image, cx, cy float64) {
	for i := 0; i < 5; i++ {
		alpha := uint8(35 - i*6)
		rx := float32(28 + i*4)
		ry := float32(5 + i)
		clr := color.RGBA{100, 180, 255, alpha}
		steps := 72
		for step := 0; step < steps; step++ {
			rad := float64(step) / float64(steps) * 2 * math.Pi
			px := float32(cx) + rx*float32(math.Cos(rad))
			py := float32(cy) + ry*float32(math.Sin(rad))
			vector.DrawFilledCircle(screen, px, py, 1.5, clr, true)
		}
	}
	// 中心亮线
	vector.StrokeLine(screen,
		float32(cx)-24, float32(cy),
		float32(cx)+24, float32(cy),
		1.5, color.RGBA{150, 210, 255, 130}, true,
	)
}

// drawStateLabel 在宠物下方绘制状态标签（科幻风格，用像素点阵文字）
func drawStateLabel(screen *ebiten.Image, state PetState, cx, cy float64) {
	var dots [][]bool
	var labelColor color.RGBA

	switch state {
	case StateIdle:
		dots = pixelString("IDLE")
		labelColor = color.RGBA{100, 200, 255, 200}
	case StateTyping:
		dots = pixelString("TYPE")
		labelColor = color.RGBA{255, 200, 80, 220}
	case StateThinking:
		dots = pixelString("THINK")
		labelColor = color.RGBA{180, 120, 255, 220}
	case StateTalking:
		dots = pixelString("TALK")
		labelColor = color.RGBA{80, 255, 160, 220}
	case StateAlert:
		dots = pixelString("ALERT")
		labelColor = color.RGBA{255, 60, 60, 240}
	}

	if len(dots) == 0 {
		return
	}

	cols := len(dots[0])
	rows := len(dots)
	scale := 2
	totalW := cols * scale
	totalH := rows * scale

	startX := int(cx) - totalW/2
	startY := int(cy) + 2

	// 背景
	bgPad := 4
	drawRoundRect(screen,
		float32(startX-bgPad), float32(startY-bgPad),
		float32(totalW+bgPad*2), float32(totalH+bgPad*2),
		3, color.RGBA{10, 20, 40, 160},
	)
	vector.StrokeRect(screen,
		float32(startX-bgPad), float32(startY-bgPad),
		float32(totalW+bgPad*2), float32(totalH+bgPad*2),
		1, labelColor, true,
	)

	// 绘制点阵文字
	for row, rowDots := range dots {
		for col, lit := range rowDots {
			if lit {
				px := startX + col*scale
				py := startY + row*scale
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						screen.Set(px+dx, py+dy, labelColor)
					}
				}
			}
		}
	}
}

// pixelString 将字符串转换为 5x7 点阵二维布尔数组（多字符横向拼接，字符间距1列）
func pixelString(s string) [][]bool {
	if len(s) == 0 {
		return nil
	}
	charMaps := make([][][]bool, 0, len(s))
	for _, ch := range s {
		charMaps = append(charMaps, pixelChar(ch))
	}

	rows := 7
	// 每个字符5列 + 1列间距（最后一个字符不加间距）
	totalCols := len(charMaps)*5 + (len(charMaps)-1)*1
	result := make([][]bool, rows)
	for r := range result {
		result[r] = make([]bool, totalCols)
	}

	colOffset := 0
	for ci, cm := range charMaps {
		for r := 0; r < rows && r < len(cm); r++ {
			for c := 0; c < 5 && c < len(cm[r]); c++ {
				result[r][colOffset+c] = cm[r][c]
			}
		}
		colOffset += 5
		if ci < len(charMaps)-1 {
			colOffset += 1 // 字符间距
		}
	}
	return result
}

// pixelChar 返回单个字符的 7行×5列 点阵（true=亮）
// 覆盖 A-Z 和常用符号
func pixelChar(ch rune) [][]bool {
	T, F := true, false
	switch ch {
	case 'A':
		return [][]bool{
			{F, T, T, T, F},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, T, T, T, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
		}
	case 'B':
		return [][]bool{
			{T, T, T, T, F},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, T, T, T, F},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, T, T, T, F},
		}
	case 'C':
		return [][]bool{
			{F, T, T, T, F},
			{T, F, F, F, T},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, F, F, F, T},
			{F, T, T, T, F},
		}
	case 'D':
		return [][]bool{
			{T, T, T, F, F},
			{T, F, F, T, F},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, T, F},
			{T, T, T, F, F},
		}
	case 'E':
		return [][]bool{
			{T, T, T, T, T},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, T, T, T, F},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, T, T, T, T},
		}
	case 'G':
		return [][]bool{
			{F, T, T, T, F},
			{T, F, F, F, T},
			{T, F, F, F, F},
			{T, F, T, T, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{F, T, T, T, F},
		}
	case 'I':
		return [][]bool{
			{T, T, T, T, T},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{T, T, T, T, T},
		}
	case 'L':
		return [][]bool{
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, T, T, T, T},
		}
	case 'P':
		return [][]bool{
			{T, T, T, T, F},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, T, T, T, F},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{T, F, F, F, F},
		}
	case 'T':
		return [][]bool{
			{T, T, T, T, T},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
		}
	case 'Y':
		return [][]bool{
			{T, F, F, F, T},
			{T, F, F, F, T},
			{F, T, F, T, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
			{F, F, T, F, F},
		}
	case 'H':
		return [][]bool{
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, T, T, T, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
		}
	case 'K':
		return [][]bool{
			{T, F, F, F, T},
			{T, F, F, T, F},
			{T, F, T, F, F},
			{T, T, F, F, F},
			{T, F, T, F, F},
			{T, F, F, T, F},
			{T, F, F, F, T},
		}
	case 'N':
		return [][]bool{
			{T, F, F, F, T},
			{T, T, F, F, T},
			{T, F, T, F, T},
			{T, F, F, T, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
		}
	case 'O':
		return [][]bool{
			{F, T, T, T, F},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{F, T, T, T, F},
		}
	case 'R':
		return [][]bool{
			{T, T, T, T, F},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, T, T, T, F},
			{T, F, T, F, F},
			{T, F, F, T, F},
			{T, F, F, F, T},
		}
	case 'S':
		return [][]bool{
			{F, T, T, T, T},
			{T, F, F, F, F},
			{T, F, F, F, F},
			{F, T, T, T, F},
			{F, F, F, F, T},
			{F, F, F, F, T},
			{T, T, T, T, F},
		}
	case 'U':
		return [][]bool{
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{F, T, T, T, F},
		}
	case 'W':
		return [][]bool{
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, F, F, T},
			{T, F, T, F, T},
			{T, F, T, F, T},
			{T, T, F, T, T},
			{T, F, F, F, T},
		}
	case 'X':
		return [][]bool{
			{T, F, F, F, T},
			{T, F, F, F, T},
			{F, T, F, T, F},
			{F, F, T, F, F},
			{F, T, F, T, F},
			{T, F, F, F, T},
			{T, F, F, F, T},
		}
	default:
		// 未定义字符：绘制实心方块
		return [][]bool{
			{T, T, T, T, T},
			{T, T, T, T, T},
			{T, T, T, T, T},
			{T, T, T, T, T},
			{T, T, T, T, T},
			{T, T, T, T, T},
			{T, T, T, T, T},
		}
	}
}

// isEmojiRune 判断一个 rune 是否属于 emoji 或特殊符号区域。
// golang.org/x/image/font/opentype 不支持彩色 emoji 字体（CBDT/CBLC），
// 中文系统字体也不包含 emoji 字形，渲染时会输出乱码方块，因此需要提前过滤。
func isEmojiRune(r rune) bool {
	return (r >= 0x1F000 && r <= 0x1FFFF) || // 表情符号主区（😀🎉🐦等）
		(r >= 0x2600 && r <= 0x27BF) || // 杂项符号（☀️✨⭐等）
		(r >= 0x2300 && r <= 0x23FF) || // 技术符号（⏰⌛等）
		(r >= 0xFE00 && r <= 0xFE0F) || // 变体选择符（emoji 修饰符）
		(r >= 0x1F900 && r <= 0x1F9FF) || // 补充符号和象形文字
		(r >= 0x1FA00 && r <= 0x1FA6F) || // 象棋符号等
		(r >= 0x1FA70 && r <= 0x1FAFF) || // 更多扩展 emoji
		r == 0x200D || // 零宽连接符（ZWJ，用于组合 emoji）
		r == 0xFE0F // emoji 变体选择符
}

// stripEmoji 移除文本中所有 emoji 及不可渲染的特殊符号，保留普通文字内容。
// 使用场景：中文/英文字体不支持 emoji，渲染时会产生乱码方块，直接移除比替换为空格更干净。
func stripEmoji(text string) string {
	var result []rune
	for _, r := range text {
		if !isEmojiRune(r) {
			result = append(result, r)
		}
	}
	return string(result)
}

// sanitizeForFont 将文本中字体无法渲染的字符移除，避免 BoundString panic 和乱码方块。
// 先移除 emoji（字体必然不支持），再逐字符探测剩余字符，宽度为 0 时视为不支持并移除。
func sanitizeForFont(text string, face font.Face) string {
	// 先移除 emoji，避免后续 BoundString 对 emoji 产生 panic 或错误宽度
	text = stripEmoji(text)
	var result []rune
	for _, r := range text {
		if r == '\n' || r == ' ' {
			result = append(result, r)
			continue
		}
		// safeBoundString 内部已有 recover，宽度为 0 表示字体不支持该字符
		if safeBoundString(face, string(r)) > 0 {
			result = append(result, r)
		}
	}
	return string(result)
}

// wrapDialogText 将消息文本按对话框可用宽度换行，返回行列表
// maxPixelWidth 为可用像素宽度，face 为字体
func wrapDialogText(content string, face font.Face, maxPixelWidth int) []string {
	if face == nil || content == "" {
		return []string{content}
	}

	// 预处理：将字体不支持的字符替换为空格，防止 BoundString panic
	content = sanitizeForFont(content, face)

	var lines []string
	runes := []rune(content)
	lineStart := 0
	for lineStart < len(runes) {
		// 先检查是否有显式换行符
		explicitBreak := -1
		for i := lineStart; i < len(runes); i++ {
			if runes[i] == '\n' {
				explicitBreak = i
				break
			}
		}

		// 二分查找当前行能容纳的最大字符数
		lineEnd := len(runes)
		if explicitBreak >= 0 {
			lineEnd = explicitBreak
		}

		// 从 lineEnd 往前收缩，直到宽度合适
		for lineEnd > lineStart+1 {
			candidate := string(runes[lineStart:lineEnd])
			width := safeBoundString(face, candidate)
			if width <= maxPixelWidth {
				break
			}
			lineEnd--
		}

		// 如果有显式换行符且在宽度限制内，直接用到换行符处
		if explicitBreak >= 0 && lineEnd >= explicitBreak {
			lineEnd = explicitBreak
		}

		lines = append(lines, string(runes[lineStart:lineEnd]))
		lineStart = lineEnd
		// 跳过换行符
		if lineStart < len(runes) && runes[lineStart] == '\n' {
			lineStart++
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// parseMdInlineSpans 解析一行文本中的行内 Markdown 标记（加粗 **、行内代码 `）
// 返回带样式的片段列表，用于逐段渲染不同颜色
func parseMdInlineSpans(line string) []mdSpan {
	var spans []mdSpan
	runes := []rune(line)
	i := 0
	var current []rune

	flushCurrent := func() {
		if len(current) > 0 {
			spans = append(spans, mdSpan{text: string(current), spanType: mdSpanText})
			current = current[:0]
		}
	}

	for i < len(runes) {
		// 行内代码：`code`
		if runes[i] == '`' {
			flushCurrent()
			i++
			codeStart := i
			for i < len(runes) && runes[i] != '`' {
				i++
			}
			if i < len(runes) {
				spans = append(spans, mdSpan{text: string(runes[codeStart:i]), spanType: mdSpanInline})
				i++ // 跳过结尾 `
			} else {
				// 未闭合，当普通文本处理
				current = append(current, '`')
				current = append(current, runes[codeStart:]...)
			}
			continue
		}

		// 加粗：**text**
		if i+1 < len(runes) && runes[i] == '*' && runes[i+1] == '*' {
			flushCurrent()
			i += 2
			boldStart := i
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '*') {
				i++
			}
			if i+1 < len(runes) {
				spans = append(spans, mdSpan{text: string(runes[boldStart:i]), spanType: mdSpanBold})
				i += 2 // 跳过结尾 **
			} else {
				// 未闭合，当普通文本处理
				current = append(current, '*', '*')
				current = append(current, runes[boldStart:]...)
				i = len(runes)
			}
			continue
		}

		current = append(current, runes[i])
		i++
	}
	flushCurrent()

	if len(spans) == 0 {
		spans = []mdSpan{{text: line, spanType: mdSpanText}}
	}
	return spans
}

// parseMdLines 将 Markdown 文本解析为带类型的行列表
// 支持：标题（#/##/###）、代码块（```）、列表（-/*）、引用（>）、分割线（---）
func parseMdLines(content string) []mdLine {
	rawLines := strings.Split(content, "\n")
	var result []mdLine
	inCodeFence := false

	for _, rawLine := range rawLines {
		trimmed := strings.TrimRight(rawLine, " \t")

		// 代码块围栏检测（``` 开头）
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeFence {
				inCodeFence = true
				lang := strings.TrimPrefix(trimmed, "```")
				result = append(result, mdLine{lineType: mdLineCodeFence, rawText: lang})
			} else {
				inCodeFence = false
				result = append(result, mdLine{lineType: mdLineCodeFence, rawText: ""})
			}
			continue
		}

		// 代码块内容（原样保留，不解析 Markdown）
		if inCodeFence {
			result = append(result, mdLine{lineType: mdLineCode, rawText: rawLine})
			continue
		}

		// 分割线（--- / *** / ___，去空格后全为同一符号）
		stripped := strings.ReplaceAll(strings.TrimSpace(trimmed), " ", "")
		if len(stripped) >= 3 && (stripped == strings.Repeat("-", len(stripped)) ||
			stripped == strings.Repeat("*", len(stripped)) ||
			stripped == strings.Repeat("_", len(stripped))) {
			result = append(result, mdLine{lineType: mdLineDivider, rawText: ""})
			continue
		}

		// 标题（# / ## / ###）
		if strings.HasPrefix(trimmed, "### ") {
			body := strings.TrimPrefix(trimmed, "### ")
			result = append(result, mdLine{lineType: mdLineHeading, headLevel: 3, spans: parseMdInlineSpans(body), rawText: body})
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			body := strings.TrimPrefix(trimmed, "## ")
			result = append(result, mdLine{lineType: mdLineHeading, headLevel: 2, spans: parseMdInlineSpans(body), rawText: body})
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			body := strings.TrimPrefix(trimmed, "# ")
			result = append(result, mdLine{lineType: mdLineHeading, headLevel: 1, spans: parseMdInlineSpans(body), rawText: body})
			continue
		}

		// 引用（>）
		if strings.HasPrefix(trimmed, "> ") {
			body := strings.TrimPrefix(trimmed, "> ")
			result = append(result, mdLine{lineType: mdLineQuote, spans: parseMdInlineSpans(body), rawText: body})
			continue
		}

		// 无序列表（- / * / +，后跟空格）
		if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
			body := trimmed[2:]
			result = append(result, mdLine{lineType: mdLineListItem, listPrefix: "• ", spans: parseMdInlineSpans(body), rawText: body})
			continue
		}

		// 有序列表（1. / 2. 等，最多 3 位数字）
		isOrderedList := false
		for digitEnd := 1; digitEnd <= 3 && digitEnd < len(trimmed); digitEnd++ {
			if trimmed[digitEnd-1] < '0' || trimmed[digitEnd-1] > '9' {
				break
			}
			if digitEnd < len(trimmed) && trimmed[digitEnd] == '.' &&
				digitEnd+1 < len(trimmed) && trimmed[digitEnd+1] == ' ' {
				prefix := trimmed[:digitEnd+2]
				body := trimmed[digitEnd+2:]
				result = append(result, mdLine{lineType: mdLineListItem, listPrefix: prefix, spans: parseMdInlineSpans(body), rawText: body})
				isOrderedList = true
				break
			}
		}
		if isOrderedList {
			continue
		}

		// 普通文本（含行内 Markdown）
		result = append(result, mdLine{lineType: mdLineNormal, spans: parseMdInlineSpans(trimmed), rawText: trimmed})
	}

	return result
}

// wrapMdLines 将 parseMdLines 解析出的行列表按最大像素宽度换行，返回可直接渲染的行列表
// 每行保留 lineType，代码块行原样保留，其他行按 rawText 换行后重新生成 spans
func wrapMdLines(lines []mdLine, face font.Face, maxPixelWidth int) []mdLine {
	var result []mdLine
	for _, line := range lines {
		switch line.lineType {
		case mdLineDivider:
			// 分割线不需要换行
			result = append(result, line)

		case mdLineCodeFence:
			// 围栏行（``` lang）直接保留，不渲染内容
			result = append(result, line)

		case mdLineCode:
			// 代码块内容：按宽度换行，留出左边距（4px 代码缩进）
			wrappedTexts := wrapDialogText(line.rawText, face, maxPixelWidth-4)
			for _, t := range wrappedTexts {
				result = append(result, mdLine{lineType: mdLineCode, rawText: t})
			}

		default:
			// 普通/标题/列表/引用：按 rawText 换行，续行缩进
			prefixWidth := 0
			if line.lineType == mdLineListItem && face != nil {
				prefixWidth = safeBoundString(face, line.listPrefix)
			}
			availWidth := maxPixelWidth - prefixWidth
			if availWidth < 40 {
				availWidth = 40
			}
			wrappedTexts := wrapDialogText(line.rawText, face, availWidth)
			for i, t := range wrappedTexts {
				newLine := mdLine{
					lineType:  line.lineType,
					headLevel: line.headLevel,
					rawText:   t,
					spans:     []mdSpan{{text: t, spanType: mdSpanText}},
				}
				if line.lineType == mdLineListItem {
					if i == 0 {
						newLine.listPrefix = line.listPrefix
					} else {
						// 续行用空格缩进，与首行前缀等宽
						newLine.listPrefix = strings.Repeat(" ", len([]rune(line.listPrefix)))
					}
				}
				result = append(result, newLine)
			}
		}
	}
	return result
}

// drawChatDialog 绘制宠物旁边的沉浸式 AI 对话框
// 支持历史消息多行自动换行、Markdown 富文本渲染、流式 token 实时追加、打字机效果和输入框
// 返回对话框的实际渲染边界 (left, top, right, bottom) 和清空按钮边界，供点击检测使用
func drawChatDialog(
	screen *ebiten.Image,
	face font.Face,
	petX, petY float64,
	screenWidth float64,
	messages []chatDialogMessage,
	inputText string,
	isThinking bool,
	thinkingTick int,
	typingCurrent string,
	typingTarget string,
	streamingText string,
	isStreaming bool,
	cursorTick int,
	scrollOffset int,
) (left, top, right, bottom, clearLeft, clearTop, clearRight, clearBottom float32) {
	const (
		cornerRadius = 10
		msgAreaPadH  = chatDialogPaddingH
		msgAreaPadV  = chatDialogPaddingV
		inputAreaH   = chatDialogInputHeight + chatDialogPaddingV*2
		titleBarH    = 24
		// 消息区可用像素宽度：对话框宽度 - 左右内边距 - 前缀宽度预留
		msgTextMaxWidth = chatDialogWidth - chatDialogPaddingH*2 - 8
		// 消息区固定显示行数（超出时通过滚动查看）
		visibleMsgLines = 10
	)

	// ── 颜色定义 ──
	bgColor := color.RGBA{12, 18, 38, 220}
	borderColor := color.RGBA{80, 160, 255, 200}
	titleBgColor := color.RGBA{20, 40, 80, 240}
	userMsgColor := color.RGBA{100, 220, 255, 255}
	aiMsgColor := color.RGBA{200, 240, 200, 255}
	streamingColor := color.RGBA{160, 255, 180, 255}
	inputBgColor := color.RGBA{8, 15, 35, 240}
	inputBorderColor := color.RGBA{60, 140, 255, 200}
	inputTextColor := color.RGBA{220, 240, 255, 255}
	thinkingColor := color.RGBA{180, 120, 255, 255}
	hintColor := color.RGBA{100, 130, 180, 180}
	cursorColor := color.RGBA{100, 200, 255, 255}
	streamingCursorColor := color.RGBA{80, 255, 160, 200}
	// Markdown 专用颜色
	mdHeading1Color := color.RGBA{255, 220, 100, 255}   // 一级标题：金色
	mdHeading2Color := color.RGBA{255, 180, 80, 255}    // 二级标题：橙色
	mdHeading3Color := color.RGBA{220, 160, 60, 255}    // 三级标题：深橙
	mdBoldColor := color.RGBA{240, 255, 200, 255}       // 加粗：亮绿白
	mdInlineCodeColor := color.RGBA{180, 240, 255, 255} // 行内代码：青色
	mdCodeBgColor := color.RGBA{20, 30, 50, 200}        // 代码块背景
	mdCodeTextColor := color.RGBA{160, 220, 160, 255}   // 代码块文字：绿色
	mdQuoteColor := color.RGBA{160, 180, 220, 200}      // 引用：淡蓝
	mdListColor := color.RGBA{200, 240, 200, 255}       // 列表：同 AI 消息色
	mdDividerColor := color.RGBA{60, 100, 160, 180}     // 分割线

	// ── 预计算所有消息的 Markdown 渲染行 ──

	// renderLine 是最终渲染单元，每行可能由多个 span 组成
	type renderLine struct {
		// 对于普通/标题/列表/引用行：用 spans 渲染
		spans     []mdSpan
		baseColor color.RGBA // spans 中 mdSpanText 使用的基础颜色
		lineType  mdLineType
		// 对于代码块行：用 rawText 渲染
		rawText string
		// 列表前缀
		listPrefix string
		// 是否是流式/打字机活跃行（用于流式光标判断）
		isActive bool
		isLast   bool
	}

	var allRenderLines []renderLine

	if face != nil {
		// 收集历史消息行（AI 回复走 Markdown 解析，用户消息直接换行）
		for _, msg := range messages {
			if msg.isUser {
				// 用户消息：直接换行，带"你: "前缀
				wrappedTexts := wrapDialogText(msg.content, face, msgTextMaxWidth-safeBoundString(face, "你: "))
				for i, t := range wrappedTexts {
					prefix := ""
					if i == 0 {
						prefix = "你: "
					} else {
						prefix = "    "
					}
					allRenderLines = append(allRenderLines, renderLine{
						spans:     []mdSpan{{text: prefix + t, spanType: mdSpanText}},
						baseColor: userMsgColor,
						lineType:  mdLineNormal,
					})
				}
			} else {
				// AI 回复：解析 Markdown，换行后渲染
				mdLines := parseMdLines(msg.content)
				wrappedMdLines := wrapMdLines(mdLines, face, msgTextMaxWidth)

				// 在 AI 消息前加一个"AI: "前缀行（仅第一行）
				firstNonFence := true
				for _, mdl := range wrappedMdLines {
					// 代码围栏行不渲染为可见行，跳过
					if mdl.lineType == mdLineCodeFence {
						continue
					}

					rl := renderLine{lineType: mdl.lineType, rawText: mdl.rawText, listPrefix: mdl.listPrefix}

					switch mdl.lineType {
					case mdLineDivider:
						rl.baseColor = mdDividerColor
						rl.spans = []mdSpan{{text: "", spanType: mdSpanText}}

					case mdLineCode:
						rl.baseColor = mdCodeTextColor
						rl.spans = []mdSpan{{text: mdl.rawText, spanType: mdSpanText}}

					case mdLineHeading:
						switch mdl.headLevel {
						case 1:
							rl.baseColor = mdHeading1Color
						case 2:
							rl.baseColor = mdHeading2Color
						default:
							rl.baseColor = mdHeading3Color
						}
						rl.spans = mdl.spans

					case mdLineQuote:
						rl.baseColor = mdQuoteColor
						rl.spans = mdl.spans

					case mdLineListItem:
						rl.baseColor = mdListColor
						rl.spans = mdl.spans

					default: // mdLineNormal
						rl.baseColor = aiMsgColor
						rl.spans = mdl.spans
					}

					// 第一个可见行加"AI: "前缀
					if firstNonFence {
						firstNonFence = false
						// 将前缀作为第一个 span 插入
						prefixSpan := mdSpan{text: "AI: ", spanType: mdSpanText}
						rl.spans = append([]mdSpan{prefixSpan}, rl.spans...)
					}

					allRenderLines = append(allRenderLines, rl)
				}
			}
		}

		// 收集流式/打字机活跃文本行（实时接收中，不做完整 Markdown 解析，直接换行展示）
		activeText := ""
		if isStreaming && streamingText != "" {
			activeText = streamingText
		} else if typingTarget != "" && typingCurrent != "" {
			activeText = typingCurrent
		}
		if activeText != "" {
			activeWrapped := wrapDialogText(activeText, face, msgTextMaxWidth-safeBoundString(face, "AI: "))
			activeColor := aiMsgColor
			if isStreaming {
				activeColor = streamingColor
			}
			for i, t := range activeWrapped {
				prefix := ""
				if i == 0 {
					prefix = "AI: "
				} else {
					prefix = "    "
				}
				isLast := i == len(activeWrapped)-1
				allRenderLines = append(allRenderLines, renderLine{
					spans:     []mdSpan{{text: prefix + t, spanType: mdSpanText}},
					baseColor: activeColor,
					lineType:  mdLineNormal,
					isActive:  true,
					isLast:    isLast,
				})
			}
		}

		// 收集思考动画行
		if isThinking {
			dotCount := (thinkingTick / 20) % 4
			dots := strings.Repeat(".", dotCount)
			allRenderLines = append(allRenderLines, renderLine{
				spans:     []mdSpan{{text: "AI: 思考中" + dots, spanType: mdSpanText}},
				baseColor: thinkingColor,
				lineType:  mdLineNormal,
				isActive:  true,
				isLast:    true,
			})
		}
	}

	// ── 计算滚动范围 ──
	totalMsgLines := len(allRenderLines)
	msgAreaH := visibleMsgLines * chatDialogLineHeight
	maxScrollLines := totalMsgLines - visibleMsgLines
	if maxScrollLines < 0 {
		maxScrollLines = 0
	}
	clampedScroll := scrollOffset
	if clampedScroll > maxScrollLines {
		clampedScroll = maxScrollLines
	}
	if clampedScroll < 0 {
		clampedScroll = 0
	}

	// ── 对话框尺寸和定位 ──
	dialogH := float32(titleBarH + msgAreaPadV + msgAreaH + msgAreaPadV + inputAreaH)
	dialogW := float32(chatDialogWidth)

	dialogRight := float32(petX) - 12
	dialogLeft := dialogRight - dialogW
	dialogBottom := float32(petY) + float32(spriteHeight)
	dialogTop := dialogBottom - dialogH

	if dialogLeft < 5 {
		dialogLeft = 5
		dialogRight = dialogLeft + dialogW
	}
	if dialogTop < 5 {
		dialogTop = 5
		dialogBottom = dialogTop + dialogH
	}

	// ── 绘制主背景和边框 ──
	drawRoundRect(screen, dialogLeft, dialogTop, dialogW, dialogH, cornerRadius, bgColor)
	vector.StrokeRect(screen, dialogLeft+1, dialogTop+1, dialogW-2, dialogH-2, 1.5, borderColor, true)

	// ── 绘制标题栏 ──
	drawRoundRect(screen, dialogLeft, dialogTop, dialogW, float32(titleBarH), cornerRadius, titleBgColor)
	vector.StrokeLine(screen, dialogLeft, dialogTop+float32(titleBarH), dialogRight, dialogTop+float32(titleBarH), 1, borderColor, true)

	clearBtnW := float32(32)
	clearBtnH := float32(titleBarH - 6)
	clearBtnRight := dialogRight - float32(msgAreaPadH)
	clearBtnLeft := clearBtnRight - clearBtnW
	clearBtnTop := dialogTop + 3
	clearBtnBottom := clearBtnTop + clearBtnH

	if face != nil {
		titleX := int(dialogLeft) + msgAreaPadH
		titleY := int(dialogTop) + titleBarH - 6
		text.Draw(screen, "DingTalk AI", face, titleX, titleY, color.RGBA{180, 220, 255, 255})

		clearBtnBg := color.RGBA{60, 30, 80, 200}
		drawRoundRect(screen, clearBtnLeft, clearBtnTop, clearBtnW, clearBtnH, 4, clearBtnBg)
		vector.StrokeRect(screen, clearBtnLeft, clearBtnTop, clearBtnW, clearBtnH, 1, color.RGBA{180, 80, 255, 180}, true)
		clearTextX := int(clearBtnLeft) + 4
		clearTextY := int(clearBtnTop) + int(clearBtnH) - 3
		text.Draw(screen, "清空", face, clearTextX, clearTextY, color.RGBA{220, 160, 255, 255})

		hintX := int(clearBtnLeft) - 56
		text.Draw(screen, "ESC关闭", face, hintX, titleY, hintColor)
	}

	// ── 绘制消息区 ──
	msgAreaTop := int(dialogTop) + titleBarH + msgAreaPadV
	currentLineY := msgAreaTop
	msgX := int(dialogLeft) + msgAreaPadH

	inputBoxTop := dialogBottom - float32(inputAreaH)
	msgAreaBottom := int(inputBoxTop) - msgAreaPadV

	if face != nil {
		var lastActiveLineY int
		var lastActiveLineText string

		for i := clampedScroll; i < len(allRenderLines); i++ {
			rl := allRenderLines[i]
			lineY := currentLineY + chatDialogLineHeight - 2
			if lineY > msgAreaBottom {
				break
			}

			switch rl.lineType {
			case mdLineDivider:
				// 分割线：绘制一条横线
				divY := float32(currentLineY + chatDialogLineHeight/2)
				vector.StrokeLine(screen,
					float32(msgX), divY,
					float32(msgX)+dialogW-float32(msgAreaPadH*2), divY,
					1, mdDividerColor, true,
				)

			case mdLineCode:
				// 代码块：绘制背景色 + 等宽文字
				codeBgX := float32(msgX - 2)
				codeBgY := float32(currentLineY)
				codeBgW := dialogW - float32(msgAreaPadH*2) + 4
				codeBgH := float32(chatDialogLineHeight)
				vector.DrawFilledRect(screen, codeBgX, codeBgY, codeBgW, codeBgH, mdCodeBgColor, true)
				codeText := sanitizeForFont(rl.rawText, face)
				text.Draw(screen, codeText, face, msgX+2, lineY, mdCodeTextColor)

			default:
				// 普通/标题/列表/引用：逐 span 渲染，不同 span 用不同颜色
				drawX := msgX

				// 列表前缀
				if rl.lineType == mdLineListItem && rl.listPrefix != "" {
					prefixText := sanitizeForFont(rl.listPrefix, face)
					text.Draw(screen, prefixText, face, drawX, lineY, rl.baseColor)
					drawX += safeBoundString(face, prefixText)
				}

				// 逐 span 渲染
				for _, span := range rl.spans {
					spanText := sanitizeForFont(span.text, face)
					if spanText == "" {
						continue
					}
					var spanColor color.RGBA
					switch span.spanType {
					case mdSpanBold:
						spanColor = mdBoldColor
					case mdSpanInline:
						spanColor = mdInlineCodeColor
					default:
						spanColor = rl.baseColor
					}
					text.Draw(screen, spanText, face, drawX, lineY, spanColor)
					drawX += safeBoundString(face, spanText)
				}
			}

			if rl.isActive && rl.isLast {
				lastActiveLineY = currentLineY
				// 拼接最后一行的完整文本，用于计算光标位置
				fullText := ""
				if rl.lineType == mdLineListItem && rl.listPrefix != "" {
					fullText += rl.listPrefix
				}
				for _, span := range rl.spans {
					fullText += span.text
				}
				lastActiveLineText = fullText
			}
			currentLineY += chatDialogLineHeight
		}

		// 流式模式：在最后一行末尾绘制闪烁光标
		if (isStreaming || isThinking) && lastActiveLineText != "" && (cursorTick/20)%2 == 0 {
			cursorOffsetX := safeBoundString(face, sanitizeForFont(lastActiveLineText, face))
			cursorX := float32(msgX + cursorOffsetX + 1)
			cursorY1 := float32(lastActiveLineY + 2)
			cursorY2 := float32(lastActiveLineY + chatDialogLineHeight - 2)
			vector.StrokeLine(screen, cursorX, cursorY1, cursorX, cursorY2, 1.5, streamingCursorColor, true)
		}

		// 滚动指示器
		if maxScrollLines > 0 {
			scrollBarX := dialogRight - 4
			scrollBarTop := float32(msgAreaTop)
			scrollBarBottom := float32(msgAreaBottom)
			scrollBarH := scrollBarBottom - scrollBarTop
			thumbRatio := float32(visibleMsgLines) / float32(totalMsgLines)
			thumbH := scrollBarH * thumbRatio
			if thumbH < 8 {
				thumbH = 8
			}
			thumbOffset := (scrollBarH - thumbH) * float32(clampedScroll) / float32(maxScrollLines)
			vector.DrawFilledRect(screen, scrollBarX, scrollBarTop+thumbOffset, 3, thumbH, color.RGBA{100, 160, 255, 120}, true)
		}
	}

	// ── 绘制输入框 ──
	vector.StrokeLine(screen, dialogLeft, inputBoxTop, dialogRight, inputBoxTop, 1, borderColor, true)
	drawRoundRect(screen,
		dialogLeft+float32(msgAreaPadH), inputBoxTop+float32(msgAreaPadV),
		dialogW-float32(msgAreaPadH*2), float32(chatDialogInputHeight),
		6, inputBgColor,
	)
	vector.StrokeRect(screen,
		dialogLeft+float32(msgAreaPadH), inputBoxTop+float32(msgAreaPadV),
		dialogW-float32(msgAreaPadH*2), float32(chatDialogInputHeight),
		1, inputBorderColor, true,
	)

	if face != nil {
		inputTextY := int(inputBoxTop) + msgAreaPadV + chatDialogInputHeight - 9
		inputTextX := int(dialogLeft) + msgAreaPadH*2

		displayInput := sanitizeForFont(inputText, face)
		maxInputWidth := chatDialogWidth - msgAreaPadH*4
		inputRunes := []rune(displayInput)
		for len(inputRunes) > 1 {
			if safeBoundString(face, string(inputRunes)) <= maxInputWidth {
				break
			}
			inputRunes = inputRunes[1:]
		}
		displayInput = string(inputRunes)

		if displayInput == "" && !isThinking && !isStreaming {
			text.Draw(screen, "输入消息，Enter 发送…", face, inputTextX, inputTextY, hintColor)
		} else {
			text.Draw(screen, displayInput, face, inputTextX, inputTextY, inputTextColor)
		}

		// 光标闪烁（思考/流式中不显示）
		if (cursorTick/30)%2 == 0 && !isThinking && !isStreaming {
			cursorOffsetX := 0
			if displayInput != "" {
				cursorOffsetX = safeBoundString(face, displayInput)
			}
			cursorX := inputTextX + cursorOffsetX
			cursorY1 := float32(inputTextY - chatDialogLineHeight + 4)
			cursorY2 := float32(inputTextY + 2)
			vector.StrokeLine(screen, float32(cursorX), cursorY1, float32(cursorX), cursorY2, 1.5, cursorColor, true)
		}
	}

	// ── 绘制连接宠物的小圆点尾巴 ──
	tailTipX := float32(petX) - 4
	tailTipY := float32(petY) + float32(spriteHeight)/2
	vector.DrawFilledCircle(screen, tailTipX, tailTipY, 4, borderColor, true)

	return dialogLeft, dialogTop, dialogRight, dialogBottom, clearBtnLeft, clearBtnTop, clearBtnRight, clearBtnBottom
}
