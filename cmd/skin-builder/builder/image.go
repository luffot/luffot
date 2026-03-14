package builder

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// loadSourceImage 从文件路径加载原始图片，支持 PNG/JPG/GIF/WEBP
func loadSourceImage(imagePath string) (image.Image, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("打开图片失败：%w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(imagePath))
	switch ext {
	case ".gif":
		gifData, err := gif.DecodeAll(file)
		if err != nil {
			return nil, fmt.Errorf("解码 GIF 失败：%w", err)
		}
		if len(gifData.Image) == 0 {
			return nil, fmt.Errorf("GIF 文件没有帧")
		}
		// 取第一帧作为基础图片
		return gifData.Image[0], nil
	case ".jpg", ".jpeg":
		img, err := jpeg.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("解码 JPG 失败：%w", err)
		}
		return img, nil
	default:
		// PNG 和其他格式使用标准解码
		img, _, err := image.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("解码图片失败：%w", err)
		}
		return img, nil
	}
}

// cropToSquare 将图片裁剪为正方形（取中心区域）
func cropToSquare(src image.Image) image.Image {
	bounds := src.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	if width == height {
		return src
	}

	size := width
	if height < width {
		size = height
	}

	offsetX := (width - size) / 2
	offsetY := (height - size) / 2

	cropped := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			cropped.Set(x, y, src.At(bounds.Min.X+offsetX+x, bounds.Min.Y+offsetY+y))
		}
	}
	return cropped
}

// resizeImage 将图片缩放到指定尺寸（双线性插值）
func resizeImage(src image.Image, targetSize int) *image.RGBA {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Max.X - srcBounds.Min.X
	srcHeight := srcBounds.Max.Y - srcBounds.Min.Y

	dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))

	scaleX := float64(srcWidth) / float64(targetSize)
	scaleY := float64(srcHeight) / float64(targetSize)

	for dstY := 0; dstY < targetSize; dstY++ {
		for dstX := 0; dstX < targetSize; dstX++ {
			// 双线性插值
			srcXFloat := (float64(dstX)+0.5)*scaleX - 0.5
			srcYFloat := (float64(dstY)+0.5)*scaleY - 0.5

			x0 := int(math.Floor(srcXFloat))
			y0 := int(math.Floor(srcYFloat))
			x1 := x0 + 1
			y1 := y0 + 1

			// 边界夹紧
			x0 = clampInt(x0, srcBounds.Min.X, srcBounds.Max.X-1)
			y0 = clampInt(y0, srcBounds.Min.Y, srcBounds.Max.Y-1)
			x1 = clampInt(x1, srcBounds.Min.X, srcBounds.Max.X-1)
			y1 = clampInt(y1, srcBounds.Min.Y, srcBounds.Max.Y-1)

			// 插值权重
			wx := srcXFloat - math.Floor(srcXFloat)
			wy := srcYFloat - math.Floor(srcYFloat)

			c00 := colorToFloat(src.At(x0, y0))
			c10 := colorToFloat(src.At(x1, y0))
			c01 := colorToFloat(src.At(x0, y1))
			c11 := colorToFloat(src.At(x1, y1))

			r := lerp(lerp(c00[0], c10[0], wx), lerp(c01[0], c11[0], wx), wy)
			g := lerp(lerp(c00[1], c10[1], wx), lerp(c01[1], c11[1], wx), wy)
			b := lerp(lerp(c00[2], c10[2], wx), lerp(c01[2], c11[2], wx), wy)
			a := lerp(lerp(c00[3], c10[3], wx), lerp(c01[3], c11[3], wx), wy)

			dst.SetRGBA(dstX, dstY, color.RGBA{
				R: floatToUint8(r),
				G: floatToUint8(g),
				B: floatToUint8(b),
				A: floatToUint8(a),
			})
		}
	}
	return dst
}

// removeBackground 去除纯色背景（将接近白色或指定颜色的像素变为透明）
// 使用边缘洪水填充算法，只去除从边缘连通的背景色，保留内部相似颜色
func removeBackground(src *image.RGBA, threshold uint8) *image.RGBA {
	bounds := src.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y

	// 采样四个角的颜色作为背景色参考
	bgColor := sampleBackgroundColor(src, width, height)

	// 创建访问标记
	visited := make([][]bool, height)
	for i := range visited {
		visited[i] = make([]bool, width)
	}

	// 从四条边缘开始洪水填充
	queue := make([]image.Point, 0, width*2+height*2)
	for x := 0; x < width; x++ {
		queue = append(queue, image.Point{x, 0}, image.Point{x, height - 1})
	}
	for y := 1; y < height-1; y++ {
		queue = append(queue, image.Point{0, y}, image.Point{width - 1, y})
	}

	dst := image.NewRGBA(bounds)
	// 先复制原图
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dst.SetRGBA(x, y, src.RGBAAt(x, y))
		}
	}

	// 洪水填充，将背景像素变透明
	for len(queue) > 0 {
		point := queue[0]
		queue = queue[1:]

		px, py := point.X, point.Y
		if px < 0 || px >= width || py < 0 || py >= height || visited[py][px] {
			continue
		}
		visited[py][px] = true

		pixelColor := src.RGBAAt(px, py)
		if !colorSimilar(pixelColor, bgColor, threshold) {
			continue
		}

		// 将背景像素变为透明
		dst.SetRGBA(px, py, color.RGBA{0, 0, 0, 0})

		// 扩展到四邻域
		queue = append(queue,
			image.Point{px + 1, py},
			image.Point{px - 1, py},
			image.Point{px, py + 1},
			image.Point{px, py - 1},
		)
	}

	return dst
}

// sampleBackgroundColor 采样图片四角颜色，取最常见的作为背景色
func sampleBackgroundColor(src *image.RGBA, width, height int) color.RGBA {
	corners := []color.RGBA{
		src.RGBAAt(0, 0),
		src.RGBAAt(width-1, 0),
		src.RGBAAt(0, height-1),
		src.RGBAAt(width-1, height-1),
	}

	// 简单取第一个角的颜色（通常背景色一致）
	// 如果四角颜色差异大，取平均值
	var totalR, totalG, totalB, totalA int
	for _, c := range corners {
		totalR += int(c.R)
		totalG += int(c.G)
		totalB += int(c.B)
		totalA += int(c.A)
	}
	return color.RGBA{
		R: uint8(totalR / 4),
		G: uint8(totalG / 4),
		B: uint8(totalB / 4),
		A: uint8(totalA / 4),
	}
}

// colorSimilar 判断两个颜色是否相似（在阈值范围内）
func colorSimilar(a, b color.RGBA, threshold uint8) bool {
	diffR := absDiff(a.R, b.R)
	diffG := absDiff(a.G, b.G)
	diffB := absDiff(a.B, b.B)
	return diffR <= threshold && diffG <= threshold && diffB <= threshold
}

// PrepareBaseImage 加载、裁剪、缩放原始图片，返回可用于帧生成的基础图。
// removeBGClient 不为 nil 时，优先使用 remove.bg API 去除背景；
// shouldFallbackRemoveBG 为 true 时，在 remove.bg 不可用时回退到本地洪水填充算法。
func PrepareBaseImage(imagePath string, targetSize int, removeBGClient *RemoveBGClient, shouldFallbackRemoveBG bool) (*image.RGBA, error) {
	// 优先使用 remove.bg API 去除背景（在缩放前处理，保留最高质量）
	if removeBGClient != nil {
		fmt.Println("  🌐 正在调用 remove.bg API 去除背景...")
		bgRemoved, err := removeBGClient.RemoveBackground(imagePath)
		if err != nil {
			fmt.Printf("  ⚠️  remove.bg 调用失败：%v\n", err)
			fmt.Println("  ↩️  回退到本地加载原图...")
			// 回退：加载原图，不去背景
			src, loadErr := loadSourceImage(imagePath)
			if loadErr != nil {
				return nil, loadErr
			}
			squared := cropToSquare(src)
			resized := resizeImage(squared, targetSize)
			if shouldFallbackRemoveBG {
				fmt.Println("  🔧 使用本地算法去除背景（效果有限）...")
				resized = removeBackground(resized, 30)
			}
			return resized, nil
		}
		fmt.Println("  ✅ remove.bg 抠图成功")
		// remove.bg 返回的图片已经是透明背景，直接裁剪缩放
		squared := cropToSquare(bgRemoved)
		return resizeImage(squared, targetSize), nil
	}

	// 无 remove.bg 客户端：加载原图
	src, err := loadSourceImage(imagePath)
	if err != nil {
		return nil, err
	}

	squared := cropToSquare(src)
	resized := resizeImage(squared, targetSize)

	// 本地洪水填充去背景（可选）
	if shouldFallbackRemoveBG {
		resized = removeBackground(resized, 30)
	}

	return resized, nil
}

// savePNG 将图片保存为 PNG 文件
func savePNG(img image.Image, filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("创建目录失败：%w", err)
	}
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败：%w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("编码 PNG 失败：%w", err)
	}
	return nil
}

// ─── 工具函数 ───────────────────────────────────────────────

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func colorToFloat(c color.Color) [4]float64 {
	r, g, b, a := c.RGBA()
	return [4]float64{
		float64(r) / 65535.0,
		float64(g) / 65535.0,
		float64(b) / 65535.0,
		float64(a) / 65535.0,
	}
}

func floatToUint8(value float64) uint8 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 255
	}
	return uint8(value * 255)
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
