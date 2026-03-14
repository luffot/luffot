package builder

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// RemoveBGClient remove.bg API 客户端
type RemoveBGClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewRemoveBGClient 创建 remove.bg 客户端
func NewRemoveBGClient(cfg RemoveBGConfig) *RemoveBGClient {
	return &RemoveBGClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.EffectiveBaseURL(),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// RemoveBackground 调用 remove.bg API 去除图片背景，返回透明背景的 RGBA 图片
func (c *RemoveBGClient) RemoveBackground(imagePath string) (*image.RGBA, error) {
	// 打开原始图片文件
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("打开图片失败：%w", err)
	}
	defer file.Close()

	// 构建 multipart/form-data 请求体
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// 添加图片文件字段
	part, err := writer.CreateFormFile("image_file", filepath.Base(imagePath))
	if err != nil {
		return nil, fmt.Errorf("创建表单字段失败：%w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("写入图片数据失败：%w", err)
	}

	// 请求返回原始尺寸（不裁剪）
	if err := writer.WriteField("size", "auto"); err != nil {
		return nil, fmt.Errorf("写入 size 字段失败：%w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart writer 失败：%w", err)
	}

	// 发起 HTTP 请求
	requestURL := c.baseURL + "/removebg"
	req, err := http.NewRequest("POST", requestURL, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败：%w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 remove.bg 失败：%w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		// 尝试提取错误信息（remove.bg 错误响应为 JSON）
		return nil, fmt.Errorf("remove.bg 返回错误（HTTP %d）：%s", resp.StatusCode, summarizeErrorBody(responseBody))
	}

	// 解码返回的 PNG 图片
	img, err := png.Decode(bytes.NewReader(responseBody))
	if err != nil {
		return nil, fmt.Errorf("解码 remove.bg 返回的 PNG 失败：%w", err)
	}

	// 转换为 *image.RGBA
	return toRGBA(img), nil
}

// toRGBA 将任意 image.Image 转换为 *image.RGBA
func toRGBA(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Max.X-bounds.Min.X, bounds.Max.Y-bounds.Min.Y))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x-bounds.Min.X, y-bounds.Min.Y, src.At(x, y))
		}
	}
	return dst
}

// summarizeErrorBody 从 remove.bg 错误响应体中提取可读信息（最多 200 字符）
func summarizeErrorBody(body []byte) string {
	text := string(body)
	if len(text) > 200 {
		return text[:200] + "..."
	}
	return text
}
