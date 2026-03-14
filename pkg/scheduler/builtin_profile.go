package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luffot/luffot/pkg/ai"
	"github.com/luffot/luffot/pkg/storage"
)

var (
	// luffotDir ~/.luffot 根目录
	luffotDir = filepath.Join(os.Getenv("HOME"), ".luffot")

	// profileCacheFile 画像文件路径：~/.luffot/.my_profile
	profileCacheFile = filepath.Join(luffotDir, ".my_profile")

	// progressCacheFile 进度缓存文件路径：~/.luffot/cache/profile_progress.json
	progressCacheFile = filepath.Join(luffotDir, "cache", "profile_progress.json")
)

// profileBatchTimeoutSeconds 单批画像生成的超时时间（秒）
const profileBatchTimeoutSeconds = 60

// hourKeyLayout 小时 key 的时间格式
const hourKeyLayout = "2006-01-02 15"

// profileProgress 进度缓存结构，持久化到 profile_progress.json
type profileProgress struct {
	// LastProcessedHour 上次已处理完成的最后一个小时，格式 "2006-01-02 15"
	// 空字符串表示从未处理过，将从一周前开始
	LastProcessedHour string `json:"last_processed_hour"`
	// UpdatedAt 上次更新时间（RFC3339）
	UpdatedAt string `json:"updated_at"`
}

// RunDailyProfileReport 内置任务：每小时增量更新用户画像。
//
// 执行策略：
//  1. 读取进度缓存（~/.luffot/cache/profile_progress.json），获取上次处理到的小时
//  2. 若从未处理过，从一周前的整点小时开始
//  3. 从下一个待处理小时起，逐小时读取该小时的消息，结合已有画像调用 LLM 更新
//  4. 每处理完一个小时，立即将进度和画像写入磁盘（断点续传）
//  5. 处理到当前小时为止（不处理未来时间）
func RunDailyProfileReport(ctx context.Context, agent *ai.Agent, st *storage.Storage, providerName string) error {
	if agent == nil || !agent.IsEnabled() {
		return fmt.Errorf("AI 未启用，无法生成画像报告")
	}

	log.Println("[ProfileReport] 开始增量更新用户画像...")

	// 读取进度缓存
	progress, err := loadProgress()
	if err != nil {
		log.Printf("[ProfileReport] 读取进度缓存失败，将从头开始: %v", err)
		progress = &profileProgress{}
	}

	// 确定起始小时：上次处理的下一小时；若从未处理则从一周前开始
	var startHour time.Time
	if progress.LastProcessedHour == "" {
		startHour = time.Now().AddDate(0, 0, -7).Truncate(time.Hour)
		log.Printf("[ProfileReport] 首次运行，从一周前开始: %s", startHour.Format(hourKeyLayout))
	} else {
		lastHour, parseErr := time.ParseInLocation(hourKeyLayout, progress.LastProcessedHour, time.Local)
		if parseErr != nil {
			log.Printf("[ProfileReport] 进度时间解析失败，从一周前重新开始: %v", parseErr)
			startHour = time.Now().AddDate(0, 0, -7).Truncate(time.Hour)
		} else {
			startHour = lastHour.Add(time.Hour)
		}
	}

	// 当前小时（不处理未来）
	currentHour := time.Now().Truncate(time.Hour)

	if !startHour.Before(currentHour) && startHour != currentHour {
		log.Println("[ProfileReport] 画像已是最新，无需处理")
		return nil
	}

	// 读取当前画像作为初始上下文
	currentProfile := loadProfileFromFile()
	if currentProfile != "" {
		log.Printf("[ProfileReport] 已加载现有画像（%d 字）", len([]rune(currentProfile)))
	}

	// 逐小时处理，直到当前小时
	processedCount := 0
	for hour := startHour; !hour.After(currentHour); hour = hour.Add(time.Hour) {
		hourKey := hour.Format(hourKeyLayout)

		// 读取该小时的消息
		hourEnd := hour.Add(time.Hour)
		messages, fetchErr := st.GetMessagesByTimeRange("", hour, hourEnd)
		if fetchErr != nil {
			log.Printf("[ProfileReport] 读取 %s 消息失败，跳过: %v", hourKey, fetchErr)
			// 即使读取失败也推进进度，避免卡死
			if saveErr := saveProgress(hourKey); saveErr != nil {
				log.Printf("[ProfileReport] 保存进度失败: %v", saveErr)
			}
			continue
		}

		if len(messages) == 0 {
			// 该小时无消息，直接推进进度，不调用 LLM
			if saveErr := saveProgress(hourKey); saveErr != nil {
				log.Printf("[ProfileReport] 保存进度失败: %v", saveErr)
			}
			continue
		}

		log.Printf("[ProfileReport] 处理 %s，消息数=%d", hourKey, len(messages))

		updatedProfile, batchErr := generateProfileForBatch(ctx, agent, providerName, messages, currentProfile, hourKey)
		if batchErr != nil {
			log.Printf("[ProfileReport] %s 生成失败，跳过（保留上一批画像）: %v", hourKey, batchErr)
			// 失败时也推进进度，避免反复卡在同一小时
			if saveErr := saveProgress(hourKey); saveErr != nil {
				log.Printf("[ProfileReport] 保存进度失败: %v", saveErr)
			}
			continue
		}

		currentProfile = updatedProfile
		processedCount++

		// 立即持久化画像和进度（断点续传）
		if saveErr := saveProfileToFile(currentProfile); saveErr != nil {
			log.Printf("[ProfileReport] 保存画像失败: %v", saveErr)
		}
		if saveErr := saveProgress(hourKey); saveErr != nil {
			log.Printf("[ProfileReport] 保存进度失败: %v", saveErr)
		}

		log.Printf("[ProfileReport] %s 完成，画像字数=%d", hourKey, len([]rune(currentProfile)))
	}

	if processedCount == 0 {
		log.Println("[ProfileReport] 本次无新消息需要处理")
		return nil
	}

	// 同步更新数据库中的个人画像（供 IntelliAnalyzer 使用）
	if saveErr := st.SaveUserProfile(currentProfile); saveErr != nil {
		log.Printf("[ProfileReport] 更新数据库画像失败（不影响文件）: %v", saveErr)
	}

	log.Printf("[ProfileReport] 增量更新完成，共处理 %d 个小时批次，最终画像字数=%d",
		processedCount, len([]rune(currentProfile)))
	return nil
}

// generateProfileForBatch 针对单个小时批次调用 LLM 更新画像。
// previousProfile 为上一批生成的画像（首批可为空）。
func generateProfileForBatch(
	ctx context.Context,
	agent *ai.Agent,
	providerName string,
	messages []*storage.Message,
	previousProfile string,
	hourLabel string,
) (string, error) {
	var msgLines []string
	for _, msg := range messages {
		line := fmt.Sprintf("[%s][%s] %s: %s",
			msg.Timestamp.Format("01-02 15:04"),
			msg.Session,
			msg.Sender,
			msg.Content,
		)
		msgLines = append(msgLines, line)
	}
	batchMessagesText := strings.Join(msgLines, "\n")

	previousProfileSection := ""
	if previousProfile != "" {
		previousProfileSection = fmt.Sprintf(`
以下是根据之前消息已生成的用户画像，请在此基础上结合新消息进行更新和完善：
<已有画像>
%s
</已有画像>
`, previousProfile)
	}

	prompt := fmt.Sprintf(`你是一个专业的用户画像分析师。请根据用户在 %s 时段收到的消息，更新用户画像。
%s
以下是该时段的消息记录：
<消息记录>
%s
</消息记录>

请从以下维度分析并描述用户画像（有信息就写，没有就跳过该维度）：
1. 职业与角色：工作职位、所在团队、负责的业务方向
2. 工作重心：当前主要关注的项目、任务或工作内容
3. 沟通圈子：经常与用户沟通的人或团队
4. 工作习惯：工作节奏、响应偏好、沟通风格等特征
5. 关注领域：特别关注的技术方向或业务领域

输出要求：
- 直接输出画像内容，使用自然语言段落描述，不要使用列表或标题
- 语言简洁客观，总字数严格控制在 500 字以内
- 不要输出任何前言、解释或总结语`, hourLabel, previousProfileSection, batchMessagesText)

	chatMessages := []ai.ChatMessage{
		{Role: "system", Content: "你是一个专业的用户画像分析助手，善于从沟通内容中提炼用户特征，输出简洁准确的画像描述。"},
		{Role: "user", Content: prompt},
	}

	reqCtx, cancel := context.WithTimeout(ctx, profileBatchTimeoutSeconds*time.Second)
	defer cancel()

	result, err := agent.ChatSync(reqCtx, chatMessages, providerName)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	result = strings.TrimSpace(result)
	if result == "" {
		return "", fmt.Errorf("LLM 返回空内容")
	}

	return result, nil
}

// loadProgress 从磁盘读取进度缓存，文件不存在时返回空进度
func loadProgress() (*profileProgress, error) {
	data, err := os.ReadFile(progressCacheFile)
	if os.IsNotExist(err) {
		return &profileProgress{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取进度文件失败: %w", err)
	}

	var progress profileProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("解析进度文件失败: %w", err)
	}
	return &progress, nil
}

// saveProgress 将当前处理到的小时 key 写入进度缓存文件
func saveProgress(lastProcessedHour string) error {
	cacheDir := filepath.Dir(progressCacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}

	progress := profileProgress{
		LastProcessedHour: lastProcessedHour,
		UpdatedAt:         time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化进度失败: %w", err)
	}

	if err := os.WriteFile(progressCacheFile, data, 0644); err != nil {
		return fmt.Errorf("写入进度文件失败: %w", err)
	}
	return nil
}

// loadProfileFromFile 从 ~/.luffot/.my_profile 读取画像文本，不存在时返回空字符串
func loadProfileFromFile() string {
	data, err := os.ReadFile(profileCacheFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// saveProfileToFile 将画像文本写入 ~/.luffot/.my_profile（纯文本，覆盖写）
func saveProfileToFile(profileText string) error {
	if err := os.MkdirAll(luffotDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(profileCacheFile, []byte(profileText+"\n"), 0644); err != nil {
		return fmt.Errorf("写入画像文件失败: %w", err)
	}
	log.Printf("[ProfileReport] 画像已写入: %s", profileCacheFile)
	return nil
}

// LoadUserProfile 供外部包读取当前用户画像（agent 注入 system prompt 使用）
func LoadUserProfile() string {
	return loadProfileFromFile()
}
