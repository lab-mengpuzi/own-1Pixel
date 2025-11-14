package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config 配置结构
type Config struct {
	Host        string                 `json:"host"`        // 服务主机
	Port        int                    `json:"port"`        // 服务端口
	ConfigPath  string                 `json:"configPath"`  // 配置文件路径
	LogPath     string                 `json:"logPath"`     // 日志路径
	DbPath      string                 `json:"dbPath"`      // 数据库路径
	TimeService TimeServiceConfig      `json:"timeService"` // 时间服务配置
	NTPServer   []TimeServiceNTPServer `json:"ntpServer"`   // NTP服务器列表
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
	IsSelected   bool    `json:"isSelected"`   // 是否被选中用于时间同步
}

// 默认配置对象
var config = Config{
	Host:       "0.0.0.0",                    // 监听IP地址
	Port:       8080,                         // 监听端口
	ConfigPath: "./backend/data/config.json", // 配置文件路径
	LogPath:    "./backend/logs/app.log",     // 日志路径
	DbPath:     "./backend/data/cash.db",     // 数据库路径
	TimeService: TimeServiceConfig{
		SyncInterval:     1 * time.Hour,                 // 同步间隔
		MaxDeviation:     2 * time.Second.Nanoseconds(), // 最大允许偏差(纳秒)
		FailureThreshold: 5,                             // 失败阈值
		RecoveryTimeout:  60 * time.Second,              // 恢复超时
	},
	NTPServer: []TimeServiceNTPServer{
		{Name: "国家授时中心", Address: "ntp.ntsc.ac.cn", Weight: 4.0, IsDomestic: true, MaxDeviation: 2 * time.Second.Nanoseconds(), IsSelected: false},
		{Name: "上海交通大学", Address: "ntp.sjtu.edu.cn", Weight: 3.0, IsDomestic: true, MaxDeviation: 2 * time.Second.Nanoseconds(), IsSelected: false},
		{Name: "阿里云", Address: "ntp.aliyun.com", Weight: 2.0, IsDomestic: true, MaxDeviation: 2 * time.Second.Nanoseconds(), IsSelected: false},
		{Name: "海外备用源（微软）", Address: "time.windows.com", Weight: 1.0, IsDomestic: false, MaxDeviation: 2 * time.Second.Nanoseconds(), IsSelected: false},
	}, // 使用默认NTP服务器列表初始化
}

// InitConfig 获取配置对象（对外提供的统一接口）
func InitConfig() Config {
	// 每次调用时重新加载配置文件，确保获取最新配置
	_config, err := LoadConfig()

	if err != nil {
		// 如果加载失败，使用内存中的默认配置
		return config
	}

	return _config
}

// LoadConfig 从JSON文件加载配置，如果文件不存在则创建默认配置文件
func LoadConfig() (Config, error) {
	// 检查文件是否存在
	if _, fileCheckErr := os.Stat(config.ConfigPath); os.IsNotExist(fileCheckErr) {
		// 文件不存在，创建目录并保存默认配置
		if err := saveConfig(config); err != nil {
			return config, fmt.Errorf("无法创建默认配置文件: %v", err)
		}
		return config, nil
	}

	// 读取文件内容
	data, readErr := os.ReadFile(config.ConfigPath)
	if readErr != nil {
		return config, fmt.Errorf("无法读取配置文件: %v", readErr)
	}

	// 解析JSON到配置结构
	if unmarshalErr := json.Unmarshal(data, &config); unmarshalErr != nil {
		return config, fmt.Errorf("无法解析JSON配置: %v", unmarshalErr)
	}

	return config, nil
}

// saveConfig 将配置保存到JSON文件
func saveConfig(cfg Config) error {
	// 将配置结构转换为JSON
	data, marshalErr := json.MarshalIndent(cfg, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("无法序列化配置为JSON: %v", marshalErr)
	}

	// 获取文件目录路径
	dir := filepath.Dir(cfg.ConfigPath)

	// 检查目录是否存在，如果不存在则创建
	if _, dirErr := os.Stat(dir); os.IsNotExist(dirErr) {
		if mkdirErr := os.MkdirAll(dir, 0755); mkdirErr != nil {
			return fmt.Errorf("无法创建目录: %v", mkdirErr)
		}
	}

	// 写入文件
	if writeErr := os.WriteFile(cfg.ConfigPath, data, 0644); writeErr != nil {
		return fmt.Errorf("无法保存配置到文件: %v", writeErr)
	}

	return nil
}
