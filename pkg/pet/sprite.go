package pet

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// ============================================================
// 钉三多（DingTalk 吉祥物）颜色常量
// 形象：全身黑色圆润小鸟，大白眼睛（左圆右眯），蓝色小嘴，胸口白色闪电
// ============================================================
var (
	// 身体：钉三多全身纯黑色
	colorFeather     = color.RGBA{20, 20, 25, 255}    // 主体黑色
	colorFeatherSide = color.RGBA{35, 35, 42, 255}    // 侧面略亮黑（体积感）
	colorFeatherDark = color.RGBA{12, 12, 15, 255}    // 深黑（阴影）
	colorBeak        = color.RGBA{30, 144, 255, 255}  // 嘴巴：钉钉蓝
	colorEyeWhite    = color.RGBA{255, 255, 255, 255} // 眼白
	colorEyePupil    = color.RGBA{15, 15, 20, 255}    // 瞳孔黑
	colorEyeShine    = color.RGBA{255, 255, 255, 220} // 眼睛高光
	colorEyeRing     = color.RGBA{20, 20, 25, 255}    // 眼圈（与身体同色，融合）

	// 头顶羽毛：黑色
	colorTuft    = color.RGBA{25, 25, 30, 255} // 头顶羽毛黑色
	colorTuftTip = color.RGBA{50, 50, 60, 255} // 羽毛尖端略亮

	// 胸口闪电：白色（参考图片）
	colorLightning     = color.RGBA{255, 255, 255, 255} // 闪电白色
	colorLightningGlow = color.RGBA{255, 255, 255, 60}  // 闪电光晕白

	// 翅膀：黑色系
	colorWingMain  = color.RGBA{22, 22, 28, 255} // 翅膀主色（黑）
	colorWingLight = color.RGBA{45, 45, 55, 200} // 翅膀亮色（深灰，体积感）
	colorWingEdge  = color.RGBA{40, 40, 50, 255} // 翅膀边缘

	// 尾巴：黑色系
	colorTail    = color.RGBA{20, 20, 25, 255} // 尾羽黑色
	colorTailTip = color.RGBA{45, 45, 55, 255} // 尾羽尖端略亮

	// 翅膀末端
	colorWingTip = color.RGBA{40, 40, 50, 255}

	// 鱼
	colorFish      = color.RGBA{255, 140, 0, 255}
	colorFishBelly = color.RGBA{255, 210, 120, 255}
	colorFishEye   = color.RGBA{20, 20, 20, 255}
	colorFishTail  = color.RGBA{220, 100, 0, 255}

	// 键盘
	colorKeyboard   = color.RGBA{30, 40, 70, 255}
	colorKeyboardHL = color.RGBA{60, 120, 220, 255}
	colorKeyTop     = color.RGBA{50, 80, 150, 255}

	// 科幻光晕
	colorGlow = color.RGBA{80, 160, 255, 60}
)

const (
	spriteWidth  = 160
	spriteHeight = 160
)

// DrawIdleFrame 绘制 idle（摸鱼）动画帧
func DrawIdleFrame(dst *ebiten.Image, frame int, tick int) {
	dst.Fill(color.RGBA{0, 0, 0, 0})

	wingAngle := math.Sin(float64(tick)*0.12) * 15.0
	bodyBob := math.Sin(float64(tick)*0.07) * 3.0
	fishSwing := math.Sin(float64(tick)*0.10) * 10.0
	tuftSwing := math.Sin(float64(tick)*0.09) * 4.0

	cx := float64(spriteWidth) / 2
	cy := float64(spriteHeight)/2 + bodyBob

	drawGlow(dst, cx, cy)
	drawTail(dst, cx, cy, 0)
	drawWingsDing(dst, cx, cy, wingAngle)
	drawBodyDing(dst, cx, cy)
	drawTuft(dst, cx, cy, tuftSwing)
	drawEyesDing(dst, cx, cy)
	drawBeakDing(dst, cx, cy)
	drawLightning(dst, cx, cy)
	drawIdleWingsWithFish(dst, cx, cy, fishSwing)
}

// DrawTypingFrame 绘制 typing（敲键盘）动画帧
func DrawTypingFrame(dst *ebiten.Image, tick int) {
	dst.Fill(color.RGBA{0, 0, 0, 0})

	bodyShake := math.Sin(float64(tick)*0.6) * 2.0
	wingAngle := math.Sin(float64(tick)*0.5) * 22.0
	tuftSwing := math.Sin(float64(tick)*0.5) * 8.0

	cx := float64(spriteWidth) / 2
	cy := float64(spriteHeight)/2 + bodyShake

	drawKeyboard(dst, cx, cy+42)
	drawGlow(dst, cx, cy)
	drawTail(dst, cx, cy, bodyShake*0.3)
	drawWingsDing(dst, cx, cy, wingAngle)
	drawBodyDing(dst, cx, cy)
	drawTuft(dst, cx, cy, tuftSwing)
	drawEyesDing(dst, cx, cy)
	drawBeakDing(dst, cx, cy)
	drawLightning(dst, cx, cy)
	drawTypingWings(dst, cx, cy, tick)
}

// drawGlow 科幻光晕
func drawGlow(dst *ebiten.Image, cx, cy float64) {
	vector.DrawFilledCircle(dst, float32(cx), float32(cy), 44, colorGlow, true)
}

// drawBodyDing 绘制钉三多身体（大圆黑色身体，参考图片）
func drawBodyDing(dst *ebiten.Image, cx, cy float64) {
	// 身体主体：大圆形黑色
	vector.DrawFilledCircle(dst, float32(cx), float32(cy)+2, 26, colorFeather, true)

	// 头部：圆形，与身体融合，整体呈圆润大鸟形
	vector.DrawFilledCircle(dst, float32(cx), float32(cy)-18, 20, colorFeather, true)

	// 头部与身体之间的连接（填充过渡区域）
	vector.DrawFilledRect(dst, float32(cx)-18, float32(cy)-18, 36, 22, colorFeather, true)

	// 顶部轻微高光（体积感，极淡）
	vector.DrawFilledCircle(dst, float32(cx)-6, float32(cy)-26, 7, colorFeatherSide, true)
}

// drawTuft 绘制头顶呆毛（钉钉 Logo 标志性元素）
// 呆毛是一撮向上翘起的蓝色羽毛，末端有亮点
func drawTuft(dst *ebiten.Image, cx, cy, swing float64) {
	baseX := cx
	baseY := cy - 34

	// 主呆毛（中间，最高）
	tipX := baseX + swing
	tipY := baseY - 18
	vector.StrokeLine(dst, float32(baseX), float32(baseY), float32(tipX), float32(tipY), 3, colorTuft, true)
	vector.DrawFilledCircle(dst, float32(tipX), float32(tipY), 4, colorTuft, true)
	vector.DrawFilledCircle(dst, float32(tipX), float32(tipY), 2, colorTuftTip, true)

	// 左侧小呆毛
	ltipX := baseX - 5 + swing*0.6
	ltipY := baseY - 12
	vector.StrokeLine(dst, float32(baseX-3), float32(baseY), float32(ltipX), float32(ltipY), 2, colorTuft, true)
	vector.DrawFilledCircle(dst, float32(ltipX), float32(ltipY), 2.5, colorTuft, true)

	// 右侧小呆毛
	rtipX := baseX + 5 + swing*0.6
	rtipY := baseY - 10
	vector.StrokeLine(dst, float32(baseX+3), float32(baseY), float32(rtipX), float32(rtipY), 2, colorTuft, true)
	vector.DrawFilledCircle(dst, float32(rtipX), float32(rtipY), 2.5, colorTuft, true)
}

// drawEyesDing 绘制钉三多大眼睛
// 左眼：大圆白眼（有黑瞳），右眼：眯成弧线的笑眼
func drawEyesDing(dst *ebiten.Image, cx, cy float64) {
	eyeY := cy - 18

	// 左眼：大圆白眼
	vector.DrawFilledCircle(dst, float32(cx)-8, float32(eyeY), 9, colorEyeWhite, true)
	vector.DrawFilledCircle(dst, float32(cx)-7, float32(eyeY)+1, 5, colorEyePupil, true)
	vector.DrawFilledCircle(dst, float32(cx)-5.5, float32(eyeY)-1, 2, colorEyeShine, true)

	// 右眼：眯眼（弧线笑眼），用一段弧形白色区域表现
	// 先画白色半圆底部（眯眼的白色部分）
	for dy := 0; dy <= 5; dy++ {
		for dx := -7; dx <= 7; dx++ {
			nx := float64(dx) / 7.0
			ny := float64(dy) / 5.0
			if nx*nx+ny*ny <= 1.0 {
				dst.Set(int(cx)+14+dx, int(eyeY)+dy, colorEyeWhite)
			}
		}
	}
	// 眯眼上方遮罩（黑色，让眼睛看起来眯起来）
	vector.DrawFilledCircle(dst, float32(cx)+14, float32(eyeY)-3, 7, colorFeather, true)
	// 眯眼弧线（白色弧形）
	vector.StrokeLine(dst, float32(cx)+7, float32(eyeY)+2, float32(cx)+14, float32(eyeY)+5, 2.5, colorEyeWhite, true)
	vector.StrokeLine(dst, float32(cx)+14, float32(eyeY)+5, float32(cx)+21, float32(eyeY)+2, 2.5, colorEyeWhite, true)
}

// drawBeakDing 绘制小嘴（蓝色三角嘴，位于两眼之间偏下）
func drawBeakDing(dst *ebiten.Image, cx, cy float64) {
	// 嘴巴在两眼中间偏下，对应 eyeY = cy-18，嘴在 cy-10 左右
	beakY := cy - 10
	drawTriangle(dst,
		float32(cx), float32(beakY+6), // 嘴尖（向下）
		float32(cx-5), float32(beakY), // 左上
		float32(cx+5), float32(beakY), // 右上
		colorBeak,
	)
}

// drawLightning 绘制胸口闪电（钉三多标志性元素，代表快捷速度）
func drawLightning(dst *ebiten.Image, cx, cy float64) {
	// 闪电光晕
	vector.DrawFilledCircle(dst, float32(cx), float32(cy)+4, 10, colorLightningGlow, true)

	// 闪电形状（Z 字形）
	lx, ly := float32(cx), float32(cy)
	// 上段：右上到中
	drawTriangle(dst,
		lx+4, ly-4,
		lx-2, ly+4,
		lx+2, ly+4,
		colorLightning,
	)
	// 下段：中到左下
	drawTriangle(dst,
		lx-2, ly+4,
		lx+2, ly+4,
		lx-4, ly+12,
		colorLightning,
	)
	drawTriangle(dst,
		lx+2, ly+4,
		lx-4, ly+12,
		lx+0, ly+12,
		colorLightning,
	)
}

// drawTail 绘制剪刀形分叉尾（雨燕特征）
func drawTail(dst *ebiten.Image, cx, cy, tilt float64) {
	tailBaseX := cx
	tailBaseY := cy + 22

	// 左尾羽
	lTipX := tailBaseX - 14 + tilt
	lTipY := tailBaseY + 22
	drawTriangle(dst,
		float32(tailBaseX-2), float32(tailBaseY),
		float32(tailBaseX+4), float32(tailBaseY+4),
		float32(lTipX), float32(lTipY),
		colorTail,
	)
	// 左尾羽亮边
	vector.StrokeLine(dst, float32(tailBaseX), float32(tailBaseY), float32(lTipX), float32(lTipY), 1.5, colorTailTip, true)

	// 右尾羽
	rTipX := tailBaseX + 14 + tilt
	rTipY := tailBaseY + 22
	drawTriangle(dst,
		float32(tailBaseX+2), float32(tailBaseY),
		float32(tailBaseX-4), float32(tailBaseY+4),
		float32(rTipX), float32(rTipY),
		colorTail,
	)
	// 右尾羽亮边
	vector.StrokeLine(dst, float32(tailBaseX), float32(tailBaseY), float32(rTipX), float32(rTipY), 1.5, colorTailTip, true)

	// 中间填充（两尾羽之间）
	drawTriangle(dst,
		float32(tailBaseX-4), float32(tailBaseY),
		float32(tailBaseX+4), float32(tailBaseY),
		float32(tailBaseX), float32(tailBaseY+10),
		colorFeatherDark,
	)
}

// drawWingsDing 绘制钉三多翅膀（雨燕翅膀，贴身，深蓝色）
func drawWingsDing(dst *ebiten.Image, cx, cy, angle float64) {
	rad := angle * math.Pi / 180.0

	// 左翅膀（向左后方展开）
	drawSwiftWing(dst, cx-10, cy-2, -0.5-rad, true)
	// 右翅膀（向右后方展开）
	drawSwiftWing(dst, cx+10, cy-2, 0.5+rad, false)
}

// drawSwiftWing 绘制单侧雨燕翅膀（尖长形）
func drawSwiftWing(dst *ebiten.Image, x, y, angle float64, isLeft bool) {
	tipDist := 28.0
	tipX := x + tipDist*math.Cos(angle)
	tipY := y + tipDist*math.Sin(angle)

	midX := x + tipDist*0.5*math.Cos(angle)
	midY := y + tipDist*0.5*math.Sin(angle)

	perpAngle := angle + math.Pi/2
	spread := 10.0

	// 翅膀主体（三角形）
	drawTriangle(dst,
		float32(x), float32(y),
		float32(midX+spread*math.Cos(perpAngle)), float32(midY+spread*math.Sin(perpAngle)),
		float32(tipX), float32(tipY),
		colorWingMain,
	)
	drawTriangle(dst,
		float32(x), float32(y),
		float32(midX-spread*0.4*math.Cos(perpAngle)), float32(midY-spread*0.4*math.Sin(perpAngle)),
		float32(tipX), float32(tipY),
		colorWingMain,
	)

	// 翅膀亮色覆盖（上层）
	drawTriangle(dst,
		float32(x), float32(y),
		float32(midX+spread*0.5*math.Cos(perpAngle)), float32(midY+spread*0.5*math.Sin(perpAngle)),
		float32(tipX), float32(tipY),
		colorWingLight,
	)

	// 翅膀边缘线
	vector.StrokeLine(dst, float32(x), float32(y), float32(tipX), float32(tipY), 1.5, colorWingEdge, true)

	// 翅膀尖端
	vector.DrawFilledCircle(dst, float32(tipX), float32(tipY), 2.5, colorWingTip, true)
}

// drawIdleWingsWithFish 绘制 idle 状态：翅膀末端捧鱼
func drawIdleWingsWithFish(dst *ebiten.Image, cx, cy, fishSwing float64) {
	// 左翅膀末端（捧鱼的"手"）
	lWingTipX := cx - 30
	lWingTipY := cy + 10
	vector.DrawFilledCircle(dst, float32(lWingTipX), float32(lWingTipY), 4, colorWingTip, true)

	// 右翅膀末端
	rWingTipX := cx + 30
	rWingTipY := cy + 10
	vector.DrawFilledCircle(dst, float32(rWingTipX), float32(rWingTipY), 4, colorWingTip, true)

	// 鱼（在两翅之间，随 fishSwing 摇摆）
	fishX := cx + fishSwing*0.4
	fishY := cy + 32
	drawFish(dst, fishX, fishY, fishSwing)
}

// drawFish 绘制一条大鱼（抱在怀里的尺寸）
func drawFish(dst *ebiten.Image, cx, cy, angle float64) {
	rad := angle * math.Pi / 180.0 * 0.25

	// 鱼身（椭圆），长轴 22、短轴 11，比原来大约 1.8 倍
	const bodyW = 22.0
	const bodyH = 11.0
	for dy := -bodyH; dy <= bodyH; dy++ {
		for dx := -bodyW; dx <= bodyW; dx++ {
			nx := dx / bodyW
			ny := dy / bodyH
			if nx*nx+ny*ny <= 1.0 {
				rotX := dx*math.Cos(rad) - dy*math.Sin(rad)
				rotY := dx*math.Sin(rad) + dy*math.Cos(rad)
				shade := 1.0 - ny*0.35
				r := uint8(float64(colorFish.R)*shade + float64(colorFishBelly.R)*(1-shade)*0.3)
				g := uint8(float64(colorFish.G)*shade + float64(colorFishBelly.G)*(1-shade)*0.3)
				b := uint8(float64(colorFish.B) * shade)
				dst.Set(int(cx+rotX), int(cy+rotY), color.RGBA{r, g, b, 255})
			}
		}
	}

	// 鱼尾（更大的扇形尾巴）
	tailX := cx - bodyW*math.Cos(rad)
	tailY := cy - bodyW*math.Sin(rad)
	drawTriangle(dst,
		float32(tailX), float32(tailY),
		float32(tailX-14*math.Cos(rad)+11*math.Sin(rad)), float32(tailY-14*math.Sin(rad)-11*math.Cos(rad)),
		float32(tailX-14*math.Cos(rad)-11*math.Sin(rad)), float32(tailY-14*math.Sin(rad)+11*math.Cos(rad)),
		colorFishTail,
	)

	// 鱼眼（更大）
	eyeX := cx + 14*math.Cos(rad)
	eyeY := cy + 14*math.Sin(rad) - 3
	vector.DrawFilledCircle(dst, float32(eyeX), float32(eyeY), 4.5, colorEyeWhite, true)
	vector.DrawFilledCircle(dst, float32(eyeX)+0.8, float32(eyeY)+0.8, 2.2, colorFishEye, true)
	// 眼睛高光
	vector.DrawFilledCircle(dst, float32(eyeX)-1, float32(eyeY)-1.5, 1.0, colorEyeShine, true)

	// 背鳍（更大）
	finX := cx + 4*math.Cos(rad)
	finY := cy + 4*math.Sin(rad) - 9
	drawTriangle(dst,
		float32(finX), float32(finY),
		float32(finX-8), float32(finY-11),
		float32(finX+8), float32(finY-7),
		colorFishTail,
	)

	// 腹鳍（增加一个小腹鳍，让鱼更有立体感）
	vfinX := cx - 4*math.Cos(rad)
	vfinY := cy - 4*math.Sin(rad) + 9
	drawTriangle(dst,
		float32(vfinX), float32(vfinY),
		float32(vfinX-5), float32(vfinY+8),
		float32(vfinX+5), float32(vfinY+5),
		colorFishTail,
	)
}

// drawKeyboard 绘制科幻感键盘
func drawKeyboard(dst *ebiten.Image, cx, cy float64) {
	kw := float32(64)
	kh := float32(22)
	kx := float32(cx) - kw/2
	ky := float32(cy)

	drawRoundRect(dst, kx, ky, kw, kh, 4, colorKeyboard)
	vector.StrokeRect(dst, kx, ky, kw, kh, 1.5, colorKeyboardHL, true)

	keyColors := []color.RGBA{colorKeyTop, colorKeyboardHL, colorKeyTop}
	for row := 0; row < 3; row++ {
		keysInRow := []int{8, 7, 6}[row]
		keyW := float32(kw-8) / float32(keysInRow)
		for col := 0; col < keysInRow; col++ {
			kbx := kx + 4 + float32(col)*keyW
			kby := ky + 3 + float32(row)*6
			drawRoundRect(dst, kbx, kby, keyW-1.5, 4, 1, keyColors[row%len(keyColors)])
		}
	}
	vector.StrokeLine(dst, kx+2, ky+kh-1, kx+kw-2, ky+kh-1, 1, colorKeyboardHL, true)
}

// drawTypingWings 绘制敲键盘状态的翅膀末端（交替敲击）
func drawTypingWings(dst *ebiten.Image, cx, cy float64, tick int) {
	leftDown := (tick/4)%2 == 0

	leftY := cy + 32
	rightY := cy + 32
	if leftDown {
		leftY += 8
	} else {
		rightY += 8
	}

	// 左翅膀末端
	vector.StrokeLine(dst, float32(cx-14), float32(cy+6), float32(cx-28), float32(leftY), 4, colorWingMain, true)
	vector.DrawFilledCircle(dst, float32(cx-28), float32(leftY), 5, colorWingTip, true)

	// 右翅膀末端
	vector.StrokeLine(dst, float32(cx+14), float32(cy+6), float32(cx+28), float32(rightY), 4, colorWingMain, true)
	vector.DrawFilledCircle(dst, float32(cx+28), float32(rightY), 5, colorWingTip, true)

	// 敲击光效
	if leftDown {
		vector.DrawFilledCircle(dst, float32(cx-28), float32(leftY+3), 8, color.RGBA{100, 180, 255, 80}, true)
	} else {
		vector.DrawFilledCircle(dst, float32(cx+28), float32(rightY+3), 8, color.RGBA{100, 180, 255, 80}, true)
	}
}

// drawTriangle 绘制实心三角形
func drawTriangle(dst *ebiten.Image, x0, y0, x1, y1, x2, y2 float32, clr color.RGBA) {
	var path vector.Path
	path.MoveTo(x0, y0)
	path.LineTo(x1, y1)
	path.LineTo(x2, y2)
	path.Close()
	vs, is := path.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].ColorR = float32(clr.R) / 255
		vs[i].ColorG = float32(clr.G) / 255
		vs[i].ColorB = float32(clr.B) / 255
		vs[i].ColorA = float32(clr.A) / 255
	}
	dst.DrawTriangles(vs, is, emptyImage, &ebiten.DrawTrianglesOptions{})
}

// emptyImage 用于三角形绘制的空白纹理
var emptyImage = func() *ebiten.Image {
	img := ebiten.NewImage(1, 1)
	img.Fill(color.White)
	return img
}()

// drawRoundRect 绘制圆角矩形
func drawRoundRect(dst *ebiten.Image, x, y, w, h, r float32, clr color.RGBA) {
	vector.DrawFilledRect(dst, x+r, y, w-2*r, h, clr, true)
	vector.DrawFilledRect(dst, x, y+r, w, h-2*r, clr, true)
	vector.DrawFilledCircle(dst, x+r, y+r, r, clr, true)
	vector.DrawFilledCircle(dst, x+w-r, y+r, r, clr, true)
	vector.DrawFilledCircle(dst, x+r, y+h-r, r, clr, true)
	vector.DrawFilledCircle(dst, x+w-r, y+h-r, r, clr, true)
}

// DrawAlertFrame 绘制告警惊讶状态动画帧
// 表现：双眼睁大惊讶、翅膀快速乱扇、身体剧烈抖动、红色警报光晕闪烁
func DrawAlertFrame(dst *ebiten.Image, tick int) {
	dst.Fill(color.RGBA{0, 0, 0, 0})

	// 身体剧烈抖动：高频率、大幅度
	bodyShakeX := math.Sin(float64(tick)*1.8) * 5.0
	bodyShakeY := math.Cos(float64(tick)*2.1) * 4.0

	// 翅膀快速乱扇：频率是 idle 的 4 倍，幅度更大
	wingAngle := math.Sin(float64(tick)*0.55) * 40.0

	// 头顶羽毛因惊吓而竖立抖动
	tuftSwing := math.Sin(float64(tick)*1.5) * 12.0

	cx := float64(spriteWidth)/2 + bodyShakeX
	cy := float64(spriteHeight)/2 + bodyShakeY

	// 红色警报光晕（闪烁效果：每 8 帧切换一次强弱）
	glowAlpha := uint8(120 + int(math.Sin(float64(tick)*0.4)*80))
	alertGlowOuter := color.RGBA{255, 30, 30, glowAlpha / 2}
	alertGlowInner := color.RGBA{255, 80, 80, glowAlpha}
	vector.DrawFilledCircle(dst, float32(cx), float32(cy), 58, alertGlowOuter, true)
	vector.DrawFilledCircle(dst, float32(cx), float32(cy), 44, alertGlowInner, true)

	drawTail(dst, cx, cy, bodyShakeX*0.5)
	drawWingsDing(dst, cx, cy, wingAngle)
	drawBodyDing(dst, cx, cy)
	drawTuft(dst, cx, cy, tuftSwing)
	drawAlertEyes(dst, cx, cy)
	drawAlertBeak(dst, cx, cy, tick)
	drawLightning(dst, cx, cy)
	drawAlertWings(dst, cx, cy, tick)
}

// drawAlertEyes 绘制惊讶状态的大眼睛：双眼都睁得很大，瞳孔收缩
func drawAlertEyes(dst *ebiten.Image, cx, cy float64) {
	eyeY := cy - 18

	// 左眼：比正常更大的圆眼，瞳孔缩小（惊讶感）
	vector.DrawFilledCircle(dst, float32(cx)-8, float32(eyeY), 11, colorEyeWhite, true)
	vector.DrawFilledCircle(dst, float32(cx)-8, float32(eyeY), 5, colorEyePupil, true)
	// 高光（两个高光点，更显惊讶）
	vector.DrawFilledCircle(dst, float32(cx)-5.5, float32(eyeY)-2, 2, colorEyeShine, true)
	vector.DrawFilledCircle(dst, float32(cx)-10, float32(eyeY)-3, 1.2, colorEyeShine, true)

	// 右眼：也睁大（告警时两眼都圆睁，不再眯眼）
	vector.DrawFilledCircle(dst, float32(cx)+14, float32(eyeY), 10, colorEyeWhite, true)
	vector.DrawFilledCircle(dst, float32(cx)+14, float32(eyeY), 4, colorEyePupil, true)
	vector.DrawFilledCircle(dst, float32(cx)+16.5, float32(eyeY)-2, 1.8, colorEyeShine, true)
	vector.DrawFilledCircle(dst, float32(cx)+12, float32(eyeY)-3, 1.0, colorEyeShine, true)

	// 眼睛周围的红色紧张线条（放射状短线，表现惊讶）
	alertLineColor := color.RGBA{255, 60, 60, 180}
	for i := 0; i < 6; i++ {
		angle := float64(i) * math.Pi / 3.0
		lineStartR := 12.0
		lineEndR := 17.0
		// 左眼周围
		lsx := float32(cx) - 8 + float32(lineStartR*math.Cos(angle))
		lsy := float32(eyeY) + float32(lineStartR*math.Sin(angle))
		lex := float32(cx) - 8 + float32(lineEndR*math.Cos(angle))
		ley := float32(eyeY) + float32(lineEndR*math.Sin(angle))
		vector.StrokeLine(dst, lsx, lsy, lex, ley, 1.5, alertLineColor, true)
	}
}

// drawAlertBeak 绘制惊讶状态的嘴巴：张开的 O 形嘴
func drawAlertBeak(dst *ebiten.Image, cx, cy float64, tick int) {
	beakY := cy - 10
	// 嘴巴张开大小随时间轻微变化（喘气感）
	mouthSize := 5.0 + math.Sin(float64(tick)*0.8)*1.5
	vector.DrawFilledCircle(dst, float32(cx), float32(beakY)+3, float32(mouthSize), colorBeak, true)
	// 嘴巴内部深色（张开感）
	vector.DrawFilledCircle(dst, float32(cx), float32(beakY)+3, float32(mouthSize*0.5), color.RGBA{10, 10, 15, 255}, true)
}

// drawAlertWings 绘制告警状态的翅膀末端：向上张开，表现惊慌失措
func drawAlertWings(dst *ebiten.Image, cx, cy float64, tick int) {
	// 翅膀末端快速上下扇动
	leftPhase := math.Sin(float64(tick)*0.6) * 18.0
	rightPhase := math.Sin(float64(tick)*0.6+math.Pi) * 18.0

	leftWingY := cy - 5 + leftPhase
	rightWingY := cy - 5 + rightPhase

	// 左翅膀末端（向上张开）
	vector.StrokeLine(dst, float32(cx-14), float32(cy+6), float32(cx-32), float32(leftWingY), 4, colorWingMain, true)
	vector.DrawFilledCircle(dst, float32(cx-32), float32(leftWingY), 5, colorWingTip, true)

	// 右翅膀末端
	vector.StrokeLine(dst, float32(cx+14), float32(cy+6), float32(cx+32), float32(rightWingY), 4, colorWingMain, true)
	vector.DrawFilledCircle(dst, float32(cx+32), float32(rightWingY), 5, colorWingTip, true)

	// 翅膀扇动时的红色气流效果
	alertGlow := color.RGBA{255, 80, 80, 60}
	vector.DrawFilledCircle(dst, float32(cx-32), float32(leftWingY), 10, alertGlow, true)
	vector.DrawFilledCircle(dst, float32(cx+32), float32(rightWingY), 10, alertGlow, true)
}

// drawAlertBubble 在宠物嘴巴旁绘制秘书汇报对话气泡
// beakX/beakY 为宠物嘴巴的屏幕坐标
func drawAlertBubble(dst *ebiten.Image, bubble *alertBubble, face font.Face, beakX, beakY float64) {
	alpha := bubbleAlpha(bubble)
	if alpha == 0 {
		return
	}

	const (
		paddingH    = 12 // 水平内边距
		paddingV    = 10 // 垂直内边距
		lineSpacing = 20 // 行间距（像素）
		cornerR     = 10 // 圆角半径
		tailSize    = 10 // 气泡尾巴大小
	)

	lineCount := len(bubble.lines)
	if lineCount == 0 {
		return
	}

	// 计算气泡尺寸
	bubbleW := float32(bubbleMaxWidth + paddingH*2)
	bubbleH := float32(lineCount*lineSpacing + paddingV*2)

	// 气泡位置：在嘴巴左上方弹出
	// 气泡右下角对齐嘴巴左侧，留出尾巴空间
	bubbleRight := float32(beakX) - float32(tailSize) + 10
	bubbleBottom := float32(beakY) - float32(tailSize)
	bubbleLeft := bubbleRight - bubbleW
	bubbleTop := bubbleBottom - bubbleH

	// 防止气泡超出屏幕左边
	if bubbleLeft < 5 {
		offset := 5 - bubbleLeft
		bubbleLeft += offset
		bubbleRight += offset
	}

	// 透明度混合颜色
	applyAlpha := func(c color.RGBA) color.RGBA {
		return color.RGBA{c.R, c.G, c.B, uint8(float64(c.A) * float64(alpha) / 255)}
	}

	bgColor := applyAlpha(color.RGBA{15, 25, 50, 230})
	borderColor := applyAlpha(color.RGBA{80, 180, 255, 220})
	textColor := applyAlpha(color.RGBA{220, 240, 255, 255})
	titleColor := applyAlpha(color.RGBA{255, 200, 80, 255})

	// 绘制气泡背景（圆角矩形）
	drawRoundRect(dst, bubbleLeft, bubbleTop, bubbleW, bubbleH, cornerR, bgColor)

	// 绘制气泡边框
	vector.StrokeRect(dst, bubbleLeft+1, bubbleTop+1, bubbleW-2, bubbleH-2, 1.5, borderColor, true)

	// 绘制气泡尾巴（三角形，指向嘴巴方向）
	tailTipX := float32(beakX)
	tailTipY := float32(beakY)
	tailBaseX := bubbleRight - 4
	tailBaseY := bubbleBottom
	drawTriangle(dst,
		tailBaseX-float32(tailSize)*0.6, tailBaseY,
		tailBaseX, tailBaseY,
		tailTipX, tailTipY,
		bgColor,
	)
	// 尾巴边框线
	vector.StrokeLine(dst, tailBaseX-float32(tailSize)*0.6, tailBaseY, tailTipX, tailTipY, 1.5, borderColor, true)
	vector.StrokeLine(dst, tailBaseX, tailBaseY, tailTipX, tailTipY, 1.5, borderColor, true)

	// 绘制顶部装饰线（科幻感）
	vector.StrokeLine(dst,
		bubbleLeft+cornerR, bubbleTop+1,
		bubbleRight-cornerR, bubbleTop+1,
		1, applyAlpha(color.RGBA{150, 220, 255, 180}), true,
	)

	// 绘制文字（如果有字体）
	// 使用 recover 防止字体库在渲染特殊字符时触发 panic（golang.org/x/image 已知 bug）
	if face != nil {
		textX := int(bubbleLeft) + paddingH
		for i, line := range bubble.lines {
			textY := int(bubbleTop) + paddingV + (i+1)*lineSpacing - 2
			// 第一行用高亮色（标题感），其余用普通色
			lineColor := textColor
			if i == 0 {
				lineColor = titleColor
			}
			safeDrawText(dst, line, face, textX, textY, lineColor)
		}
	}
}

// safeDrawText 安全地绘制文字，捕获字体库可能触发的 panic（如特殊字符导致的 index out of range）
func safeDrawText(dst *ebiten.Image, str string, face font.Face, x, y int, clr color.Color) {
	defer func() {
		if r := recover(); r != nil {
			// 字体库渲染特殊字符时可能 panic，静默忽略，不影响程序运行
			_ = r
		}
	}()
	text.Draw(dst, str, face, x, y, clr)
}
