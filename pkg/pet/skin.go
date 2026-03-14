package pet

import "image/color"

// SkinConfig 桌宠皮肤配置，包含所有可定制颜色
type SkinConfig struct {
	Name string

	// 身体
	Body     color.RGBA
	BodySide color.RGBA
	BodyDark color.RGBA
	Beak     color.RGBA
	EyeWhite color.RGBA
	EyePupil color.RGBA
	EyeShine color.RGBA

	// 头顶羽毛
	Tuft    color.RGBA
	TuftTip color.RGBA

	// 闪电
	Lightning     color.RGBA
	LightningGlow color.RGBA

	// 翅膀
	WingMain  color.RGBA
	WingLight color.RGBA
	WingEdge  color.RGBA
	WingTip   color.RGBA

	// 尾巴
	Tail    color.RGBA
	TailTip color.RGBA
}

// SkinNames 所有可用皮肤的名称列表（有序）
var SkinNames = []string{
	"钉三多（经典黑）",
	"星空蓝",
	"暗金",
	"樱花粉",
}

// LuaSkinNames Lua 皮肤名称列表（动态加载）
var LuaSkinNames []string

// Skins 预置皮肤库
var Skins = map[string]SkinConfig{
	"钉三多（经典黑）": {
		Name:          "钉三多（经典黑）",
		Body:          color.RGBA{20, 20, 25, 255},
		BodySide:      color.RGBA{35, 35, 42, 255},
		BodyDark:      color.RGBA{12, 12, 15, 255},
		Beak:          color.RGBA{30, 144, 255, 255},
		EyeWhite:      color.RGBA{255, 255, 255, 255},
		EyePupil:      color.RGBA{15, 15, 20, 255},
		EyeShine:      color.RGBA{255, 255, 255, 220},
		Tuft:          color.RGBA{25, 25, 30, 255},
		TuftTip:       color.RGBA{50, 50, 60, 255},
		Lightning:     color.RGBA{255, 255, 255, 255},
		LightningGlow: color.RGBA{255, 255, 255, 60},
		WingMain:      color.RGBA{22, 22, 28, 255},
		WingLight:     color.RGBA{45, 45, 55, 200},
		WingEdge:      color.RGBA{40, 40, 50, 255},
		WingTip:       color.RGBA{40, 40, 50, 255},
		Tail:          color.RGBA{20, 20, 25, 255},
		TailTip:       color.RGBA{45, 45, 55, 255},
	},
	"星空蓝": {
		Name:          "星空蓝",
		Body:          color.RGBA{10, 30, 80, 255},
		BodySide:      color.RGBA{20, 50, 120, 255},
		BodyDark:      color.RGBA{5, 15, 50, 255},
		Beak:          color.RGBA{100, 220, 255, 255},
		EyeWhite:      color.RGBA{200, 240, 255, 255},
		EyePupil:      color.RGBA{10, 20, 60, 255},
		EyeShine:      color.RGBA{255, 255, 255, 220},
		Tuft:          color.RGBA{30, 80, 180, 255},
		TuftTip:       color.RGBA{80, 160, 255, 255},
		Lightning:     color.RGBA{150, 220, 255, 255},
		LightningGlow: color.RGBA{100, 180, 255, 80},
		WingMain:      color.RGBA{15, 50, 130, 255},
		WingLight:     color.RGBA{40, 100, 200, 200},
		WingEdge:      color.RGBA{60, 140, 230, 255},
		WingTip:       color.RGBA{80, 160, 255, 255},
		Tail:          color.RGBA{10, 40, 110, 255},
		TailTip:       color.RGBA{50, 120, 220, 255},
	},
	"暗金": {
		Name:          "暗金",
		Body:          color.RGBA{60, 40, 5, 255},
		BodySide:      color.RGBA{90, 65, 10, 255},
		BodyDark:      color.RGBA{35, 22, 2, 255},
		Beak:          color.RGBA{255, 200, 50, 255},
		EyeWhite:      color.RGBA{255, 240, 200, 255},
		EyePupil:      color.RGBA{40, 20, 5, 255},
		EyeShine:      color.RGBA{255, 255, 220, 220},
		Tuft:          color.RGBA{80, 55, 8, 255},
		TuftTip:       color.RGBA{200, 160, 40, 255},
		Lightning:     color.RGBA{255, 220, 80, 255},
		LightningGlow: color.RGBA{255, 180, 0, 80},
		WingMain:      color.RGBA{70, 48, 8, 255},
		WingLight:     color.RGBA{140, 100, 20, 200},
		WingEdge:      color.RGBA{180, 140, 30, 255},
		WingTip:       color.RGBA{200, 160, 40, 255},
		Tail:          color.RGBA{55, 38, 5, 255},
		TailTip:       color.RGBA{160, 120, 25, 255},
	},
	"樱花粉": {
		Name:          "樱花粉",
		Body:          color.RGBA{80, 20, 40, 255},
		BodySide:      color.RGBA{120, 40, 65, 255},
		BodyDark:      color.RGBA{50, 10, 25, 255},
		Beak:          color.RGBA{255, 150, 180, 255},
		EyeWhite:      color.RGBA{255, 230, 235, 255},
		EyePupil:      color.RGBA{50, 10, 25, 255},
		EyeShine:      color.RGBA{255, 255, 255, 220},
		Tuft:          color.RGBA{100, 30, 55, 255},
		TuftTip:       color.RGBA{230, 120, 160, 255},
		Lightning:     color.RGBA{255, 200, 220, 255},
		LightningGlow: color.RGBA{255, 150, 180, 80},
		WingMain:      color.RGBA{90, 25, 50, 255},
		WingLight:     color.RGBA{180, 80, 120, 200},
		WingEdge:      color.RGBA{210, 100, 140, 255},
		WingTip:       color.RGBA{230, 120, 160, 255},
		Tail:          color.RGBA{70, 18, 38, 255},
		TailTip:       color.RGBA{190, 90, 130, 255},
	},
}

// DefaultSkinName 默认皮肤名称
const DefaultSkinName = "钉三多（经典黑）"

// SetSkinByName 统一皮肤切换入口：优先匹配 Lua 皮肤，其次匹配图片皮肤，最后匹配矢量皮肤。
// 切换到某种皮肤时会清除其他皮肤类型的激活状态。
// 返回是否切换成功。
func SetSkinByName(name string) bool {
	// 优先尝试 Lua 皮肤
	if SetActiveLuaSkin(name) {
		ClearActiveImageSkin()
		ClearActiveSkin()
		return true
	}
	// 其次尝试图片皮肤
	if SetActiveImageSkin(name) {
		ClearActiveLuaSkin()
		ClearActiveSkin()
		return true
	}
	// 回退到矢量皮肤
	if SetActiveSkin(name) {
		ClearActiveLuaSkin()
		ClearActiveImageSkin()
		return true
	}
	return false
}

// GetCurrentSkinName 获取当前激活皮肤的名称（Lua 皮肤、图片皮肤或矢量皮肤）
func GetCurrentSkinName() string {
	// 优先检查 Lua 皮肤
	if name := GetActiveLuaSkinName(); name != "" {
		return name
	}
	// 其次检查图片皮肤
	if skin := GetActiveImageSkin(); skin != nil {
		return skin.Name
	}
	// 最后返回矢量皮肤
	return GetActiveSkinName()
}

// AllSkinNames 返回所有可用皮肤名称（矢量皮肤 + 图片皮肤 + Lua 皮肤，有序）
func AllSkinNames() []string {
	// 初始化 Lua 皮肤系统（如果尚未初始化）
	InitLuaSkinSystem()

	luaNames := GetLuaSkinNames()
	result := make([]string, 0, len(SkinNames)+len(GetImageSkinNames())+len(luaNames))
	result = append(result, SkinNames...)
	result = append(result, GetImageSkinNames()...)
	result = append(result, luaNames...)
	return result
}

// activeSkin 当前激活的皮肤（全局，供 sprite.go 绘制函数使用）
var activeSkin = Skins[DefaultSkinName]

// SetActiveSkin 设置当前皮肤（线程安全由调用方保证，在 ebiten 主循环外调用即可）
func SetActiveSkin(name string) bool {
	skin, ok := Skins[name]
	if !ok {
		return false
	}
	activeSkin = skin
	// 同步更新 sprite.go 中的全局颜色变量
	applyActiveSkin()
	return true
}

// GetActiveSkinName 获取当前皮肤名称
func GetActiveSkinName() string {
	return activeSkin.Name
}

// ClearActiveSkin 清除矢量皮肤激活状态，恢复默认皮肤
func ClearActiveSkin() {
	activeSkin = Skins[DefaultSkinName]
	applyActiveSkin()
}

// applyActiveSkin 将 activeSkin 的颜色同步到 sprite.go 的全局颜色变量
// 变量名必须与 sprite.go 中声明的 var 块完全一致
func applyActiveSkin() {
	s := activeSkin
	colorFeather = s.Body
	colorFeatherSide = s.BodySide
	colorFeatherDark = s.BodyDark
	colorBeak = s.Beak
	colorEyeWhite = s.EyeWhite
	colorEyeRing = s.EyePupil // sprite.go 中眼圈色即瞳孔色
	colorTuft = s.Tuft
	colorTuftTip = s.TuftTip
	colorLightning = s.Lightning
	colorLightningGlow = s.LightningGlow
	colorWingMain = s.WingMain
	colorWingLight = s.WingLight
	colorWingEdge = s.WingEdge
	colorWingTip = s.WingTip
	colorTail = s.Tail
	colorTailTip = s.TailTip
}
