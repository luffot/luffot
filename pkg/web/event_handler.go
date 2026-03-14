package web

import (
	"context"
	"fmt"

	"github.com/luffot/luffot/pkg/eventsource"
	"github.com/luffot/luffot/pkg/manager"
)

// EventServerConfig HTTP 事件接收服务配置
type EventServerConfig struct {
	Host    string
	Port    int
	Manager *manager.Manager
}

// EventServer HTTP 事件接收服务
// 封装 HTTPEventSource 的启动，与 Manager 集成
type EventServer struct {
	config     EventServerConfig
	httpSource *eventsource.HTTPEventSource
}

// NewEventServer 创建 HTTP 事件接收服务
func NewEventServer(cfg EventServerConfig) *EventServer {
	httpSource := eventsource.NewHTTPEventSource(eventsource.HTTPEventSourceConfig{
		Host:       cfg.Host,
		Port:       cfg.Port,
		PathPrefix: "/event",
		AppName:    "",
	})
	cfg.Manager.AddEventSource(httpSource)

	return &EventServer{
		config:     cfg,
		httpSource: httpSource,
	}
}

// Addr 返回监听地址字符串
func (s *EventServer) Addr() string {
	return fmt.Sprintf("http://%s:%d/event/{app}/on_msg", s.config.Host, s.config.Port)
}

// Stop 停止事件服务
func (s *EventServer) Stop() error {
	return s.httpSource.Stop()
}

// IsRunning 检查是否运行中
func (s *EventServer) IsRunning() bool {
	return s.httpSource.IsRunning()
}

// StartEventServer 便捷函数：创建并启动 HTTP 事件接收服务
// 返回 EventServer 实例，调用方负责在 Manager.Start 之前调用此函数
func StartEventServer(ctx context.Context, host string, port int, mgr *manager.Manager) (*EventServer, error) {
	server := NewEventServer(EventServerConfig{
		Host:    host,
		Port:    port,
		Manager: mgr,
	})
	fmt.Printf("正在启动 HTTP 事件服务 (端口：%d)... ", port)
	fmt.Println("完成")
	return server, nil
}
