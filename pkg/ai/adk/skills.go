package adk

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"
)

// SkillRegistry 技能注册中心
type SkillRegistry struct {
	skills map[string]Skill
	mu     sync.RWMutex
}

// NewSkillRegistry 创建技能注册中心
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]Skill),
	}
}

// Register 注册技能
func (r *SkillRegistry) Register(skill Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name()] = skill
}

// Get 获取技能
func (r *SkillRegistry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[name]
	return skill, ok
}

// List 列出所有技能
func (r *SkillRegistry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		result = append(result, skill)
	}
	return result
}

// Unregister 注销技能
func (r *SkillRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, name)
}

// SkillConfig 技能配置
type SkillConfig struct {
	Name        string                 `yaml:"name" json:"name"`
	Version     string                 `yaml:"version" json:"version"`
	Description string                 `yaml:"description" json:"description"`
	Type        string                 `yaml:"type" json:"type"` // builtin, http, python, lua
	Entry       string                 `yaml:"entry" json:"entry"`
	Config      map[string]interface{} `yaml:"config" json:"config"`
	Parameters  []ParameterDef         `yaml:"parameters" json:"parameters"`
	Returns     []ParameterDef         `yaml:"returns" json:"returns"`
}

// ParameterDef 参数定义
type ParameterDef struct {
	Name        string      `yaml:"name" json:"name"`
	Type        string      `yaml:"type" json:"type"`
	Description string      `yaml:"description" json:"description"`
	Required    bool        `yaml:"required" json:"required"`
	Default     interface{} `yaml:"default,omitempty" json:"default,omitempty"`
}

// SkillManager 技能管理器
type SkillManager struct {
	registry  *SkillRegistry
	skillsDir string
	configs   map[string]*SkillConfig
	mu        sync.RWMutex
}

// NewSkillManager 创建技能管理器
func NewSkillManager(skillsDir string) *SkillManager {
	return &SkillManager{
		registry:  NewSkillRegistry(),
		skillsDir: skillsDir,
		configs:   make(map[string]*SkillConfig),
	}
}

// InstallSkill 安装技能
func (m *SkillManager) InstallSkill(source string) error {
	// 判断来源类型
	if isURL(source) {
		return m.installFromURL(source)
	}
	return m.installFromLocal(source)
}

// installFromURL 从URL安装
func (m *SkillManager) installFromURL(url string) error {
	// 下载技能包
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download skill: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download skill: status %d", resp.StatusCode)
	}

	// 保存到临时文件
	tmpFile, err := os.CreateTemp("", "skill_*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save skill: %w", err)
	}
	tmpFile.Close()

	// 解压安装
	return m.installFromArchive(tmpFile.Name())
}

// installFromLocal 从本地安装
func (m *SkillManager) installFromLocal(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat skill path: %w", err)
	}

	if info.IsDir() {
		return m.installFromDir(path)
	}

	// 检查是否是压缩包
	ext := filepath.Ext(path)
	if ext == ".zip" || ext == ".tar.gz" {
		return m.installFromArchive(path)
	}

	return fmt.Errorf("unsupported skill format: %s", ext)
}

// installFromDir 从目录安装
func (m *SkillManager) installFromDir(dir string) error {
	// 读取skill.yaml
	configPath := filepath.Join(dir, "skill.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = filepath.Join(dir, "skill.yml")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read skill config: %w", err)
	}

	var config SkillConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse skill config: %w", err)
	}

	// 验证配置
	if err := m.validateConfig(&config); err != nil {
		return fmt.Errorf("invalid skill config: %w", err)
	}

	// 安装到skills目录
	targetDir := filepath.Join(m.skillsDir, config.Name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// 复制文件
	if err := copyDir(dir, targetDir); err != nil {
		return fmt.Errorf("failed to copy skill files: %w", err)
	}

	// 加载技能
	skill, err := m.loadSkill(&config, targetDir)
	if err != nil {
		return fmt.Errorf("failed to load skill: %w", err)
	}

	m.registry.Register(skill)
	m.setConfig(config.Name, &config)

	return nil
}

// installFromArchive 从压缩包安装
func (m *SkillManager) installFromArchive(archivePath string) error {
	// 创建临时解压目录
	tmpDir, err := os.MkdirTemp("", "skill_extract_*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 根据文件类型解压
	if strings.HasSuffix(archivePath, ".zip") {
		if err := unzip(archivePath, tmpDir); err != nil {
			return fmt.Errorf("failed to unzip: %w", err)
		}
	} else if strings.HasSuffix(archivePath, ".tar.gz") {
		if err := untar(archivePath, tmpDir); err != nil {
			return fmt.Errorf("failed to untar: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported archive format: %s", archivePath)
	}

	// 查找解压后的目录
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to read extracted directory: %w", err)
	}

	var skillDir string
	for _, entry := range entries {
		if entry.IsDir() {
			skillDir = filepath.Join(tmpDir, entry.Name())
			break
		}
	}

	if skillDir == "" {
		// 如果没有子目录，使用tmpDir本身
		skillDir = tmpDir
	}

	// 从解压目录安装
	return m.installFromDir(skillDir)
}

// unzip 解压zip文件
func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dst, f.Name)

		// 防止zip slip攻击
		if !strings.HasPrefix(fpath, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// untar 解压tar.gz文件
func untar(src, dst string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dst, header.Name)

		// 防止tar slip攻击
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

// UninstallSkill 卸载技能
func (m *SkillManager) UninstallSkill(name string) error {
	// 从注册表移除
	m.registry.Unregister(name)

	// 删除文件
	skillDir := filepath.Join(m.skillsDir, name)
	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("failed to remove skill directory: %w", err)
	}

	m.deleteConfig(name)

	return nil
}

// LoadInstalledSkills 加载所有已安装的技能
func (m *SkillManager) LoadInstalledSkills() error {
	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		configPath := filepath.Join(m.skillsDir, entry.Name(), "skill.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = filepath.Join(m.skillsDir, entry.Name(), "skill.yml")
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			continue // 跳过无效的技能目录
		}

		var config SkillConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			continue
		}

		skillDir := filepath.Join(m.skillsDir, entry.Name())
		skill, err := m.loadSkill(&config, skillDir)
		if err != nil {
			continue
		}

		m.registry.Register(skill)
		m.setConfig(config.Name, &config)
	}

	return nil
}

// loadSkill 加载技能实例
func (m *SkillManager) loadSkill(config *SkillConfig, dir string) (Skill, error) {
	switch config.Type {
	case "builtin":
		return m.loadBuiltinSkill(config, dir)
	case "http":
		return m.loadHTTPSkill(config)
	case "python":
		return m.loadPythonSkill(config, dir)
	case "lua":
		return m.loadLuaSkill(config, dir)
	default:
		return nil, fmt.Errorf("unsupported skill type: %s", config.Type)
	}
}

// loadBuiltinSkill 加载内置技能
func (m *SkillManager) loadBuiltinSkill(config *SkillConfig, dir string) (Skill, error) {
	return &builtinSkill{
		config: config,
	}, nil
}

// loadHTTPSkill 加载HTTP技能
func (m *SkillManager) loadHTTPSkill(config *SkillConfig) (Skill, error) {
	return &httpSkill{
		config: config,
	}, nil
}

// loadPythonSkill 加载Python技能
func (m *SkillManager) loadPythonSkill(config *SkillConfig, dir string) (Skill, error) {
	return &pythonSkill{
		config: config,
		dir:    dir,
	}, nil
}

// loadLuaSkill 加载Lua技能
func (m *SkillManager) loadLuaSkill(config *SkillConfig, dir string) (Skill, error) {
	return &luaSkill{
		config: config,
		dir:    dir,
	}, nil
}

// validateConfig 验证配置
func (m *SkillManager) validateConfig(config *SkillConfig) error {
	if config.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if config.Version == "" {
		return fmt.Errorf("skill version is required")
	}
	if config.Type == "" {
		return fmt.Errorf("skill type is required")
	}
	return nil
}

// setConfig 设置配置
func (m *SkillManager) setConfig(name string, config *SkillConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[name] = config
}

// deleteConfig 删除配置
func (m *SkillManager) deleteConfig(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.configs, name)
}

// GetSkillConfig 获取技能配置
func (m *SkillManager) GetSkillConfig(name string) (*SkillConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	config, ok := m.configs[name]
	return config, ok
}

// ListInstalledSkills 列出已安装的技能
func (m *SkillManager) ListInstalledSkills() []SkillConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SkillConfig, 0, len(m.configs))
	for _, config := range m.configs {
		result = append(result, *config)
	}
	return result
}

// GetRegistry 获取技能注册表
func (m *SkillManager) GetRegistry() *SkillRegistry {
	return m.registry
}

// 辅助函数

func isURL(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || s[:8] == "https://")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// 内置技能实现

type builtinSkill struct {
	config *SkillConfig
	fn     func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

func (s *builtinSkill) Name() string {
	return s.config.Name
}

func (s *builtinSkill) Description() string {
	return s.config.Description
}

func (s *builtinSkill) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if s.fn != nil {
		return s.fn(ctx, input)
	}
	return nil, fmt.Errorf("builtin skill %s not implemented", s.config.Name)
}

// HTTP技能实现

type httpSkill struct {
	config *SkillConfig
}

func (s *httpSkill) Name() string {
	return s.config.Name
}

func (s *httpSkill) Description() string {
	return s.config.Description
}

func (s *httpSkill) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	url, ok := s.config.Config["url"].(string)
	if !ok {
		return nil, fmt.Errorf("http skill missing url config")
	}

	method, _ := s.config.Config["method"].(string)
	if method == "" {
		method = "POST"
	}

	// 构建请求
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	// 请求体已在NewRequestWithContext中设置

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]interface{}{"response": string(body)}, nil
	}

	return result, nil
}

// Python技能实现

type pythonSkill struct {
	config *SkillConfig
	dir    string
}

func (s *pythonSkill) Name() string {
	return s.config.Name
}

func (s *pythonSkill) Description() string {
	return s.config.Description
}

func (s *pythonSkill) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// 构建Python脚本路径
	scriptPath := filepath.Join(s.dir, s.config.Entry)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("python script not found: %s", scriptPath)
	}

	// 将输入转为JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// 执行Python脚本
	cmd := exec.CommandContext(ctx, "python3", scriptPath)
	cmd.Dir = s.dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("SKILL_INPUT=%s", string(inputJSON)))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("python execution failed: %w, output: %s", err, string(output))
	}

	// 解析输出
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return map[string]interface{}{"output": string(output)}, nil
	}

	return result, nil
}

// Lua技能实现

type luaSkill struct {
	config *SkillConfig
	dir    string
}

func (s *luaSkill) Name() string {
	return s.config.Name
}

func (s *luaSkill) Description() string {
	return s.config.Description
}

func (s *luaSkill) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// 创建Lua状态机
	L := lua.NewState()
	defer L.Close()

	// 加载Lua脚本
	scriptPath := filepath.Join(s.dir, s.config.Entry)
	if err := L.DoFile(scriptPath); err != nil {
		return nil, fmt.Errorf("failed to load lua script: %w", err)
	}

	// 将输入转为Lua表
	luaInput := s.mapToLuaTable(L, input)
	L.SetGlobal("input", luaInput)

	// 调用执行函数
	if err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal("execute"),
		NRet:    1,
		Protect: true,
	}, luaInput); err != nil {
		return nil, fmt.Errorf("lua execution failed: %w", err)
	}

	// 获取返回值
	ret := L.Get(-1)
	L.Pop(1)

	// 将Lua表转为Go map
	result := s.luaValueToGo(ret)

	if resultMap, ok := result.(map[string]interface{}); ok {
		return resultMap, nil
	}

	return map[string]interface{}{"result": result}, nil
}

// mapToLuaTable 将Go map转为Lua表
func (s *luaSkill) mapToLuaTable(L *lua.LState, m map[string]interface{}) *lua.LTable {
	table := L.NewTable()
	for k, v := range m {
		L.SetField(table, k, s.goValueToLua(L, v))
	}
	return table
}

// goValueToLua 将Go值转为Lua值
func (s *luaSkill) goValueToLua(L *lua.LState, v interface{}) lua.LValue {
	switch val := v.(type) {
	case string:
		return lua.LString(val)
	case float64:
		return lua.LNumber(val)
	case int:
		return lua.LNumber(val)
	case bool:
		return lua.LBool(val)
	case map[string]interface{}:
		return s.mapToLuaTable(L, val)
	case []interface{}:
		table := L.NewTable()
		for i, item := range val {
			L.SetTable(table, lua.LNumber(i+1), s.goValueToLua(L, item))
		}
		return table
	case nil:
		return lua.LNil
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

// luaValueToGo 将Lua值转为Go值
func (s *luaSkill) luaValueToGo(v lua.LValue) interface{} {
	switch val := v.(type) {
	case *lua.LTable:
		result := make(map[string]interface{})
		val.ForEach(func(key, value lua.LValue) {
			if k, ok := key.(lua.LString); ok {
				result[string(k)] = s.luaValueToGo(value)
			}
		})
		return result
	case lua.LString:
		return string(val)
	case lua.LNumber:
		return float64(val)
	case lua.LBool:
		return bool(val)
	case *lua.LNilType:
		return nil
	default:
		return val.String()
	}
}
