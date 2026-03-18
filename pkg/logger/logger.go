package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RollingWriter 实现滚动日志写入器
type RollingWriter struct {
	mu           sync.Mutex
	basePath     string        // 日志文件基础路径
	maxAge       time.Duration // 最大保留时间
	maxSize      int64         // 最大文件大小（字节）
	currentFile  *os.File
	currentSize  int64
	currentDate  string
	stdoutWriter io.Writer
}

// RollingConfig 滚动日志配置
type RollingConfig struct {
	BasePath string        // 日志文件基础路径（如 ~/.luffot/log/app.log）
	MaxAge   time.Duration // 最大保留时间（如 2 * 24 * time.Hour）
	MaxSize  int64         // 最大文件大小（字节，如 200 * 1024 * 1024）
}

// NewRollingWriter 创建滚动日志写入器
func NewRollingWriter(config RollingConfig) (*RollingWriter, error) {
	// 确保日志目录存在
	dir := filepath.Dir(config.BasePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	rw := &RollingWriter{
		basePath:     config.BasePath,
		maxAge:       config.MaxAge,
		maxSize:      config.MaxSize,
		stdoutWriter: os.Stdout,
		currentDate:  time.Now().Format("2006-01-02"),
	}

	// 打开当前日志文件
	if err := rw.openFile(); err != nil {
		return nil, err
	}

	// 清理过期日志
	go rw.cleanupRoutine()

	return rw, nil
}

// openFile 打开当前日志文件
func (rw *RollingWriter) openFile() error {
	// 检查是否需要按日期滚动
	today := time.Now().Format("2006-01-02")
	if today != rw.currentDate {
		rw.currentDate = today
		rw.currentSize = 0
	}

	// 构建日志文件路径
	logPath := rw.buildLogPath()

	// 以追加模式打开文件
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	// 获取当前文件大小
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("获取日志文件信息失败: %w", err)
	}

	rw.currentFile = file
	rw.currentSize = info.Size()

	return nil
}

// buildLogPath 构建日志文件路径
func (rw *RollingWriter) buildLogPath() string {
	ext := filepath.Ext(rw.basePath)
	base := strings.TrimSuffix(rw.basePath, ext)
	return fmt.Sprintf("%s-%s%s", base, rw.currentDate, ext)
}

// shouldRotate 检查是否需要滚动
func (rw *RollingWriter) shouldRotate(n int) bool {
	return rw.currentSize+int64(n) > rw.maxSize
}

// rotate 执行日志滚动
func (rw *RollingWriter) rotate() error {
	// 关闭当前文件
	if rw.currentFile != nil {
		rw.currentFile.Close()
		rw.currentFile = nil
	}

	// 重命名当前文件（添加序号）
	currentPath := rw.buildLogPath()
	for i := 1; ; i++ {
		newPath := fmt.Sprintf("%s.%d", currentPath, i)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			if err := os.Rename(currentPath, newPath); err != nil {
				return fmt.Errorf("重命名日志文件失败: %w", err)
			}
			break
		}
	}

	// 重新打开新文件
	rw.currentSize = 0
	return rw.openFile()
}

// Write 实现 io.Writer 接口
func (rw *RollingWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// 同时输出到 stdout
	if rw.stdoutWriter != nil {
		rw.stdoutWriter.Write(p)
	}

	// 检查是否需要滚动
	if rw.shouldRotate(len(p)) {
		if err := rw.rotate(); err != nil {
			return 0, err
		}
	}

	// 写入文件
	n, err = rw.currentFile.Write(p)
	if err != nil {
		return n, err
	}

	rw.currentSize += int64(n)
	return n, nil
}

// Close 关闭写入器
func (rw *RollingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.currentFile != nil {
		return rw.currentFile.Close()
	}
	return nil
}

// cleanupRoutine 定期清理过期日志
func (rw *RollingWriter) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rw.cleanup()
	}
}

// cleanup 清理过期日志文件
func (rw *RollingWriter) cleanup() {
	dir := filepath.Dir(rw.basePath)
	baseName := filepath.Base(rw.basePath)
	ext := filepath.Ext(baseName)
	base := strings.TrimSuffix(baseName, ext)

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[Logger] 读取日志目录失败: %v", err)
		return
	}

	cutoff := time.Now().Add(-rw.maxAge)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// 检查是否是该应用的日志文件
		if !strings.HasPrefix(name, base+"-") || !strings.HasSuffix(name, ext) {
			continue
		}

		// 获取文件信息
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 删除过期文件
		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(dir, name)
			if err := os.Remove(fullPath); err != nil {
				log.Printf("[Logger] 删除过期日志文件失败 %s: %v", fullPath, err)
			} else {
				log.Printf("[Logger] 已删除过期日志: %s", name)
			}
		}
	}
}

// GetLogFiles 获取所有日志文件列表（按时间排序）
func (rw *RollingWriter) GetLogFiles() ([]string, error) {
	dir := filepath.Dir(rw.basePath)
	baseName := filepath.Base(rw.basePath)
	ext := filepath.Ext(baseName)
	base := strings.TrimSuffix(baseName, ext)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, base+"-") && strings.HasSuffix(name, ext) {
			files = append(files, filepath.Join(dir, name))
		}
	}

	// 按修改时间排序（最新的在前）
	sort.Slice(files, func(i, j int) bool {
		infoI, _ := os.Stat(files[i])
		infoJ, _ := os.Stat(files[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return files, nil
}

// InitLogger 初始化全局日志，同时输出到 stdout 和滚动日志文件
func InitLogger(logDir string) error {
	logPath := filepath.Join(logDir, "app.log")

	rw, err := NewRollingWriter(RollingConfig{
		BasePath: logPath,
		MaxAge:   2 * 24 * time.Hour, // 2天
		MaxSize:  200 * 1024 * 1024,  // 200MB
	})
	if err != nil {
		return err
	}

	// 设置 log 包的输出
	log.SetOutput(rw)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return nil
}
