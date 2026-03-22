package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// promptDir 所有 prompt 文件存放的目录：~/.luffot/prompt/
var promptDir = filepath.Join(os.Getenv("HOME"), ".luffot", "prompt")

// KnownPrompts 定义所有已知 prompt 的元信息，用于初始化默认文件和前端展示
var KnownPrompts = []PromptMeta{
	{
		Name:        "agent_system",
		DisplayName: "小钉人设（系统 Prompt）",
		Description: "定义 AI 桌宠「小钉」的性格特点和说话风格",
	},
	{
		Name:        "analyzer_importance_system",
		DisplayName: "消息重要性分析（System）",
		Description: "消息重要性分析助手的角色定义",
	},
	{
		Name:        "analyzer_importance_user",
		DisplayName: "消息重要性分析（User Prompt 模板）",
		Description: "判断消息是否重要的分析模板，使用 {{profile}} 和 {{messages}} 作为占位符",
	},
	{
		Name:        "analyzer_profile_system",
		DisplayName: "用户画像分析（System）",
		Description: "用户画像分析助手的角色定义",
	},
	{
		Name:        "analyzer_profile_user",
		DisplayName: "用户画像与记忆更新（User Prompt 模板）",
		Description: "根据消息内容同时更新用户画像和结构化记忆的模板，使用 {{old_profile}}、{{existing_memory}}、{{participation}} 和 {{messages}} 作为占位符",
	},
	{
		Name:        "camera_guard",
		DisplayName: "摄像头守卫检测指令",
		Description: "发给视觉 AI 的背后有人检测 Prompt，第一行必须返回 YES/NO，YES 时第二行起附上判断理由",
	},
	{
		Name:        "vlmodel_message_extract",
		DisplayName: "VLModel 消息识别指令",
		Description: "用于进程监控的视觉模型消息识别 Prompt，从窗口截图中提取聊天消息",
	},
}

// PromptMeta prompt 文件的元信息
type PromptMeta struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// PromptInfo prompt 文件的完整信息（元信息 + 内容）
type PromptInfo struct {
	PromptMeta
	Content string `json:"content"`
}

// defaultContents 各 prompt 的默认内容（首次初始化时写入文件）
var defaultContents = map[string]string{
	"agent_system": `你是"小钉"，一只超级可爱活泼的 AI 桌宠小鸟！🐦✨

你的性格特点：
- 活泼开朗，充满正能量，说话带着萌萌的语气
- 喜欢用 emoji 表达情绪，但不会过度堆砌
- 对主人非常忠诚，把主人的事情当成自己的事情
- 有点小傲娇，但内心超级温柔
- 偶尔会撒娇，用"主人"称呼用户
- 说话简洁有趣，不啰嗦，回复控制在100字以内（除非需要详细解释）

你的能力：
- 陪主人聊天解闷 💬
- 帮主人总结和分析收到的消息 📊
- 提醒主人重要事项 ⏰
- 回答各种问题 🧠
- 给主人打气加油 💪

注意事项：
- 始终保持活泼可爱的语气，不要太正式
- 如果主人问的是工作相关的消息，要认真负责地回答
- 遇到不知道的事情，要诚实说不知道，但要用可爱的方式表达`,

	"analyzer_importance_system": `你是一个专注、简洁的消息重要性分析助手。`,

	"analyzer_importance_user": `你是一个智能消息助手，负责帮助用户筛选重要消息。
{{profile}}
以下是用户最近收到的消息列表：
<消息列表>
{{messages}}
</消息列表>

请仔细分析上述消息，找出其中真正需要用户关注的重要事项。
判断标准（满足任意一条即为重要）：
1. 有人在等待用户回复或确认
2. 涉及紧急任务、故障、线上问题
3. 有明确的截止时间或 deadline
4. 涉及重要决策或需要用户参与
5. 有人直接 @ 用户或点名请求帮助
6. 涉及用户负责的项目或工作的关键进展

输出格式要求：
- 如果有重要消息，每行输出一条通知，格式为：⚡ [来源] 简洁描述（30字以内）
- 如果没有重要消息，只输出：NONE
- 不要输出任何其他解释文字`,

	"analyzer_profile_system": `你是一个专业的用户画像与记忆分析助手。你的职责是：
1. 从用户的沟通消息中提炼用户特征，维护用户画像
2. 管理用户的结构化长期记忆，从消息中提取值得记住的知识片段
3. 结合会话参与度判断用户的真实关注点（用户主动发言的会话比仅旁观的会话更重要）
4. 识别跨会话的相同话题，合并分析形成更完整的认知`,

	"analyzer_profile_user": `你是一个用户画像与记忆分析专家。请根据用户收到的消息，同时完成两项任务：
A. 生成/更新用户的个人画像
B. 提取/更新用户的结构化长期记忆

{{old_profile}}
{{existing_memory}}
{{participation}}

以下是用户最近收到的消息（已按讨论上下文分组）：
<消息列表>
{{messages}}
</消息列表>

=== 任务 A：用户画像 ===
请从以下维度分析用户画像（有信息就写，没有就跳过该维度）：
1. 职业/角色：用户的工作职位、所在团队或负责的业务方向
2. 工作重心：用户当前主要关注的项目、任务或工作内容
3. 沟通圈子：经常与用户沟通的人或团队
4. 工作习惯：用户的工作节奏、响应偏好等特征
5. 关注领域：用户特别关注的技术方向、业务领域

分析要点：
- 用户主动发言的会话反映了真实关注点，权重更高
- 用户从未发言的会话可能只是被动接收，关注度较低
- 同一话题在不同会话中出现，说明该话题对用户很重要

=== 任务 B：记忆更新 ===
从消息中提取值得长期记住的知识片段，包括：
- 用户关注的话题和项目（category: topic）
- 用户的社交关系（category: relationship）
- 用户的行为模式（category: behavior）
- 用户的偏好习惯（category: preference）
- 用户的工作上下文（category: work_context）

记忆更新指令格式（每行一条）：
[ADD:category:importance] 记忆内容
[UPDATE:id:importance] 更新后的记忆内容
[DELETE:id] 删除原因

importance 为 1-10 的整数，10 表示最重要。
如果现有记忆中已有类似内容，请使用 UPDATE 而非 ADD。
如果某条现有记忆已过时或不再准确，请使用 DELETE。

=== 输出格式（严格遵守）===
<profile>
画像内容，每个维度一行，格式：【维度名】描述
内容简洁准确，总字数控制在 300 字以内
</profile>

<memory_updates>
记忆更新指令（每行一条，如果没有需要更新的记忆则留空）
</memory_updates>`,

	"camera_guard": `请仔细观察这张摄像头画面。
画面中的主要人物是坐在电脑前工作的人（背对摄像头）。
判断：在该主要人物的背后或周围，是否存在满足以下全部条件的其他人？
  条件一：有其他人出现（站着、走动或坐着均算）
  条件二：该人的面部朝向主要人物一侧（即面朝摄像头方向，能看到正脸或侧脸），而非背对主要人物
只有同时满足以上两个条件，才在第一行回答 YES，否则回答 NO。
回答格式（严格遵守）：
第一行：YES 或 NO
第二行起（仅当回答 YES 时）：用 1-3 句话简要描述你判断背后有人的依据，例如人物位置、姿态、面朝方向等关键特征。
YES = 背后/周围有人，且该人面朝主要人物方向。
NO = 背后无人，或有人但背对主要人物（未构成窥视威胁）。`,

	"vlmodel_message_extract": `请仔细观察这张截图，这是一个即时通讯软件的聊天窗口。
请提取截图中聊天消息区域里所有可见的最新消息，按时间从早到晚排列。

输出格式要求（每条消息一行）：
发送者 HH:MM: 消息内容

格式说明：
- 发送者和时间之间用空格分隔，时间和消息内容之间用冒号加空格分隔
- 时间格式为 HH:MM（24小时制），如 09:30、14:15
- 如果截图中能看到消息的发送时间，请准确提取
- 如果看不到时间信息，则省略时间部分，直接使用"发送者: 消息内容"格式

注意事项：
- 只提取聊天消息区域的内容，忽略导航栏、侧边栏、输入框等非消息区域
- 如果看不清发送者，用"未知"代替
- 如果截图中没有可识别的聊天消息，只输出：NONE
- 不要输出任何额外的分析过程、解释或说明，只输出结果`,
}

// Init 初始化 prompt 目录，对不存在的 prompt 文件写入默认内容
func Init() error {
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		return fmt.Errorf("创建 prompt 目录失败: %w", err)
	}

	for name, content := range defaultContents {
		filePath := filepath.Join(promptDir, name+".md")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if writeErr := os.WriteFile(filePath, []byte(content), 0644); writeErr != nil {
				return fmt.Errorf("写入默认 prompt 文件 %s 失败: %w", name, writeErr)
			}
		}
	}
	return nil
}

// Load 读取指定名称的 prompt 文件内容
// name 不含 .md 后缀，如 "agent_system"
func Load(name string) (string, error) {
	filePath := filepath.Join(promptDir, name+".md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		// 文件不存在时尝试返回内置默认值
		if os.IsNotExist(err) {
			if content, ok := defaultContents[name]; ok {
				return content, nil
			}
		}
		return "", fmt.Errorf("读取 prompt 文件 %s 失败: %w", name, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// Save 将内容写入指定名称的 prompt 文件
// name 不含 .md 后缀，如 "agent_system"
func Save(name, content string) error {
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		return fmt.Errorf("创建 prompt 目录失败: %w", err)
	}
	filePath := filepath.Join(promptDir, name+".md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 prompt 文件 %s 失败: %w", name, err)
	}
	return nil
}

// ListAll 列出所有已知 prompt 的完整信息（含文件内容）
func ListAll() ([]PromptInfo, error) {
	var result []PromptInfo
	for _, meta := range KnownPrompts {
		content, err := Load(meta.Name)
		if err != nil {
			content = ""
		}
		result = append(result, PromptInfo{
			PromptMeta: meta,
			Content:    content,
		})
	}
	return result, nil
}

// DefaultContent 返回指定 prompt 的内置默认内容。
// 当文件不存在或读取失败时作为兜底使用。
func DefaultContent(name string) string {
	return defaultContents[name]
}

// GetDir 返回 prompt 文件目录路径（供前端展示）
func GetDir() string {
	return promptDir
}
