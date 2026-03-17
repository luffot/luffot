package barrage

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"github.com/luffot/luffot/pkg/config"
	"github.com/luffot/luffot/pkg/pet"
)

// urlRegexp 匹配 URL 的正则表达式
var urlRegexp = regexp.MustCompile(`https?://[^\s]+`)

// replaceURLsWithEmoji 将文本中的 URL 替换为 [🔗]
func replaceURLsWithEmoji(text string) string {
	return urlRegexp.ReplaceAllString(text, "[🔗]")
}

// defaultBarrageColor 默认弹幕颜色：蓝色
var defaultBarrageColor = color.RGBA{0, 120, 255, 255}

// BarrageDisplayConfig 弹幕显示配置
type BarrageDisplayConfig struct {
	ScreenWidth  int
	ScreenHeight int
	FontSize     int
	TrackHeight  int
	MaxTracks    int
	BgColor      color.RGBA
	ShowBorder   bool
}

// barrageItem 单条弹幕
type barrageItem struct {
	displayText string
	x           float64
	y           float64
	speed       float64
	textColor   color.RGBA
	// 头像相关
	avatarImage *ebiten.Image // 已缩放的圆形头像，nil 表示无头像或尚未加载完成
	avatarSize  int           // 头像边长（正方形）
}

// avatarCacheEntry 头像缓存条目
type avatarCacheEntry struct {
	image *ebiten.Image
	ready bool // 是否已加载完成
}

// basketballSprite 全屏篮球精灵，用于蔡徐坤皮肤 alert 状态的抛物线动画
// 篮球从桌宠位置出发，沿贝塞尔曲线飞向屏幕正中心，由小变大，到达后消失
type basketballSprite struct {
	active   bool    // 是否正在播放
	progress float64 // 动画进度 0.0~1.0
	// 起点（桌宠中心）
	startX float64
	startY float64
	// 贝塞尔控制点（弧顶，在起点和终点之间偏上方）
	controlX float64
	controlY float64
	// 终点（屏幕正中心）
	endX float64
	endY float64
	// 当前帧旋转角度
	rotation float64
}

// BarrageDisplay 弹幕显示器，同时实现 ebiten.Game 接口
type BarrageDisplay struct {
	mu           sync.Mutex
	items        []*barrageItem
	screenWidth  int
	screenHeight int
	trackHeight  int
	maxTracks    int
	fontFace     font.Face
	emojiFace    font.Face // emoji 专用字体，用于渲染 emoji 字符
	fontSize     int

	// 头像缓存：URL -> 已处理的圆形头像
	avatarCacheMu sync.RWMutex
	avatarCache   map[string]*avatarCacheEntry

	// 桌面宠物精灵（钉钉小蜜蜂）
	petSprite *pet.PetSprite

	// 弹幕可见性，Shift+Alt+T 切换（默认可见）
	barrageVisible bool

	// 告警遮罩动画帧计数（用于边缘红色渐变闪烁效果）
	alertOverlayTick int

	// 上一次应用的皮肤名称，用于检测配置变更后实时切换
	lastAppliedSkinName string

	// 全屏篮球精灵（蔡徐坤皮肤 alert 状态专用）
	basketball basketballSprite
	// 上一帧是否处于 alert 状态，用于检测 alert 开始时机
	wasAlertActive bool
}

// NewBarrageDisplay 创建弹幕显示器
func NewBarrageDisplay(config BarrageDisplayConfig) *BarrageDisplay {
	maxTracks := config.MaxTracks
	if maxTracks <= 0 && config.TrackHeight > 0 {
		maxTracks = config.ScreenHeight / config.TrackHeight
	}
	if maxTracks <= 0 {
		maxTracks = 8
	}

	fontSize := config.FontSize
	if fontSize <= 0 {
		fontSize = 32
	}

	trackHeight := config.TrackHeight
	if trackHeight <= 0 {
		trackHeight = fontSize + 18
	}

	face := loadChineseFontFace(fontSize)
	emojiFace := loadEmojiFontFace(fontSize)

	return &BarrageDisplay{
		items:          make([]*barrageItem, 0),
		screenWidth:    config.ScreenWidth,
		screenHeight:   config.ScreenHeight,
		trackHeight:    trackHeight,
		maxTracks:      maxTracks,
		fontFace:       face,
		emojiFace:      emojiFace,
		fontSize:       fontSize,
		avatarCache:    make(map[string]*avatarCacheEntry),
		petSprite:      pet.NewPetSprite(config.ScreenWidth, config.ScreenHeight),
		barrageVisible: true,
	}
}

// defaultHighlightColor 特别关注弹幕默认高亮颜色：金色
var defaultHighlightColor = color.RGBA{255, 215, 0, 255}

// isBarrageFiltered 检查消息是否命中过滤关键词，命中则不显示弹幕
func isBarrageFiltered(content string) bool {
	barrageCfg := config.GetBarrageConfig()
	if len(barrageCfg.FilterKeywords) == 0 {
		return false
	}
	lowerContent := strings.ToLower(content)
	for _, keyword := range barrageCfg.FilterKeywords {
		if keyword != "" && strings.Contains(lowerContent, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// resolveBarrageHighlightColor 检查消息是否命中特别关注规则，返回对应高亮颜色
// 若未命中任何规则则返回 nil
func resolveBarrageHighlightColor(content string) *color.RGBA {
	barrageCfg := config.GetBarrageConfig()
	if len(barrageCfg.HighlightRules) == 0 {
		return nil
	}
	lowerContent := strings.ToLower(content)
	for _, rule := range barrageCfg.HighlightRules {
		if rule.Keyword != "" && strings.Contains(lowerContent, strings.ToLower(rule.Keyword)) {
			if rule.Color != "" {
				parsed := parseHexColor(rule.Color)
				return &parsed
			}
			return &defaultHighlightColor
		}
	}
	return nil
}

// AddMessage 添加一条弹幕消息（线程安全）
// avatarURL 为空时不显示头像，colorHex 为空时使用默认蓝色
// 若消息命中过滤关键词则静默丢弃；若命中特别关注规则则使用高亮颜色覆盖
func (d *BarrageDisplay) AddMessage(content, sender, app, avatarURL, colorHex string) {
	// 过滤检查：命中过滤关键词则不显示弹幕
	if isBarrageFiltered(content) {
		return
	}

	// 特别关注检查：命中高亮规则则覆盖颜色
	var textColor color.RGBA
	if highlightColor := resolveBarrageHighlightColor(content); highlightColor != nil {
		textColor = *highlightColor
	} else {
		textColor = parseHexColor(colorHex)
	}

	avatarSize := d.fontSize + 10
	var avatarImg *ebiten.Image

	if avatarURL != "" {
		// 先检查缓存
		d.avatarCacheMu.RLock()
		entry, cached := d.avatarCache[avatarURL]
		d.avatarCacheMu.RUnlock()

		if cached && entry.ready {
			avatarImg = entry.image
		} else if !cached {
			// 占位，防止重复下载
			d.avatarCacheMu.Lock()
			d.avatarCache[avatarURL] = &avatarCacheEntry{ready: false}
			d.avatarCacheMu.Unlock()

			// 异步下载头像
			go d.fetchAndCacheAvatar(avatarURL, avatarSize)
		}
	}

	// macOS 状态栏高度约 28px，额外留出 10px 间距，避免弹幕被遮挡
	const statusBarOffset = 38

	track := rand.Intn(d.maxTracks)
	yPos := float64(track*d.trackHeight+d.trackHeight) + statusBarOffset
	speed := 3.0 + rand.Float64()*2.0

	// 替换 URL 为 [🔗]
	processedContent := replaceURLsWithEmoji(content)
	item := &barrageItem{
		displayText: sender + ": " + processedContent,
		x:           float64(d.screenWidth + 20),
		y:           yPos,
		speed:       speed,
		textColor:   textColor,
		avatarImage: avatarImg,
		avatarSize:  avatarSize,
	}

	d.mu.Lock()
	d.items = append(d.items, item)
	d.mu.Unlock()
}

// fetchAndCacheAvatar 异步下载头像并写入缓存
func (d *BarrageDisplay) fetchAndCacheAvatar(avatarURL string, size int) {
	resp, err := http.Get(avatarURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	src, _, err := image.Decode(resp.Body)
	if err != nil {
		return
	}

	// 缩放并裁剪为圆形
	circleImg := makeCircleAvatar(src, size)
	ebitenImg := ebiten.NewImageFromImage(circleImg)

	d.avatarCacheMu.Lock()
	d.avatarCache[avatarURL] = &avatarCacheEntry{image: ebitenImg, ready: true}
	d.avatarCacheMu.Unlock()

	// 将已在屏幕上的同 URL 弹幕也更新头像
	d.mu.Lock()
	for _, item := range d.items {
		if item.avatarImage == nil && item.avatarSize == size {
			item.avatarImage = ebitenImg
		}
	}
	d.mu.Unlock()
}

// Update 实现 ebiten.Game 接口，每帧更新弹幕位置和宠物状态
func (d *BarrageDisplay) Update() error {
	// 检测 Shift+Alt+L 全局快捷键，切换弹幕可见性
	if pet.ConsumeToggleBarrage() {
		d.barrageVisible = !d.barrageVisible
		if d.barrageVisible {
			fmt.Println("[BARRAGE] 弹幕已显示（Shift+Alt+L）")
		} else {
			fmt.Println("[BARRAGE] 弹幕已隐藏（Shift+Alt+L）")
		}
	}

	// 检测皮肤配置变更，实时切换桌宠皮肤
	// 空字符串表示使用默认经典皮肤（钉三多（经典黑））
	configuredSkin := config.Get().PetSkin
	if configuredSkin != d.lastAppliedSkinName {
		if configuredSkin == "" {
			pet.SetSkinByName(pet.DefaultSkinName)
		} else {
			pet.SetSkinByName(configuredSkin)
		}
		d.lastAppliedSkinName = configuredSkin
	}

	// 更新宠物状态（键盘/鼠标检测必须在主 goroutine 中执行）
	// 根据宠物是否被拖拽来决定鼠标穿透
	isDragging := d.petSprite.Update()
	ebiten.SetWindowMousePassthrough(!isDragging && !d.petSprite.IsHovered())

	// 全屏篮球精灵：仅在支持篮球特效的图片皮肤处于 alert 状态时触发
	isAlertNow := d.petSprite.IsAlertActive()
	activeSkin := pet.GetActiveImageSkin()
	hasBasketballEffect := activeSkin != nil && activeSkin.FullscreenEffect == pet.FullscreenEffectBasketball
	if hasBasketballEffect && isAlertNow && !d.wasAlertActive {
		// alert 刚开始：初始化篮球精灵，从桌宠中心飞向屏幕正中心
		petX, petY := d.petSprite.GetPosition()
		d.basketball = basketballSprite{
			active:   true,
			progress: 0,
			startX:   petX + 80, // 桌宠中心（精灵宽 160）
			startY:   petY + 80, // 桌宠中心（精灵高 160）
			endX:     float64(d.screenWidth) / 2,
			endY:     float64(d.screenHeight) / 2,
			rotation: 0,
		}
		// 贝塞尔控制点：在起点和终点连线的中点偏上方（形成弧顶）
		midX := (d.basketball.startX + d.basketball.endX) / 2
		midY := (d.basketball.startY + d.basketball.endY) / 2
		d.basketball.controlX = midX
		d.basketball.controlY = midY - float64(d.screenHeight)*0.3
	}
	d.wasAlertActive = isAlertNow

	// 更新篮球精灵动画进度（约 1.5 秒完成，60fps 下 90 帧）
	if d.basketball.active {
		d.basketball.progress += 1.0 / 90.0
		d.basketball.rotation += 8 // 每帧旋转 8 度
		if d.basketball.progress >= 1.0 {
			d.basketball.active = false
			d.basketball.progress = 1.0
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	alive := d.items[:0]
	for _, item := range d.items {
		item.x -= item.speed
		// 估算文字宽度：每个字符约 fontSize * 0.55 像素，加上头像宽度
		textWidth := float64(len([]rune(item.displayText))) * float64(d.fontSize) * 0.55
		totalWidth := textWidth
		if item.avatarImage != nil {
			totalWidth += float64(item.avatarSize) + 6
		}
		if item.x+totalWidth >= 0 {
			alive = append(alive, item)
		}
	}
	d.items = alive
	return nil
}

// Draw 实现 ebiten.Game 接口，每帧绘制弹幕和宠物
func (d *BarrageDisplay) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 0, 0, 0})

	// 仅在弹幕可见时绘制弹幕内容
	if d.barrageVisible {
		d.mu.Lock()
		for _, item := range d.items {
			drawX := int(item.x)
			textY := int(item.y)

			// 绘制头像（垂直居中对齐文字基线）
			if item.avatarImage != nil {
				avatarY := textY - item.avatarSize
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(drawX), float64(avatarY))
				screen.DrawImage(item.avatarImage, op)
				drawX += item.avatarSize + 6
			}

			// 分段渲染：emoji 使用 emoji 字体，其余使用中文字体
			d.drawTextWithEmoji(screen, item.displayText, drawX, textY, item.textColor)
		}
		d.mu.Unlock()
	}

	// 告警状态：在宠物绘制前先绘制屏幕边缘红色渐变遮罩
	if d.petSprite.IsAlertActive() {
		d.alertOverlayTick++
		drawAlertEdgeOverlay(screen, d.screenWidth, d.screenHeight, d.alertOverlayTick)
	} else {
		d.alertOverlayTick = 0
	}

	// 宠物始终绘制（不受弹幕可见性影响）
	d.petSprite.Draw(screen)

	// 全屏篮球精灵：在宠物之上绘制（蔡徐坤皮肤 alert 状态）
	if d.basketball.active {
		drawFullscreenBasketball(screen, &d.basketball)
	}
}

// drawFullscreenBasketball 在全屏坐标系绘制篮球精灵
// 沿二次贝塞尔曲线从桌宠位置飞向屏幕正中心，由小变大，到达后消失
func drawFullscreenBasketball(screen *ebiten.Image, ball *basketballSprite) {
	t := ball.progress
	// 缓动：先快后慢（ease-out）
	easedT := 1 - (1-t)*(1-t)

	// 二次贝塞尔曲线插值位置
	// B(t) = (1-t)^2 * P0 + 2*(1-t)*t * P1 + t^2 * P2
	oneMinusT := 1 - easedT
	cx := oneMinusT*oneMinusT*ball.startX + 2*oneMinusT*easedT*ball.controlX + easedT*easedT*ball.endX
	cy := oneMinusT*oneMinusT*ball.startY + 2*oneMinusT*easedT*ball.controlY + easedT*easedT*ball.endY

	// 篮球半径：由小变大（起点 8px → 终点 50px）
	radius := 8 + easedT*42

	// 在全屏 ebiten.Image 上用 DrawTriangles 绘制篮球
	// 使用 emptyOverlayImage 作为纹理，通过顶点颜色填充
	drawBasketballOnScreen(screen, cx, cy, radius, ball.rotation)
}

// drawBasketballOnScreen 在屏幕坐标系绘制一个篮球（橙色圆形+纹路）
func drawBasketballOnScreen(screen *ebiten.Image, cx, cy, radius, rotationDeg float64) {
	// 用多边形近似圆形（32段）
	const segments = 32
	blankImg := emptyOverlayImage

	// 绘制橙色主体（多个同心圆从深到浅）
	layers := []struct {
		radiusRatio float64
		r, g, b, a  float32
	}{
		{1.0, 0.85, 0.38, 0.05, 1.0},  // 最外层（深橙）
		{0.85, 0.92, 0.45, 0.08, 1.0}, // 中层
		{0.6, 1.0, 0.55, 0.12, 1.0},   // 内层（亮橙）
		{0.3, 1.0, 0.75, 0.35, 0.8},   // 高光
	}

	opts := &ebiten.DrawTrianglesOptions{Blend: ebiten.BlendSourceOver}

	for _, layer := range layers {
		r := radius * layer.radiusRatio
		verts := make([]ebiten.Vertex, segments+1)
		indices := make([]uint16, segments*3)

		// 圆心
		verts[0] = ebiten.Vertex{
			DstX: float32(cx), DstY: float32(cy),
			SrcX: 0, SrcY: 0,
			ColorR: layer.r, ColorG: layer.g, ColorB: layer.b, ColorA: layer.a,
		}
		for i := 0; i < segments; i++ {
			angle := float64(i) / float64(segments) * 2 * math.Pi
			verts[i+1] = ebiten.Vertex{
				DstX: float32(cx + r*math.Cos(angle)),
				DstY: float32(cy + r*math.Sin(angle)),
				SrcX: 0, SrcY: 0,
				ColorR: layer.r * 0.7, ColorG: layer.g * 0.7, ColorB: layer.b * 0.7, ColorA: layer.a,
			}
			indices[i*3] = 0
			indices[i*3+1] = uint16(i + 1)
			indices[i*3+2] = uint16((i+1)%segments + 1)
		}
		screen.DrawTriangles(verts, indices, blankImg, opts)
	}

	// 绘制篮球纹路（黑色弧线，用短线段近似）
	rot := rotationDeg * math.Pi / 180.0
	lineColor := color.RGBA{20, 10, 5, 180}

	// 竖向弧线（经线，两条）
	for _, offset := range []float64{-0.35, 0.35} {
		var prevX, prevY float32
		for i := 0; i <= 20; i++ {
			theta := float64(i)/20.0*math.Pi - math.Pi/2
			px := float32(cx + radius*math.Sin(theta+offset)*math.Cos(rot))
			py := float32(cy + radius*math.Cos(theta)*0.7)
			if i > 0 {
				drawLineOnScreen(screen, prevX, prevY, px, py, float32(radius*0.06+1), lineColor)
			}
			prevX, prevY = px, py
		}
	}

	// 横向弧线（赤道）
	var prevX, prevY float32
	for i := 0; i <= 20; i++ {
		theta := float64(i)/20.0*math.Pi*2 - math.Pi
		px := float32(cx + radius*math.Cos(theta))
		py := float32(cy + radius*0.15*math.Sin(theta+rot))
		if i > 0 {
			drawLineOnScreen(screen, prevX, prevY, px, py, float32(radius*0.06+1), lineColor)
		}
		prevX, prevY = px, py
	}
}

// drawLineOnScreen 在屏幕上绘制一条粗线段（用细长矩形近似）
func drawLineOnScreen(screen *ebiten.Image, x1, y1, x2, y2, width float32, clr color.RGBA) {
	dx := x2 - x1
	dy := y2 - y1
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length < 0.5 {
		return
	}
	nx := -dy / length * width / 2
	ny := dx / length * width / 2

	cr := float32(clr.R) / 255
	cg := float32(clr.G) / 255
	cb := float32(clr.B) / 255
	ca := float32(clr.A) / 255

	verts := []ebiten.Vertex{
		{DstX: x1 + nx, DstY: y1 + ny, SrcX: 0, SrcY: 0, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca},
		{DstX: x1 - nx, DstY: y1 - ny, SrcX: 0, SrcY: 0, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca},
		{DstX: x2 - nx, DstY: y2 - ny, SrcX: 0, SrcY: 0, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca},
		{DstX: x2 + nx, DstY: y2 + ny, SrcX: 0, SrcY: 0, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca},
	}
	indices := []uint16{0, 1, 2, 0, 2, 3}
	opts := &ebiten.DrawTrianglesOptions{Blend: ebiten.BlendSourceOver}
	screen.DrawTriangles(verts, indices, emptyOverlayImage, opts)
}

// Layout 实现 ebiten.Game 接口
func (d *BarrageDisplay) Layout(outsideWidth, outsideHeight int) (int, int) {
	return d.screenWidth, d.screenHeight
}

// GetScreenSize 返回弹幕窗口尺寸（供外部使用）
func GetScreenSize() (width, height int) {
	return ebiten.ScreenSizeInFullscreen()
}

// ShowAlert 触发桌宠对话气泡，展示秘书汇报消息（线程安全）
// 可从任意 goroutine 调用，消息将在下一帧渲染时显示
func (d *BarrageDisplay) ShowAlert(message string) {
	if d.petSprite != nil {
		d.petSprite.ShowAlert(message)
	}
}

// ShowChatReply 将 AI 回复写入桌宠对话框消息列表（流式完成后调用，线程安全）
func (d *BarrageDisplay) ShowChatReply(reply string) {
	if d.petSprite != nil {
		d.petSprite.ShowChatReply(reply)
	}
}

// AppendStreamToken 将流式 token 实时追加到桌宠对话框（线程安全）
func (d *BarrageDisplay) AppendStreamToken(token string) {
	if d.petSprite != nil {
		d.petSprite.AppendStreamToken(token)
	}
}

// SetPetThinking 设置桌宠 AI 思考中状态（线程安全）
func (d *BarrageDisplay) SetPetThinking(thinking bool) {
	if d.petSprite != nil {
		d.petSprite.SetThinking(thinking)
	}
}

// SetPetChatCallback 注入 AI 对话回调到桌宠（用户在对话框提交输入时触发）
func (d *BarrageDisplay) SetPetChatCallback(callback func(input string)) {
	if d.petSprite != nil {
		d.petSprite.SetChatSubmitCallback(callback)
	}
}

// parseHexColor 将 #RRGGBB 或 #RGB 格式颜色字符串解析为 color.RGBA
// 若解析失败则返回默认蓝色
func parseHexColor(hexStr string) color.RGBA {
	if hexStr == "" {
		return defaultBarrageColor
	}

	hexStr = strings.TrimPrefix(hexStr, "#")

	// 支持 #RGB 简写形式，展开为 #RRGGBB
	if len(hexStr) == 3 {
		hexStr = string([]byte{hexStr[0], hexStr[0], hexStr[1], hexStr[1], hexStr[2], hexStr[2]})
	}

	if len(hexStr) != 6 {
		return defaultBarrageColor
	}

	r, errR := strconv.ParseUint(hexStr[0:2], 16, 8)
	g, errG := strconv.ParseUint(hexStr[2:4], 16, 8)
	b, errB := strconv.ParseUint(hexStr[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return defaultBarrageColor
	}

	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}

// makeCircleAvatar 将任意图片缩放并裁剪为圆形，返回 RGBA 图像
func makeCircleAvatar(src image.Image, size int) image.Image {
	// 先缩放到目标尺寸
	scaled := resizeImage(src, size, size)

	// 创建目标 RGBA 图像（透明背景）
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	cx := float64(size) / 2
	cy := float64(size) / 2
	radius := float64(size) / 2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx + 0.5
			dy := float64(y) - cy + 0.5
			if math.Sqrt(dx*dx+dy*dy) <= radius {
				dst.Set(x, y, scaled.At(x, y))
			}
		}
	}
	return dst
}

// resizeImage 使用最近邻插值将图片缩放到指定尺寸
func resizeImage(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Max.X - srcBounds.Min.X
	srcHeight := srcBounds.Max.Y - srcBounds.Min.Y

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), image.Transparent, image.Point{}, draw.Src)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + x*srcWidth/width
			srcY := srcBounds.Min.Y + y*srcHeight/height
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

// loadChineseFontFace 加载支持中文的字体，优先使用系统字体，回退到内嵌英文字体
func loadChineseFontFace(fontSize int) font.Face {
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
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			tt, err := opentype.Parse(data)
			if err != nil {
				continue
			}
			face, err := opentype.NewFace(tt, &opentype.FaceOptions{
				Size:    float64(fontSize),
				DPI:     72,
				Hinting: font.HintingFull,
			})
			if err != nil {
				continue
			}
			return face
		}
		tt, err := collection.Font(0)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(tt, &opentype.FaceOptions{
			Size:    float64(fontSize),
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			continue
		}
		return face
	}

	tt, _ := opentype.Parse(goregular.TTF)
	face, _ := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    float64(fontSize),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	return face
}

// loadEmojiFontFace 加载 Apple Color Emoji 字体，用于渲染 emoji 字符
// 若加载失败则返回 nil，调用方需做 nil 判断
func loadEmojiFontFace(fontSize int) font.Face {
	data, err := os.ReadFile("/System/Library/Fonts/Apple Color Emoji.ttc")
	if err != nil {
		return nil
	}
	collection, err := opentype.ParseCollection(data)
	if err != nil {
		return nil
	}
	tt, err := collection.Font(0)
	if err != nil {
		return nil
	}
	face, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    float64(fontSize),
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil
	}
	return face
}

// textSegment 表示一段连续的文本及其是否为 emoji 段
type textSegment struct {
	text    string
	isEmoji bool
}

// isEmojiRune 判断一个 rune 是否属于 emoji 字符范围
func isEmojiRune(r rune) bool {
	// Emoticons
	if r >= 0x1F600 && r <= 0x1F64F {
		return true
	}
	// Miscellaneous Symbols and Pictographs
	if r >= 0x1F300 && r <= 0x1F5FF {
		return true
	}
	// Transport and Map Symbols
	if r >= 0x1F680 && r <= 0x1F6FF {
		return true
	}
	// Supplemental Symbols and Pictographs
	if r >= 0x1F900 && r <= 0x1F9FF {
		return true
	}
	// Symbols and Pictographs Extended-A
	if r >= 0x1FA00 && r <= 0x1FA6F {
		return true
	}
	if r >= 0x1FA70 && r <= 0x1FAFF {
		return true
	}
	// Dingbats
	if r >= 0x2702 && r <= 0x27B0 {
		return true
	}
	// Miscellaneous Symbols
	if r >= 0x2600 && r <= 0x26FF {
		return true
	}
	// Enclosed Alphanumeric Supplement (keycap emoji etc.)
	if r >= 0x1F1E0 && r <= 0x1F1FF {
		return true
	}
	// Regional indicator symbols (flags)
	if r >= 0x1F1E6 && r <= 0x1F1FF {
		return true
	}
	// Variation selectors (emoji presentation)
	if r == 0xFE0F || r == 0xFE0E {
		return true
	}
	// Zero-width joiner
	if r == 0x200D {
		return true
	}
	return false
}

// splitTextSegments 将文本按 emoji / 非 emoji 分段
func splitTextSegments(s string) []textSegment {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}

	var segments []textSegment
	start := 0
	currentIsEmoji := isEmojiRune(runes[0])

	for i := 1; i < len(runes); i++ {
		segIsEmoji := isEmojiRune(runes[i])
		if segIsEmoji != currentIsEmoji {
			segments = append(segments, textSegment{
				text:    string(runes[start:i]),
				isEmoji: currentIsEmoji,
			})
			start = i
			currentIsEmoji = segIsEmoji
		}
	}
	segments = append(segments, textSegment{
		text:    string(runes[start:]),
		isEmoji: currentIsEmoji,
	})
	return segments
}

// drawTextWithEmoji 分段渲染文本，emoji 字符使用 emoji 字体，其余使用中文字体
// 若 emojiFace 为 nil，则所有字符均使用 fontFace 渲染
func (d *BarrageDisplay) drawTextWithEmoji(screen *ebiten.Image, content string, x, y int, clr color.RGBA) {
	if d.emojiFace == nil {
		text.Draw(screen, content, d.fontFace, x, y, clr)
		return
	}

	segments := splitTextSegments(content)
	currentX := x
	for _, seg := range segments {
		var face font.Face
		if seg.isEmoji {
			face = d.emojiFace
		} else {
			face = d.fontFace
		}
		text.Draw(screen, seg.text, face, currentX, y, clr)
		// 计算该段宽度以推进 x 坐标
		segRunes := []rune(seg.text)
		advance := 0
		for _, r := range segRunes {
			a, ok := face.GlyphAdvance(r)
			if ok {
				advance += int(a >> 6)
			}
		}
		currentX += advance
	}
}

// drawAlertEdgeOverlay 在屏幕四边绘制红色渐变遮罩
// 效果：屏幕边缘红色明显，越往中心越透明，营造紧急告警氛围
// 实现：用四个梯形（上/下/左/右边框带）分别绘制，每个带从边缘（不透明红）到内侧（透明）渐变
// 使用 ebiten.DrawTriangles 的顶点颜色插值实现平滑渐变
func drawAlertEdgeOverlay(screen *ebiten.Image, screenWidth, screenHeight, tick int) {
	// 渐变带宽度：屏幕短边的 25%
	bandWidth := float32(math.Min(float64(screenWidth), float64(screenHeight)) * 0.25)

	// 闪烁效果：边缘透明度随时间脉动（0.55~1.0 之间）
	pulseAlpha := float32(0.55 + 0.45*math.Abs(math.Sin(float64(tick)*0.05)))

	// 边缘颜色：红色，不透明
	edgeAlpha := float32(0.75) * pulseAlpha
	// 内侧颜色：完全透明
	innerAlpha := float32(0)

	sw := float32(screenWidth)
	sh := float32(screenHeight)

	// 使用空白纹理（1x1 白色像素）作为绘制基础
	blankImg := emptyOverlayImage

	// 顶点颜色辅助函数
	makeVertex := func(x, y, r, g, b, a float32) ebiten.Vertex {
		return ebiten.Vertex{
			DstX:   x,
			DstY:   y,
			SrcX:   0,
			SrcY:   0,
			ColorR: r,
			ColorG: g,
			ColorB: b,
			ColorA: a,
		}
	}

	opts := &ebiten.DrawTrianglesOptions{
		Blend: ebiten.BlendSourceOver,
	}

	// 上边渐变带（从顶部向下渐变到透明）
	topVerts := []ebiten.Vertex{
		makeVertex(0, 0, 1, 0, 0, edgeAlpha),
		makeVertex(sw, 0, 1, 0, 0, edgeAlpha),
		makeVertex(sw, bandWidth, 1, 0, 0, innerAlpha),
		makeVertex(0, bandWidth, 1, 0, 0, innerAlpha),
	}
	topIndices := []uint16{0, 1, 2, 0, 2, 3}
	screen.DrawTriangles(topVerts, topIndices, blankImg, opts)

	// 下边渐变带（从底部向上渐变到透明）
	bottomVerts := []ebiten.Vertex{
		makeVertex(0, sh-bandWidth, 1, 0, 0, innerAlpha),
		makeVertex(sw, sh-bandWidth, 1, 0, 0, innerAlpha),
		makeVertex(sw, sh, 1, 0, 0, edgeAlpha),
		makeVertex(0, sh, 1, 0, 0, edgeAlpha),
	}
	bottomIndices := []uint16{0, 1, 2, 0, 2, 3}
	screen.DrawTriangles(bottomVerts, bottomIndices, blankImg, opts)

	// 左边渐变带（从左侧向右渐变到透明）
	leftVerts := []ebiten.Vertex{
		makeVertex(0, 0, 1, 0, 0, edgeAlpha),
		makeVertex(bandWidth, 0, 1, 0, 0, innerAlpha),
		makeVertex(bandWidth, sh, 1, 0, 0, innerAlpha),
		makeVertex(0, sh, 1, 0, 0, edgeAlpha),
	}
	leftIndices := []uint16{0, 1, 2, 0, 2, 3}
	screen.DrawTriangles(leftVerts, leftIndices, blankImg, opts)

	// 右边渐变带（从右侧向左渐变到透明）
	rightVerts := []ebiten.Vertex{
		makeVertex(sw-bandWidth, 0, 1, 0, 0, innerAlpha),
		makeVertex(sw, 0, 1, 0, 0, edgeAlpha),
		makeVertex(sw, sh, 1, 0, 0, edgeAlpha),
		makeVertex(sw-bandWidth, sh, 1, 0, 0, innerAlpha),
	}
	rightIndices := []uint16{0, 1, 2, 0, 2, 3}
	screen.DrawTriangles(rightVerts, rightIndices, blankImg, opts)
}

// emptyOverlayImage 用于渐变遮罩绘制的 1x1 白色纹理
var emptyOverlayImage = func() *ebiten.Image {
	img := ebiten.NewImage(1, 1)
	img.Fill(color.White)
	return img
}()

// RunBarrage 启动弹幕窗口（必须在主 goroutine 调用）
// 窗口覆盖整个屏幕，弹幕在上方区域，宠物在右下角
func RunBarrage(display *BarrageDisplay) error {
	screenW, screenH := ebiten.ScreenSizeInFullscreen()

	// 将逻辑尺寸更新为全屏尺寸，使宠物能渲染到屏幕右下角
	display.screenWidth = screenW
	display.screenHeight = screenH
	display.petSprite.SetScreenSize(screenW, screenH)

	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowPosition(0, 0)
	ebiten.SetWindowTitle("弹幕")
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)
	// 弹幕区域鼠标穿透，但宠物区域需要响应鼠标，由 PetSprite 动态控制
	ebiten.SetWindowMousePassthrough(false)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)

	return ebiten.RunGameWithOptions(display, &ebiten.RunGameOptions{
		ScreenTransparent: true,
	})
}
