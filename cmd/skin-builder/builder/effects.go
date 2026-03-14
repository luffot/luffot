package builder

import (
	"image"
	"image/color"
	"math"
)

// RenderFrame 根据特效配置渲染单帧图片
// baseImage：基础图片（已缩放到目标尺寸）
// frameIndex：当前帧索引（0 开始）
// totalFrames：总帧数
// stateCfg：该状态的特效配置
func RenderFrame(baseImage *image.RGBA, frameIndex, totalFrames int, stateCfg *StateConfig) *image.RGBA {
	// 动画进度 t：0.0 ~ 1.0（循环）
	progress := float64(frameIndex) / float64(totalFrames)

	switch stateCfg.Effect {
	case EffectNone:
		return copyImage(baseImage)
	case EffectBob:
		return applyBob(baseImage, progress, stateCfg.EffectIntensity)
	case EffectShake:
		return applyShake(baseImage, progress, stateCfg.EffectIntensity)
	case EffectSpin:
		return applySpin(baseImage, progress, stateCfg.EffectIntensity)
	case EffectPulse:
		return applyPulse(baseImage, progress, stateCfg.EffectIntensity)
	case EffectGlow:
		return applyGlow(baseImage, progress, stateCfg.GlowColor, stateCfg.EffectIntensity)
	case EffectParticle:
		return applyParticle(baseImage, progress, stateCfg.ParticleType, stateCfg.EffectIntensity)
	case EffectRainbow:
		return applyRainbow(baseImage, progress, stateCfg.EffectIntensity)
	case EffectFlash:
		return applyFlash(baseImage, progress, stateCfg.EffectIntensity)
	case EffectBounce:
		return applyBounce(baseImage, progress, stateCfg.EffectIntensity)
	case EffectWave:
		return applyWave(baseImage, progress, stateCfg.EffectIntensity)
	default:
		return copyImage(baseImage)
	}
}

// copyImage 复制图片（避免修改原图）
func copyImage(src *image.RGBA) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	copy(dst.Pix, src.Pix)
	return dst
}

// translateImage 将图片平移（offsetX, offsetY 为像素偏移，超出边界填透明）
func translateImage(src *image.RGBA, offsetX, offsetY int) *image.RGBA {
	bounds := src.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y
	dst := image.NewRGBA(bounds)

	for dstY := 0; dstY < height; dstY++ {
		for dstX := 0; dstX < width; dstX++ {
			srcX := dstX - offsetX
			srcY := dstY - offsetY
			if srcX >= 0 && srcX < width && srcY >= 0 && srcY < height {
				dst.SetRGBA(dstX, dstY, src.RGBAAt(srcX, srcY))
			}
		}
	}
	return dst
}

// scaleImageCentered 以图片中心为基准缩放图片
func scaleImageCentered(src *image.RGBA, scale float64) *image.RGBA {
	bounds := src.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y
	dst := image.NewRGBA(bounds)

	cx := float64(width) / 2.0
	cy := float64(height) / 2.0

	for dstY := 0; dstY < height; dstY++ {
		for dstX := 0; dstX < width; dstX++ {
			// 从目标坐标反推源坐标
			srcXFloat := cx + (float64(dstX)-cx)/scale
			srcYFloat := cy + (float64(dstY)-cy)/scale

			srcX := int(math.Round(srcXFloat))
			srcY := int(math.Round(srcYFloat))

			if srcX >= 0 && srcX < width && srcY >= 0 && srcY < height {
				dst.SetRGBA(dstX, dstY, src.RGBAAt(srcX, srcY))
			}
		}
	}
	return dst
}

// rotateImageCentered 以图片中心为基准旋转图片（angle 为弧度）
func rotateImageCentered(src *image.RGBA, angle float64) *image.RGBA {
	bounds := src.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y
	dst := image.NewRGBA(bounds)

	cx := float64(width) / 2.0
	cy := float64(height) / 2.0
	cosA := math.Cos(-angle)
	sinA := math.Sin(-angle)

	for dstY := 0; dstY < height; dstY++ {
		for dstX := 0; dstX < width; dstX++ {
			// 从目标坐标反推源坐标（逆旋转）
			dx := float64(dstX) - cx
			dy := float64(dstY) - cy
			srcXFloat := cx + dx*cosA - dy*sinA
			srcYFloat := cy + dx*sinA + dy*cosA

			srcX := int(math.Round(srcXFloat))
			srcY := int(math.Round(srcYFloat))

			if srcX >= 0 && srcX < width && srcY >= 0 && srcY < height {
				dst.SetRGBA(dstX, dstY, src.RGBAAt(srcX, srcY))
			}
		}
	}
	return dst
}

// blendColor 将颜色叠加到图片上（仅对不透明像素生效）
func blendColor(src *image.RGBA, blendR, blendG, blendB uint8, alpha float64) *image.RGBA {
	dst := copyImage(src)
	bounds := dst.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := dst.RGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			c.R = uint8(float64(c.R)*(1-alpha) + float64(blendR)*alpha)
			c.G = uint8(float64(c.G)*(1-alpha) + float64(blendG)*alpha)
			c.B = uint8(float64(c.B)*(1-alpha) + float64(blendB)*alpha)
			dst.SetRGBA(x, y, c)
		}
	}
	return dst
}

// drawGlowCircle 在图片上绘制光晕圆（叠加到透明区域外围）
func drawGlowCircle(dst *image.RGBA, cx, cy, radius float64, glowColor [3]uint8, alpha float64) {
	bounds := dst.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dist := math.Sqrt(math.Pow(float64(x)-cx, 2) + math.Pow(float64(y)-cy, 2))
			if dist > radius {
				continue
			}
			// 光晕强度随距离衰减
			intensity := (1.0 - dist/radius) * alpha
			existing := dst.RGBAAt(x, y)

			// 叠加光晕颜色
			newR := clampFloat(float64(existing.R)+float64(glowColor[0])*intensity, 0, 255)
			newG := clampFloat(float64(existing.G)+float64(glowColor[1])*intensity, 0, 255)
			newB := clampFloat(float64(existing.B)+float64(glowColor[2])*intensity, 0, 255)
			newA := clampFloat(float64(existing.A)+255*intensity, 0, 255)

			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(newR),
				G: uint8(newG),
				B: uint8(newB),
				A: uint8(newA),
			})
		}
	}
}

// drawParticle 在指定位置绘制一个粒子
func drawParticle(dst *image.RGBA, x, y int, particleType string, size float64, alpha float64) {
	particleColor := color.RGBA{255, 220, 50, uint8(alpha * 255)}

	switch particleType {
	case "heart":
		particleColor = color.RGBA{255, 100, 150, uint8(alpha * 255)}
	case "note":
		particleColor = color.RGBA{100, 200, 255, uint8(alpha * 255)}
	case "sparkle":
		particleColor = color.RGBA{255, 255, 200, uint8(alpha * 255)}
	}

	radius := int(size)
	bounds := dst.Bounds()
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			dist := math.Sqrt(float64(dx*dx + dy*dy))
			if dist > size {
				continue
			}
			px, py := x+dx, y+dy
			if px < 0 || px >= bounds.Max.X || py < 0 || py >= bounds.Max.Y {
				continue
			}
			// 粒子边缘柔化
			edgeAlpha := (1.0 - dist/size) * alpha
			existing := dst.RGBAAt(px, py)
			blended := blendPixel(existing, particleColor, edgeAlpha)
			dst.SetRGBA(px, py, blended)
		}
	}
}

// blendPixel 将前景色以指定透明度叠加到背景色上
func blendPixel(bg, fg color.RGBA, fgAlpha float64) color.RGBA {
	invAlpha := 1.0 - fgAlpha
	return color.RGBA{
		R: uint8(float64(bg.R)*invAlpha + float64(fg.R)*fgAlpha),
		G: uint8(float64(bg.G)*invAlpha + float64(fg.G)*fgAlpha),
		B: uint8(float64(bg.B)*invAlpha + float64(fg.B)*fgAlpha),
		A: uint8(math.Min(float64(bg.A)+float64(fg.A)*fgAlpha, 255)),
	}
}

// ─── 各特效实现 ───────────────────────────────────────────────

// applyBob 上下浮动（呼吸感）
func applyBob(src *image.RGBA, progress, intensity float64) *image.RGBA {
	maxOffset := int(math.Round(intensity * 8.0)) // 最大偏移 8 像素
	offsetY := int(math.Round(math.Sin(progress*2*math.Pi) * float64(maxOffset)))
	return translateImage(src, 0, offsetY)
}

// applyShake 左右抖动
func applyShake(src *image.RGBA, progress, intensity float64) *image.RGBA {
	maxOffset := int(math.Round(intensity * 10.0)) // 最大偏移 10 像素
	// 高频抖动：使用 sin(4π*t) 实现来回抖动
	offsetX := int(math.Round(math.Sin(progress*4*math.Pi) * float64(maxOffset)))
	return translateImage(src, offsetX, 0)
}

// applySpin 旋转
func applySpin(src *image.RGBA, progress, intensity float64) *image.RGBA {
	// 完整旋转一圈
	angle := progress * 2 * math.Pi
	// 低强度时只旋转部分角度（摇摆）
	if intensity < 0.7 {
		angle = math.Sin(progress*2*math.Pi) * math.Pi * intensity
	}
	return rotateImageCentered(src, angle)
}

// applyPulse 缩放脉冲（心跳感）
func applyPulse(src *image.RGBA, progress, intensity float64) *image.RGBA {
	// 缩放范围：1.0 ~ 1.0+intensity*0.2
	scaleRange := intensity * 0.2
	scale := 1.0 + math.Sin(progress*2*math.Pi)*scaleRange
	return scaleImageCentered(src, scale)
}

// applyGlow 发光光晕
func applyGlow(src *image.RGBA, progress float64, glowColor [3]uint8, intensity float64) *image.RGBA {
	dst := copyImage(src)
	bounds := dst.Bounds()
	cx := float64(bounds.Max.X) / 2.0
	cy := float64(bounds.Max.Y) / 2.0

	// 光晕半径随时间脉动
	baseRadius := float64(bounds.Max.X) * 0.45
	radiusPulse := math.Sin(progress*2*math.Pi) * baseRadius * 0.15
	radius := baseRadius + radiusPulse

	// 光晕强度随时间变化
	glowAlpha := (0.3 + math.Sin(progress*2*math.Pi)*0.2) * intensity

	drawGlowCircle(dst, cx, cy, radius, glowColor, glowAlpha)
	return dst
}

// applyParticle 粒子特效（星星/音符/爱心飘散）
func applyParticle(src *image.RGBA, progress float64, particleType string, intensity float64) *image.RGBA {
	dst := copyImage(src)
	bounds := dst.Bounds()
	width := float64(bounds.Max.X)
	height := float64(bounds.Max.Y)

	// 生成 6 个粒子，各自有不同的相位偏移
	particleCount := 6
	for i := 0; i < particleCount; i++ {
		phase := float64(i) / float64(particleCount)
		particleProgress := math.Mod(progress+phase, 1.0)

		// 粒子从中心向外飘散，同时向上移动
		startX := width * 0.5
		startY := height * 0.6
		spreadX := (float64(i%3) - 1.0) * width * 0.3
		spreadY := -height * 0.5

		particleX := int(startX + spreadX*particleProgress)
		particleY := int(startY + spreadY*particleProgress)

		// 粒子大小和透明度随生命周期变化（出现→消失）
		lifeAlpha := math.Sin(particleProgress * math.Pi)
		particleSize := 3.0 + lifeAlpha*3.0*intensity
		particleAlpha := lifeAlpha * intensity

		drawParticle(dst, particleX, particleY, particleType, particleSize, particleAlpha)
	}
	return dst
}

// applyRainbow 彩虹色调变换
func applyRainbow(src *image.RGBA, progress, intensity float64) *image.RGBA {
	// 将 HSV 色相旋转映射到 RGB
	hue := progress * 360.0
	r, g, b := hsvToRGB(hue, 0.8, 1.0)
	return blendColor(src, r, g, b, intensity*0.4)
}

// applyFlash 闪烁（亮度交替）
func applyFlash(src *image.RGBA, progress, intensity float64) *image.RGBA {
	// 高频闪烁：每帧交替亮/暗
	flashValue := math.Sin(progress * 4 * math.Pi)
	if flashValue > 0 {
		// 亮帧：叠加白色
		return blendColor(src, 255, 255, 255, flashValue*intensity*0.5)
	}
	// 暗帧：正常显示
	return copyImage(src)
}

// applyBounce 弹跳
func applyBounce(src *image.RGBA, progress, intensity float64) *image.RGBA {
	// 使用绝对值 sin 模拟弹跳（落地后反弹）
	bounceHeight := intensity * 20.0 // 最大弹跳高度 20 像素
	// abs(sin) 产生弹跳效果，频率加倍让弹跳更快
	offsetY := -int(math.Round(math.Abs(math.Sin(progress*2*math.Pi)) * bounceHeight))
	return translateImage(src, 0, offsetY)
}

// applyWave 波浪摆动（左右摇摆，类似海草）
func applyWave(src *image.RGBA, progress, intensity float64) *image.RGBA {
	bounds := src.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y
	dst := image.NewRGBA(bounds)

	maxOffset := intensity * 8.0 // 最大水平偏移 8 像素
	waveAngle := progress * 2 * math.Pi

	for y := 0; y < height; y++ {
		// 越靠近顶部摆动越大（底部固定，顶部摇摆）
		verticalFactor := 1.0 - float64(y)/float64(height)
		offsetX := int(math.Round(math.Sin(waveAngle) * maxOffset * verticalFactor))

		for x := 0; x < width; x++ {
			srcX := x - offsetX
			if srcX >= 0 && srcX < width {
				dst.SetRGBA(x, y, src.RGBAAt(srcX, y))
			}
		}
	}
	return dst
}

// ─── 颜色工具 ───────────────────────────────────────────────

// hsvToRGB 将 HSV 颜色转换为 RGB（h: 0~360, s/v: 0~1）
func hsvToRGB(h, s, v float64) (uint8, uint8, uint8) {
	h = math.Mod(h, 360.0)
	if h < 0 {
		h += 360.0
	}
	sectorIndex := int(h / 60.0)
	sectorFraction := h/60.0 - float64(sectorIndex)

	p := v * (1 - s)
	q := v * (1 - s*sectorFraction)
	t := v * (1 - s*(1-sectorFraction))

	var r, g, b float64
	switch sectorIndex {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}

	return uint8(r * 255), uint8(g * 255), uint8(b * 255)
}
