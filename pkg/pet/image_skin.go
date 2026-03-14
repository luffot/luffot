package pet

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/png"
	"os"
	"path/filepath"

	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
)

// imageSkinStateName 状态名称到目录文件名的映射
var imageSkinStateName = map[PetState]string{
	StateIdle:     "idle",
	StateTyping:   "typing",
	StateThinking: "thinking",
	StateTalking:  "talking",
	StateAlert:    "alert",
}

// FullscreenEffectType 全屏特效类型
type FullscreenEffectType string

const (
	// FullscreenEffectNone 无全屏特效
	FullscreenEffectNone FullscreenEffectType = ""
	// FullscreenEffectBasketball 篮球抛物线特效（alert 状态时从桌宠位置飞向屏幕正中心）
	FullscreenEffectBasketball FullscreenEffectType = "basketball"
)

// ImageSkin 图片皮肤，包含各状态的帧序列
type ImageSkin struct {
	Name   string
	Dir    string
	frames map[PetState][]*ebiten.Image

	// frameDelays 每帧的持续 tick 数（GIF 自带延迟时间转换而来）。
	// 若某状态来自 PNG 序列，则该状态的 slice 为 nil，使用默认节奏（每 6 tick 一帧）。
	frameDelays map[PetState][]int

	// FullscreenEffect 该皮肤在 alert 状态时触发的全屏特效类型（空字符串表示无特效）
	FullscreenEffect FullscreenEffectType
}

// LoadImageSkin 从磁盘指定目录加载图片皮肤。
// 每个状态优先加载 {state}.gif，没有时再加载 {state}_0.png、{state}_1.png... 序列。
// 每个状态至少需要 1 帧，缺少某状态时回退到 idle 帧。
func LoadImageSkin(name, dir string) (*ImageSkin, error) {
	return LoadImageSkinFromFS(name, os.DirFS(dir))
}

// LoadImageSkinFromFS 从 fs.FS 加载图片皮肤（支持 embed.FS 或 os.DirFS）。
// 每个状态优先加载 {state}.gif，没有时再加载 {state}_0.png、{state}_1.png... 序列。
// 每个状态至少需要 1 帧，缺少某状态时回退到 idle 帧。
func LoadImageSkinFromFS(name string, skinFS fs.FS) (*ImageSkin, error) {
	skin := &ImageSkin{
		Name:        name,
		frames:      make(map[PetState][]*ebiten.Image),
		frameDelays: make(map[PetState][]int),
	}

	for state, stateName := range imageSkinStateName {
		frames, delays, err := loadStateFramesOrGIFFromFS(skinFS, stateName)
		if err != nil {
			return nil, fmt.Errorf("加载皮肤 %q 状态 %q 失败: %w", name, stateName, err)
		}
		if len(frames) > 0 {
			skin.frames[state] = frames
			skin.frameDelays[state] = delays // nil 表示使用默认节奏
		}
	}

	// idle 状态必须有帧
	if len(skin.frames[StateIdle]) == 0 {
		return nil, fmt.Errorf("皮肤 %q 缺少 idle 状态帧（至少需要 idle.gif 或 idle_0.png）", name)
	}

	// 缺少某状态时回退到 idle
	for state := range imageSkinStateName {
		if len(skin.frames[state]) == 0 {
			skin.frames[state] = skin.frames[StateIdle]
			skin.frameDelays[state] = skin.frameDelays[StateIdle]
		}
	}

	return skin, nil
}

// loadStateFramesOrGIF 加载某个状态的帧序列（磁盘路径版本）。
func loadStateFramesOrGIF(dir, stateName string) (frames []*ebiten.Image, delays []int, err error) {
	return loadStateFramesOrGIFFromFS(os.DirFS(dir), stateName)
}

// loadStateFramesOrGIFFromFS 从 fs.FS 加载某个状态的帧序列。
// 优先查找 {stateName}.gif，存在时解码 GIF 并返回合成帧和每帧 tick 延迟；
// 不存在时回退到 {stateName}_0.png 序列（delays 返回 nil，使用默认节奏）。
func loadStateFramesOrGIFFromFS(skinFS fs.FS, stateName string) (frames []*ebiten.Image, delays []int, err error) {
	gifName := stateName + ".gif"
	if gifFile, openErr := skinFS.Open(gifName); openErr == nil {
		gifFile.Close()
		// GIF 文件存在，优先使用
		frames, delays, err = loadGIFFramesFromFS(skinFS, gifName)
		return
	}

	// 回退到 PNG 序列
	frames, err = loadPNGFrameSequenceFromFS(skinFS, stateName)
	return // delays 为 nil
}

// loadGIFFrames 解码 GIF 文件（磁盘路径版本）。
func loadGIFFrames(filePath string) ([]*ebiten.Image, []int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 GIF 文件失败: %w", err)
	}
	defer file.Close()
	return decodeGIFFrames(file)
}

// loadGIFFramesFromFS 从 fs.FS 解码 GIF 文件，将每帧合成为完整 RGBA 图像，并返回每帧的 tick 延迟。
func loadGIFFramesFromFS(skinFS fs.FS, gifName string) ([]*ebiten.Image, []int, error) {
	file, err := skinFS.Open(gifName)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 GIF 文件失败: %w", err)
	}
	defer file.Close()
	return decodeGIFFrames(file)
}

// decodeGIFFrames 解码 GIF 数据流，将每帧合成为完整 RGBA 图像，并返回每帧的 tick 延迟。
// GIF 的 Delay 单位是 1/100s，以 60fps（1 tick ≈ 16.7ms）换算为 tick 数，最小 1 tick。
func decodeGIFFrames(reader fs.File) ([]*ebiten.Image, []int, error) {
	animatedGIF, err := gif.DecodeAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("解码 GIF 失败: %w", err)
	}
	if len(animatedGIF.Image) == 0 {
		return nil, nil, fmt.Errorf("GIF 文件不包含任何帧")
	}

	// 以第一帧尺寸为画布尺寸（GIF 规范中 LogicalScreenWidth/Height 是全局画布）
	canvasWidth := animatedGIF.Config.Width
	canvasHeight := animatedGIF.Config.Height
	if canvasWidth == 0 || canvasHeight == 0 {
		// 回退到第一帧尺寸
		bounds := animatedGIF.Image[0].Bounds()
		canvasWidth = bounds.Max.X
		canvasHeight = bounds.Max.Y
	}

	canvas := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	// prevCanvas 保存上一帧绘制前的画布快照，用于 DisposalPrevious 恢复
	prevCanvas := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	frames := make([]*ebiten.Image, 0, len(animatedGIF.Image))
	delays := make([]int, 0, len(animatedGIF.Image))

	for frameIndex, paletteImage := range animatedGIF.Image {
		disposal := byte(gif.DisposalNone)
		if frameIndex < len(animatedGIF.Disposal) {
			disposal = animatedGIF.Disposal[frameIndex]
		}

		// 在绘制当前帧之前，保存画布快照（供 DisposalPrevious 使用）
		draw.Draw(prevCanvas, prevCanvas.Bounds(), canvas, image.Point{}, draw.Src)

		// 将当前 GIF 帧（调色板图）绘制到画布上
		frameBounds := paletteImage.Bounds()
		draw.Draw(canvas, frameBounds, paletteImage, frameBounds.Min, draw.Over)

		// 复制当前画布为完整帧
		frameCopy := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
		draw.Draw(frameCopy, frameCopy.Bounds(), canvas, image.Point{}, draw.Src)
		frames = append(frames, ebiten.NewImageFromImage(frameCopy))

		// 根据 Disposal 方法处理画布状态，为下一帧做准备
		switch disposal {
		case gif.DisposalBackground:
			// 将当前帧区域恢复为背景色（透明）
			draw.Draw(canvas, frameBounds, image.NewUniform(color.RGBA{}), image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			// 恢复到绘制当前帧之前的画布状态
			draw.Draw(canvas, canvas.Bounds(), prevCanvas, image.Point{}, draw.Src)
		}
		// gif.DisposalNone / 未指定：保留当前画布，下一帧叠加绘制

		// 将 GIF 延迟（1/100s）转换为 tick 数（60fps，1 tick ≈ 16.7ms）
		// delay * 10ms / 16.7ms ≈ delay * 0.6，最小保证 1 tick
		delayHundredths := 10 // 默认 100ms（10/100s）
		if frameIndex < len(animatedGIF.Delay) && animatedGIF.Delay[frameIndex] > 0 {
			delayHundredths = animatedGIF.Delay[frameIndex]
		}
		tickDelay := int(float64(delayHundredths) * 0.6)
		if tickDelay < 1 {
			tickDelay = 1
		}
		delays = append(delays, tickDelay)
	}

	return frames, delays, nil
}

// loadPNGFrameSequence 加载 PNG 帧序列（磁盘路径版本）。
func loadPNGFrameSequence(dir, stateName string) ([]*ebiten.Image, error) {
	return loadPNGFrameSequenceFromFS(os.DirFS(dir), stateName)
}

// loadPNGFrameSequenceFromFS 从 fs.FS 加载 PNG 帧序列，帧编号从 0 开始连续读取直到文件不存在为止。
func loadPNGFrameSequenceFromFS(skinFS fs.FS, stateName string) ([]*ebiten.Image, error) {
	var frames []*ebiten.Image
	for frameIndex := 0; ; frameIndex++ {
		fileName := fmt.Sprintf("%s_%d.png", stateName, frameIndex)
		img, err := loadPNGFromFS(skinFS, fileName)
		if err != nil {
			// 文件不存在时停止，其他错误直接返回
			break
		}
		frames = append(frames, img)
	}
	return frames, nil
}

// loadPNG 从磁盘文件路径加载 PNG 并转换为 ebiten.Image。
func loadPNG(filePath string) (*ebiten.Image, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	return ebiten.NewImageFromImage(img), nil
}

// loadPNGFromFS 从 fs.FS 加载 PNG 并转换为 ebiten.Image。
func loadPNGFromFS(skinFS fs.FS, fileName string) (*ebiten.Image, error) {
	file, err := skinFS.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("解码图片 %q 失败: %w", fileName, err)
	}

	return ebiten.NewImageFromImage(img), nil
}

// GetFrame 获取指定状态在当前 tick 下应显示的帧（循环播放）。
// 若该状态有 GIF 帧延迟信息，则按各帧自身的延迟时间推进；
// 否则使用默认节奏（每 6 tick 切换一帧，约 10fps）。
func (skin *ImageSkin) GetFrame(state PetState, tick int) *ebiten.Image {
	frames := skin.frames[state]
	if len(frames) == 0 {
		return nil
	}

	delays := skin.frameDelays[state]
	if len(delays) != len(frames) {
		// PNG 序列或延迟数据缺失：使用默认节奏（每 6 tick 一帧）
		return frames[(tick/6)%len(frames)]
	}

	// 计算所有帧的总 tick 周期，然后在周期内定位当前帧
	totalTicks := 0
	for _, delay := range delays {
		totalTicks += delay
	}
	if totalTicks <= 0 {
		return frames[0]
	}

	// 在总周期内取模，找到当前 tick 对应的帧
	positionInCycle := tick % totalTicks
	accumulated := 0
	for frameIndex, delay := range delays {
		accumulated += delay
		if positionInCycle < accumulated {
			return frames[frameIndex]
		}
	}

	// 理论上不会到达这里，保险起见返回最后一帧
	return frames[len(frames)-1]
}

// ============================================================
// 全局图片皮肤注册表
// ============================================================

// imageSkins 已注册的图片皮肤（name -> *ImageSkin）
var imageSkins = map[string]*ImageSkin{}

// imageSkinNames 图片皮肤名称有序列表
var imageSkinNames []string

// activeImageSkin 当前激活的图片皮肤（nil 表示使用矢量皮肤）
var activeImageSkin *ImageSkin

// RegisterImageSkin 注册一个图片皮肤（从目录加载），注册失败返回错误
func RegisterImageSkin(name, dir string) error {
	return RegisterImageSkinWithEffect(name, dir, FullscreenEffectNone)
}

// RegisterImageSkinWithEffect 注册一个带全屏特效的图片皮肤
func RegisterImageSkinWithEffect(name, dir string, effect FullscreenEffectType) error {
	skin, err := LoadImageSkin(name, dir)
	if err != nil {
		return err
	}
	skin.FullscreenEffect = effect
	if _, exists := imageSkins[name]; !exists {
		imageSkinNames = append(imageSkinNames, name)
	}
	imageSkins[name] = skin
	return nil
}

// SetActiveImageSkin 激活指定图片皮肤，返回是否成功
func SetActiveImageSkin(name string) bool {
	skin, ok := imageSkins[name]
	if !ok {
		return false
	}
	activeImageSkin = skin
	return true
}

// ClearActiveImageSkin 清除图片皮肤，恢复矢量绘制模式
func ClearActiveImageSkin() {
	activeImageSkin = nil
}

// GetActiveImageSkin 获取当前激活的图片皮肤（nil 表示矢量模式）
func GetActiveImageSkin() *ImageSkin {
	return activeImageSkin
}

// GetImageSkinNames 获取所有已注册的图片皮肤名称
func GetImageSkinNames() []string {
	return imageSkinNames
}

// IsImageSkinActive 返回当前是否处于图片皮肤模式
func IsImageSkinActive() bool {
	return activeImageSkin != nil
}

// SkinBasePath 图片皮肤根目录（相对于可执行文件）
const SkinBasePath = "pkg/embedfs/static/skins"

// skinFullscreenEffects 皮肤名称到全屏特效类型的映射（硬编码兜底，优先读取 skin_meta.json）。
var skinFullscreenEffects = map[string]FullscreenEffectType{
	"蔡徐坤": FullscreenEffectBasketball,
}

// skinMeta skin_meta.json 的结构（与 skin-builder 生成的格式一致）
type skinMeta struct {
	Name             string               `json:"name"`
	FullscreenEffect FullscreenEffectType `json:"fullscreen_effect"`
}

// loadSkinMeta 尝试从皮肤目录读取 skin_meta.json，返回元信息。
// 文件不存在或解析失败时静默返回 nil。
func loadSkinMeta(skinDir string) *skinMeta {
	metaPath := filepath.Join(skinDir, "skin_meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil
	}
	var meta skinMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

// loadSkinMetaFromFS 尝试从 fs.FS 子文件系统读取 skin_meta.json，返回元信息。
// 文件不存在或解析失败时静默返回 nil。
func loadSkinMetaFromFS(skinFS fs.FS) *skinMeta {
	data, err := fs.ReadFile(skinFS, "skin_meta.json")
	if err != nil {
		return nil
	}
	var meta skinMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

// AutoLoadImageSkins 自动扫描磁盘 assets/skins 目录，加载所有子目录作为图片皮肤。
// 优先读取皮肤目录内的 skin_meta.json 获取全屏特效配置；
// 若无 skin_meta.json，则回退到 skinFullscreenEffects 硬编码映射。
func AutoLoadImageSkins() {
	entries, err := os.ReadDir(SkinBasePath)
	if err != nil {
		// 目录不存在时静默跳过
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skinName := entry.Name()
		skinDir := filepath.Join(SkinBasePath, skinName)

		// 优先从 skin_meta.json 读取全屏特效配置
		var effect FullscreenEffectType
		if meta := loadSkinMeta(skinDir); meta != nil {
			effect = meta.FullscreenEffect
			// skin_meta.json 中记录了皮肤的显示名称，优先使用
			if meta.Name != "" {
				skinName = meta.Name
			}
		} else {
			// 回退到硬编码映射
			effect = skinFullscreenEffects[skinName]
		}

		if err := RegisterImageSkinWithEffect(skinName, skinDir, effect); err != nil {
			// 加载失败时跳过（可能是目录结构不完整）
			continue
		}
	}
}

// AutoLoadImageSkinsFromFS 从 fs.FS 自动扫描并加载所有图片皮肤（用于 embed 模式）。
// skinsFS 的根目录即为 assets/skins 目录（每个子目录对应一个皮肤）。
// 优先读取皮肤目录内的 skin_meta.json 获取全屏特效配置；
// 若无 skin_meta.json，则回退到 skinFullscreenEffects 硬编码映射。
func AutoLoadImageSkinsFromFS(skinsFS fs.FS) {
	entries, err := fs.ReadDir(skinsFS, ".")
	if err != nil {
		// 目录不存在时静默跳过
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skinName := entry.Name()

		// 获取该皮肤目录的子 FS
		skinSubFS, err := fs.Sub(skinsFS, skinName)
		if err != nil {
			continue
		}

		// 优先从 skin_meta.json 读取全屏特效配置
		var effect FullscreenEffectType
		if meta := loadSkinMetaFromFS(skinSubFS); meta != nil {
			effect = meta.FullscreenEffect
			// skin_meta.json 中记录了皮肤的显示名称，优先使用
			if meta.Name != "" {
				skinName = meta.Name
			}
		} else {
			// 回退到硬编码映射
			effect = skinFullscreenEffects[skinName]
		}

		skin, err := LoadImageSkinFromFS(skinName, skinSubFS)
		if err != nil {
			// 加载失败时跳过（可能是目录结构不完整）
			continue
		}
		skin.FullscreenEffect = effect

		if _, exists := imageSkins[skinName]; !exists {
			imageSkinNames = append(imageSkinNames, skinName)
		}
		imageSkins[skinName] = skin
	}
}
