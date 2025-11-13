package config

import (
	"time"
)

// Config 配置结构
type Config struct {
	Host        string            `json:"host"`        // 服务主机
	Port        int               `json:"port"`        // 服务端口
	DbPath      string            `json:"dbPath"`      // 数据库路径
	LogPath     string            `json:"logPath"`     // 日志路径
	TimeService TimeServiceConfig `json:"timeService"` // 时间服务配置
}

// TimeServiceConfig 时间服务配置
type TimeServiceConfig struct {
	SyncInterval     time.Duration `json:"syncInterval"`     // 同步间隔
	MaxDeviation     int64         `json:"maxDeviation"`     // 最大允许偏差(纳秒)
	FailureThreshold int64         `json:"failureThreshold"` // 失败阈值
	RecoveryTimeout  time.Duration `json:"recoveryTimeout"`  // 恢复超时
}

// TimeServiceNTPServer NTP服务器配置
type TimeServiceNTPServer struct {
	Name         string  `json:"name"`         // 服务器名称
	Address      string  `json:"address"`      // 服务器地址
	Weight       float64 `json:"weight"`       // 权重
	IsDomestic   bool    `json:"isDomestic"`   // 是否为国内服务器
	MaxDeviation int64   `json:"maxDeviation"` // 最大允许偏差(纳秒)
}

// GetConfig 获取默认配置
func GetConfig() Config {
	return Config{
		Host:    "0.0.0.0",                // 监听IP地址
		Port:    8080,                     // 监听端口
		DbPath:  "./backend/data/cash.db", // 数据库路径
		LogPath: "./backend/logs/app.log", // 日志路径
		TimeService: TimeServiceConfig{
			SyncInterval:     1 * time.Hour,                 // 同步间隔
			MaxDeviation:     2 * time.Second.Nanoseconds(), // 最大允许偏差(纳秒)
			FailureThreshold: 5,                             // 失败阈值
			RecoveryTimeout:  60 * time.Second,              // 恢复超时
		},
	}
}

// GetDefaultNTPServers 获取默认NTP服务器列表
func GetDefaultNTPServers() []TimeServiceNTPServer {
	return []TimeServiceNTPServer{
		{Name: "国家授时中心", Address: "ntp.ntsc.ac.cn", Weight: 10.0, IsDomestic: true, MaxDeviation: 2 * time.Second.Nanoseconds()},
		{Name: "上海交通大学", Address: "ntp.sjtu.edu.cn", Weight: 5.0, IsDomestic: true, MaxDeviation: 2 * time.Second.Nanoseconds()},
		{Name: "阿里云", Address: "ntp.aliyun.com", Weight: 2.0, IsDomestic: true, MaxDeviation: 2 * time.Second.Nanoseconds()},
		{Name: "海外备用源（微软）", Address: "time.windows.com", Weight: 1.0, IsDomestic: false, MaxDeviation: 2 * time.Second.Nanoseconds()},
	}
}
