package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateSkin 根据配置生成完整的皮肤素材目录
// outputDir：输出目录（如 assets/skins/我的猫咪）
// removeBGClient：remove.bg 客户端，为 nil 时不使用 API 抠图
func GenerateSkin(cfg *SkinBuildConfig, outputDir string, removeBGClient *RemoveBGClient) error {
	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败：%w", err)
	}

	// 根据 BackgroundMode 决定抠图方式
	var activeRemoveBGClient *RemoveBGClient
	shouldFallbackLocal := false
	switch cfg.BackgroundMode {
	case "removebg":
		activeRemoveBGClient = removeBGClient
	case "local":
		shouldFallbackLocal = true
	}

	// 加载并预处理基础图片
	fmt.Println("  📷 加载并处理原始图片...")
	baseImage, err := PrepareBaseImage(cfg.ImagePath, cfg.SpriteSize, activeRemoveBGClient, shouldFallbackLocal)
	if err != nil {
		return fmt.Errorf("处理图片失败：%w", err)
	}

	// 逐状态生成帧序列
	for _, state := range AllStates {
		stateCfg := cfg.States[state]
		stateName := string(state)
		displayName := StateDisplayName[state]

		fmt.Printf("  🎬 生成状态 [%s] 的 %d 帧...\n", displayName, stateCfg.FrameCount)

		for frameIndex := 0; frameIndex < stateCfg.FrameCount; frameIndex++ {
			frame := RenderFrame(baseImage, frameIndex, stateCfg.FrameCount, stateCfg)

			fileName := fmt.Sprintf("%s_%d.png", stateName, frameIndex)
			filePath := filepath.Join(outputDir, fileName)

			if err := savePNG(frame, filePath); err != nil {
				return fmt.Errorf("保存帧 %s 失败：%w", fileName, err)
			}
		}

		fmt.Printf("  ✅ [%s] 完成，共 %d 帧\n", displayName, stateCfg.FrameCount)
	}

	// 生成 skin_meta.json
	if err := writeSkinMeta(cfg, outputDir); err != nil {
		return fmt.Errorf("写入皮肤元信息失败：%w", err)
	}

	return nil
}

// writeSkinMeta 将皮肤元信息写入 skin_meta.json
func writeSkinMeta(cfg *SkinBuildConfig, outputDir string) error {
	statesMeta := make(map[string]StateMeta)
	for _, state := range AllStates {
		stateCfg := cfg.States[state]
		statesMeta[string(state)] = StateMeta{
			Effect:     stateCfg.Effect,
			FrameCount: stateCfg.FrameCount,
		}
	}

	meta := SkinMeta{
		Name:             cfg.SkinName,
		Version:          "1.0",
		FullscreenEffect: cfg.FullscreenEffect,
		States:           statesMeta,
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化元信息失败：%w", err)
	}

	metaPath := filepath.Join(outputDir, "skin_meta.json")
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("写入 skin_meta.json 失败：%w", err)
	}

	fmt.Printf("  📝 已写入皮肤元信息：%s\n", metaPath)
	return nil
}
