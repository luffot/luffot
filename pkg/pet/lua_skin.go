package pet

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	lua "github.com/yuin/gopher-lua"
)

// LuaSkin 表示一个 Lua 皮肤配置
type LuaSkin struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Version  string `json:"version"`
}

// LuaSkinManager 管理 Lua 皮肤的加载和渲染
type LuaSkinManager struct {
	mu sync.RWMutex

	// 已注册的 Lua 皮肤
	skins map[string]*LuaSkin

	// 当前激活的皮肤
	activeSkin *LuaSkin

	// Lua 虚拟机
	luaState *lua.LState

	// 运行状态
	isRunning bool

	// 宠物当前位置
	posX, posY float64

	// 当前状态
	currentState PetState

	// 当前皮肤实例（Lua 表）
	skinInstance lua.LValue

	// 绘制命令缓冲区
	drawCommands []DrawCommand

	// 当前绘制颜色
	currentColor color.RGBA
}

// DrawCommand 表示一个绘制命令
type DrawCommand struct {
	Type   string
	X, Y   float64
	Radius float64
	Width  float64
	Height float64
	Color  color.RGBA
}

// 全局 Lua 皮肤管理器实例
var luaSkinManager *LuaSkinManager

// 全局 Lua 皮肤注册表
var luaSkins = map[string]*LuaSkin{}
var luaSkinNames []string

// LuaSkinBasePath Lua 皮肤根目录
const LuaSkinBasePath = "pkg/embedfs/static/lua/skin"

// GetLuaSkinManager 获取或创建 Lua 皮肤管理器单例
func GetLuaSkinManager() *LuaSkinManager {
	if luaSkinManager == nil {
		luaSkinManager = &LuaSkinManager{
			skins:        make(map[string]*LuaSkin),
			posX:         80,
			posY:         80,
			currentState: StateIdle,
			drawCommands: make([]DrawCommand, 0, 100),
		}
		// 初始化时自动加载 Lua 皮肤
		AutoLoadLuaSkins()
	}
	return luaSkinManager
}

// RegisterLuaSkin 注册一个 Lua 皮肤
func RegisterLuaSkin(name, filePath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("Lua skin file not found: %s", filePath)
	}

	skin := &LuaSkin{
		Name:     name,
		FilePath: filePath,
		Version:  "1.0.0",
	}

	luaSkins[name] = skin
	// 检查是否已存在
	exists := false
	for _, n := range luaSkinNames {
		if n == name {
			exists = true
			break
		}
	}
	if !exists {
		luaSkinNames = append(luaSkinNames, name)
	}

	return nil
}

// AutoLoadLuaSkins 自动扫描并加载所有 Lua 皮肤
func AutoLoadLuaSkins() error {
	entries, err := os.ReadDir(LuaSkinBasePath)
	if err != nil {
		// 目录不存在时静默跳过
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// 只加载 .lua 文件
		if filepath.Ext(name) != ".lua" {
			continue
		}

		// 去掉 .lua 后缀作为皮肤名称
		skinName := name[:len(name)-4]
		filePath := filepath.Join(LuaSkinBasePath, name)

		if err := RegisterLuaSkin(skinName, filePath); err != nil {
			fmt.Printf("Failed to register Lua skin %s: %v\n", skinName, err)
			continue
		}

		fmt.Printf("Registered Lua skin: %s\n", skinName)
	}

	return nil
}

// GetLuaSkinNames 获取所有已注册的 Lua 皮肤名称
func GetLuaSkinNames() []string {
	return luaSkinNames
}

// IsLuaSkinActive 返回当前是否使用 Lua 皮肤
func IsLuaSkinActive() bool {
	manager := GetLuaSkinManager()
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.activeSkin != nil && manager.isRunning
}

// GetActiveLuaSkinName 获取当前激活的 Lua 皮肤名称
func GetActiveLuaSkinName() string {
	manager := GetLuaSkinManager()
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if manager.activeSkin == nil {
		return ""
	}
	return manager.activeSkin.Name
}

// SetActiveLuaSkin 激活指定的 Lua 皮肤
func SetActiveLuaSkin(name string) bool {
	manager := GetLuaSkinManager()

	skin, ok := luaSkins[name]
	if !ok {
		return false
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	// 如果已经有运行的皮肤，先停止
	if manager.isRunning {
		manager.stopLocked()
	}

	manager.activeSkin = skin

	// 启动 Lua 虚拟机
	if err := manager.startLocked(); err != nil {
		fmt.Printf("Failed to start Lua skin %s: %v\n", name, err)
		manager.activeSkin = nil
		return false
	}

	return true
}

// ClearActiveLuaSkin 清除当前 Lua 皮肤，恢复其他渲染模式
func ClearActiveLuaSkin() {
	manager := GetLuaSkinManager()
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if manager.isRunning {
		manager.stopLocked()
	}
	manager.activeSkin = nil
}

// SetLuaPetState 设置 Lua 宠物的状态
func SetLuaPetState(state PetState) bool {
	manager := GetLuaSkinManager()
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if !manager.isRunning || manager.skinInstance == nil {
		return false
	}

	stateName := petStateToString(state)
	manager.currentState = state

	// 调用 Lua 的 setState 方法
	L := manager.luaState
	setStateFn := L.GetField(manager.skinInstance, "setState")
	if setStateFn.Type() == lua.LTFunction {
		L.Push(setStateFn)
		L.Push(manager.skinInstance)
		L.Push(lua.LString(stateName))
		L.Call(2, 0)
	}

	return true
}

// SetLuaPetPosition 设置 Lua 宠物的位置
func SetLuaPetPosition(x, y float64) bool {
	manager := GetLuaSkinManager()
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if !manager.isRunning || manager.skinInstance == nil {
		return false
	}

	manager.posX = x
	manager.posY = y

	// 调用 Lua 的 setPosition 方法
	L := manager.luaState
	setPosFn := L.GetField(manager.skinInstance, "setPosition")
	if setPosFn.Type() == lua.LTFunction {
		L.Push(setPosFn)
		L.Push(manager.skinInstance)
		L.Push(lua.LNumber(x))
		L.Push(lua.LNumber(y))
		L.Call(3, 0)
	}

	return true
}

// Update 更新 Lua 皮肤动画
func UpdateLuaSkin() {
	manager := GetLuaSkinManager()
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if !manager.isRunning || manager.skinInstance == nil || manager.luaState == nil {
		return
	}

	L := manager.luaState

	// 调用 Lua 的 update 方法
	updateFn := L.GetField(manager.skinInstance, "update")
	if updateFn.Type() == lua.LTFunction {
		L.Push(updateFn)
		L.Push(manager.skinInstance)
		L.Push(lua.LNumber(1.0 / 60.0)) // dt = 1/60 秒
		L.Call(2, 0)
	}
}

// Draw 绘制 Lua 皮肤
func DrawLuaSkin(screen *ebiten.Image, x, y float64) {
	manager := GetLuaSkinManager()
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if !manager.isRunning || manager.skinInstance == nil || manager.luaState == nil {
		return
	}

	L := manager.luaState

	// 更新位置
	manager.posX = x
	manager.posY = y

	// 调用 Lua 的 setPosition 方法
	setPosFn := L.GetField(manager.skinInstance, "setPosition")
	if setPosFn.Type() == lua.LTFunction {
		L.Push(setPosFn)
		L.Push(manager.skinInstance)
		L.Push(lua.LNumber(x))
		L.Push(lua.LNumber(y))
		L.Call(3, 0)
	}

	// 清空绘制命令缓冲区
	manager.drawCommands = manager.drawCommands[:0]

	// 调用 Lua 的 draw 方法
	drawFn := L.GetField(manager.skinInstance, "draw")
	if drawFn.Type() == lua.LTFunction {
		L.Push(drawFn)
		L.Push(manager.skinInstance)
		L.Call(1, 0)
	}

	// 执行绘制命令
	for _, cmd := range manager.drawCommands {
		switch cmd.Type {
		case "circle":
			vector.DrawFilledCircle(screen, float32(cmd.X), float32(cmd.Y), float32(cmd.Radius), cmd.Color, false)
		case "rect":
			vector.DrawFilledRect(screen, float32(cmd.X), float32(cmd.Y), float32(cmd.Width), float32(cmd.Height), cmd.Color, false)
		case "line":
			// 使用矩形模拟线条
			vector.DrawFilledRect(screen, float32(cmd.X), float32(cmd.Y), float32(cmd.Width), float32(cmd.Height), cmd.Color, false)
		case "triangle":
			// 使用路径绘制三角形
			vector.DrawFilledRect(screen, float32(cmd.X), float32(cmd.Y), float32(cmd.Width), float32(cmd.Height), cmd.Color, false)
		}
	}
}

// 将 PetState 转换为字符串
func petStateToString(state PetState) string {
	switch state {
	case StateIdle:
		return "idle"
	case StateTyping:
		return "typing"
	case StateThinking:
		return "thinking"
	case StateTalking:
		return "talking"
	case StateAlert:
		return "alert"
	default:
		return "idle"
	}
}

// startLocked 启动 Lua 虚拟机（需要持有锁）
func (m *LuaSkinManager) startLocked() error {
	if m.activeSkin == nil {
		return fmt.Errorf("no active skin")
	}

	// 创建新的 Lua 状态
	m.luaState = lua.NewState()
	L := m.luaState

	// 设置绘制 API
	m.setupDrawAPI(L)

	// 加载皮肤脚本
	if err := L.DoFile(m.activeSkin.FilePath); err != nil {
		m.luaState.Close()
		m.luaState = nil
		return fmt.Errorf("failed to load Lua skin: %w", err)
	}

	// 获取返回的皮肤对象
	m.skinInstance = L.Get(-1)
	L.Pop(1)

	if m.skinInstance.Type() != lua.LTTable {
		m.luaState.Close()
		m.luaState = nil
		m.skinInstance = nil
		return fmt.Errorf("Lua skin must return a table")
	}

	// 初始化皮肤
	setPosFn := L.GetField(m.skinInstance, "setPosition")
	if setPosFn.Type() == lua.LTFunction {
		L.Push(setPosFn)
		L.Push(m.skinInstance)
		L.Push(lua.LNumber(m.posX))
		L.Push(lua.LNumber(m.posY))
		L.Call(3, 0)
	}

	// 设置初始状态
	setStateFn := L.GetField(m.skinInstance, "setState")
	if setStateFn.Type() == lua.LTFunction {
		L.Push(setStateFn)
		L.Push(m.skinInstance)
		L.Push(lua.LString("idle"))
		L.Call(2, 0)
	}

	m.isRunning = true
	return nil
}

// stopLocked 停止 Lua 虚拟机（需要持有锁）
func (m *LuaSkinManager) stopLocked() {
	if !m.isRunning {
		return
	}

	if m.luaState != nil {
		m.luaState.Close()
		m.luaState = nil
	}

	m.skinInstance = nil
	m.isRunning = false
	m.drawCommands = m.drawCommands[:0]
}

// setupDrawAPI 设置绘制 API
func (m *LuaSkinManager) setupDrawAPI(L *lua.LState) {
	// 创建 graphics 表
	graphics := L.NewTable()

	// setColor(r, g, b, a)
	L.SetField(graphics, "setColor", L.NewFunction(func(L *lua.LState) int {
		r := float32(L.CheckNumber(1))
		g := float32(L.CheckNumber(2))
		b := float32(L.CheckNumber(3))
		a := float32(L.CheckNumber(4))
		m.currentColor = color.RGBA{
			R: uint8(r * 255),
			G: uint8(g * 255),
			B: uint8(b * 255),
			A: uint8(a * 255),
		}
		return 0
	}))

	// circle(mode, x, y, radius)
	L.SetField(graphics, "circle", L.NewFunction(func(L *lua.LState) int {
		mode := L.CheckString(1)
		x := float64(L.CheckNumber(2))
		y := float64(L.CheckNumber(3))
		radius := float64(L.CheckNumber(4))
		if mode == "fill" {
			m.drawCommands = append(m.drawCommands, DrawCommand{
				Type:   "circle",
				X:      x,
				Y:      y,
				Radius: radius,
				Color:  m.currentColor,
			})
		}
		return 0
	}))

	// rectangle(mode, x, y, width, height)
	L.SetField(graphics, "rectangle", L.NewFunction(func(L *lua.LState) int {
		mode := L.CheckString(1)
		x := float64(L.CheckNumber(2))
		y := float64(L.CheckNumber(3))
		width := float64(L.CheckNumber(4))
		height := float64(L.CheckNumber(5))
		if mode == "fill" {
			m.drawCommands = append(m.drawCommands, DrawCommand{
				Type:   "rect",
				X:      x,
				Y:      y,
				Width:  width,
				Height: height,
				Color:  m.currentColor,
			})
		}
		return 0
	}))

	// line(x1, y1, x2, y2, ...)
	L.SetField(graphics, "line", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		if n >= 4 {
			x1 := float64(L.CheckNumber(1))
			y1 := float64(L.CheckNumber(2))
			x2 := float64(L.CheckNumber(3))
			y2 := float64(L.CheckNumber(4))

			// 计算线宽（使用默认值 1）
			width := 1.0
			if n >= 5 {
				width = float64(L.CheckNumber(5))
			}

			// 使用矩形模拟线条
			dx := x2 - x1
			dy := y2 - y1
			length := math.Sqrt(dx*dx + dy*dy)
			if length > 0 {
				_ = math.Atan2(dy, dx) // 计算角度（预留用于旋转）
				m.drawCommands = append(m.drawCommands, DrawCommand{
					Type:   "line",
					X:      x1,
					Y:      y1 - width/2,
					Width:  length,
					Height: width,
					Color:  m.currentColor,
				})
			}
		}
		return 0
	}))

	// print(text, x, y)
	L.SetField(graphics, "print", L.NewFunction(func(L *lua.LState) int {
		// 暂时不实现文字渲染
		return 0
	}))

	// 将 graphics 表设置为全局变量
	L.SetGlobal("graphics", graphics)

	// 添加 math 函数
	L.SetGlobal("sin", L.NewFunction(func(L *lua.LState) int {
		v := L.CheckNumber(1)
		L.Push(lua.LNumber(math.Sin(float64(v))))
		return 1
	}))

	L.SetGlobal("cos", L.NewFunction(func(L *lua.LState) int {
		v := L.CheckNumber(1)
		L.Push(lua.LNumber(math.Cos(float64(v))))
		return 1
	}))

	L.SetGlobal("abs", L.NewFunction(func(L *lua.LState) int {
		v := L.CheckNumber(1)
		L.Push(lua.LNumber(math.Abs(float64(v))))
		return 1
	}))

	L.SetGlobal("min", L.NewFunction(func(L *lua.LState) int {
		a := float64(L.CheckNumber(1))
		b := float64(L.CheckNumber(2))
		if a < b {
			L.Push(lua.LNumber(a))
		} else {
			L.Push(lua.LNumber(b))
		}
		return 1
	}))

	L.SetGlobal("max", L.NewFunction(func(L *lua.LState) int {
		a := float64(L.CheckNumber(1))
		b := float64(L.CheckNumber(2))
		if a > b {
			L.Push(lua.LNumber(a))
		} else {
			L.Push(lua.LNumber(b))
		}
		return 1
	}))

	L.SetGlobal("floor", L.NewFunction(func(L *lua.LState) int {
		v := L.CheckNumber(1)
		L.Push(lua.LNumber(math.Floor(float64(v))))
		return 1
	}))

	L.SetGlobal("pi", lua.LNumber(math.Pi))
}

// GetLuaSkinManager 获取或创建 Lua 皮肤管理器单例

// IsLuaSkinAvailable 检查 Lua 皮肤系统是否可用
func IsLuaSkinAvailable() bool {
	return true // gopher-lua 总是可用
}

// InitLuaSkinSystem 初始化 Lua 皮肤系统
func InitLuaSkinSystem() error {
	// 自动加载所有 Lua 皮肤
	if err := AutoLoadLuaSkins(); err != nil {
		return err
	}

	return nil
}
