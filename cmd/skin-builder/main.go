// skin-builder：桌宠皮肤制作 AI 智能体
//
// 使用方式：
//
//	go run ./cmd/skin-builder
//
// 该程序会引导用户上传一张图片，通过 AI 对话收集各状态下的特效偏好，
// 最终生成符合 luffot 皮肤规范的素材目录，可直接放入 assets/skins/ 使用。
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/luffot/luffot/cmd/skin-builder/builder"
)

// appConfig 全局程序配置（启动时加载）
var appConfig *builder.AppConfig

func main() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║          🎨  Luffot 皮肤制作助手  skin-builder        ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  我会引导你一步步制作属于自己的桌宠皮肤。")
	fmt.Println("  制作完成后，皮肤素材会自动放入 assets/skins/ 目录，")
	fmt.Println("  重启桌宠后即可在设置页面选择使用。")
	fmt.Println()

	// 加载配置文件
	var loadConfigErr error
	appConfig, loadConfigErr = builder.LoadAppConfig(builder.DefaultConfigPath())
	if loadConfigErr != nil {
		fmt.Printf("  ⚠️  配置文件加载失败：%v（将使用默认配置继续）\n\n", loadConfigErr)
		appConfig = &builder.AppConfig{}
	}

	// 创建 remove.bg 客户端（未配置时为 nil）
	var removeBGClient *builder.RemoveBGClient
	if appConfig.RemoveBG.IsConfigured() {
		removeBGClient = builder.NewRemoveBGClient(appConfig.RemoveBG)
		fmt.Println("  ✅ 已加载 remove.bg API 配置，支持高质量抠图")
		fmt.Println()
	}

	scanner := bufio.NewScanner(os.Stdin)

	// 第一步：获取图片路径
	imagePath := promptImagePath(scanner)

	// 第二步：获取皮肤名称
	skinName := promptSkinName(scanner)

	// 第三步：启动引导对话，收集各状态特效配置
	hasRemoveBG := removeBGClient != nil
	session := builder.NewSession(imagePath, skinName, scanner, hasRemoveBG)
	skinConfig, err := session.RunGuidedDialog()
	if err != nil {
		fmt.Printf("\n❌ 皮肤配置收集失败：%v\n", err)
		os.Exit(1)
	}

	// 第四步：生成皮肤素材
	fmt.Println()
	fmt.Println("🔨 正在生成皮肤素材，请稍候...")

	outputDir := filepath.Join("assets", "skins", sanitizeDirName(skinName))
	if err := builder.GenerateSkin(skinConfig, outputDir, removeBGClient); err != nil {
		fmt.Printf("\n❌ 皮肤生成失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║                  ✅  皮肤制作完成！                   ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Printf("\n  皮肤目录：%s\n", outputDir)
	fmt.Println("  重启桌宠后，在设置页面 → 皮肤 中即可选择使用。")
	fmt.Println()
}

// promptImagePath 引导用户输入图片路径，验证文件存在且为支持的格式
func promptImagePath(scanner *bufio.Scanner) string {
	supportedFormats := []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}

	for {
		fmt.Println("📷 第一步：选择图片")
		fmt.Println("  请输入图片文件路径（支持 PNG / JPG / GIF / WEBP）：")
		fmt.Print("  > ")

		if !scanner.Scan() {
			fmt.Println("输入结束，退出。")
			os.Exit(0)
		}

		inputPath := strings.TrimSpace(scanner.Text())
		if inputPath == "" {
			fmt.Println("  ⚠️  路径不能为空，请重新输入。\n")
			continue
		}

		// 展开 ~ 路径
		if strings.HasPrefix(inputPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				inputPath = filepath.Join(homeDir, inputPath[2:])
			}
		}

		// 检查文件是否存在
		info, err := os.Stat(inputPath)
		if err != nil || info.IsDir() {
			fmt.Printf("  ⚠️  找不到文件：%s，请检查路径后重新输入。\n\n", inputPath)
			continue
		}

		// 检查文件格式
		ext := strings.ToLower(filepath.Ext(inputPath))
		isSupported := false
		for _, format := range supportedFormats {
			if ext == format {
				isSupported = true
				break
			}
		}
		if !isSupported {
			fmt.Printf("  ⚠️  不支持的图片格式 %q，请使用 PNG / JPG / GIF / WEBP。\n\n", ext)
			continue
		}

		fmt.Printf("  ✅ 已选择图片：%s\n\n", inputPath)
		return inputPath
	}
}

// promptSkinName 引导用户输入皮肤名称
func promptSkinName(scanner *bufio.Scanner) string {
	for {
		fmt.Println("🏷️  第二步：给皮肤起个名字")
		fmt.Println("  皮肤名称将显示在设置页面中（例如：我的猫咪、蔡徐坤、皮卡丘）：")
		fmt.Print("  > ")

		if !scanner.Scan() {
			os.Exit(0)
		}

		name := strings.TrimSpace(scanner.Text())
		if name == "" {
			fmt.Println("  ⚠️  名称不能为空，请重新输入。\n")
			continue
		}
		if len([]rune(name)) > 20 {
			fmt.Println("  ⚠️  名称过长（最多 20 个字符），请重新输入。\n")
			continue
		}

		fmt.Printf("  ✅ 皮肤名称：%s\n\n", name)
		return name
	}
}

// sanitizeDirName 将皮肤名称转换为安全的目录名（保留中文、字母、数字、下划线）
func sanitizeDirName(name string) string {
	var result strings.Builder
	for _, char := range name {
		if char == ' ' || char == '\t' {
			result.WriteRune('_')
		} else if char > 127 || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_' || char == '-' {
			result.WriteRune(char)
		}
	}
	dirName := result.String()
	if dirName == "" {
		dirName = "my_skin"
	}
	return dirName
}
