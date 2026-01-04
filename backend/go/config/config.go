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
	Main             MainConfig             `json:"main"`             // 服务配置
	ConfigPath       string                 `json:"configPath"`       // 配置文件路径
	Logger           LoggerConfig           `json:"logger"`           // 日志系统配置
	Cash             CashConfig             `json:"cash"`             // 现金系统配置
	Market           MarketConfig           `json:"market"`           // 市场系统配置
	AuctionWebSocket AuctionWebSocketConfig `json:"auctionWebSocket"` // 拍卖系统WebSocket配置
	TimeService      TimeServiceConfig      `json:"timeService"`      // 时间服务配置
}

// MainConfig 服务配置
type MainConfig struct {
	Host string `json:"host"` // 服务主机
	Port int    `json:"port"` // 服务端口
}

// LoggerConfig 日志系统配置
type LoggerConfig struct {
	Path string `json:"path"` // 日志文件路径
}

// CashConfig 现金系统配置
type CashConfig struct {
	DbPath string `json:"dbPath"` // 数据库路径
}

// MarketConfig 市场系统配置
type MarketConfig struct {
	InitialApplePrice  float64 `json:"initialApplePrice"`  // 苹果初始价格
	InitialWoodPrice   float64 `json:"initialWoodPrice"`   // 木材初始价格
	DefaultBalance     float64 `json:"defaultBalance"`     // 默认平衡系数
	DefaultFluctuation float64 `json:"defaultFluctuation"` // 默认波动系数
	DefaultMaxChange   float64 `json:"defaultMaxChange"`   // 默认最大变动系数
}

// AuctionWebSocketConfig 拍卖系统WebSocket配置
type AuctionWebSocketConfig struct {
	ReadLimit    int           `json:"readLimit"`    // 读取消息大小限制
	ReadTimeout  time.Duration `json:"readTimeout"`  // 读取超时时间
	PingInterval time.Duration `json:"pingInterval"` // 心跳间隔
	WriteTimeout time.Duration `json:"writeTimeout"` // 写入超时时间
}

// TimeServiceConfig 时间服务配置
type TimeServiceConfig struct {
	FailureThreshold int64                  `json:"failureThreshold"` // 失败阈值
	SampleCount      int                    `json:"sampleCount"`      // 样本数量
	SampleDelay      time.Duration          `json:"sampleDelay"`      // 样本延迟
	SyncInterval     time.Duration          `json:"syncInterval"`     // 同步间隔
	RecoveryTimeout  time.Duration          `json:"recoveryTimeout"`  // 恢复超时
	NTPServers       []TimeServiceNTPServer `json:"ntpServers"`       // NTP服务器列表
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
	ConfigPath: "./backend/data/config.json", // 配置文件路径
	Main: MainConfig{
		Host: "0.0.0.0", // 监听IP地址
		Port: 8080,      // 监听端口
	},
	Logger: LoggerConfig{
		Path: "./backend/logs/app.log", // 日志文件路径
	},
	Cash: CashConfig{
		DbPath: "./backend/data/cash.db", // 数据库路径
	},
	Market: MarketConfig{
		InitialApplePrice:  1.0, // 苹果初始价格
		InitialWoodPrice:   5.0, // 木材初始价格
		DefaultBalance:     1.0, // 默认平衡系数
		DefaultFluctuation: 1.0, // 默认波动系数
		DefaultMaxChange:   1.0, // 默认最大变动系数
	},
	AuctionWebSocket: AuctionWebSocketConfig{
		ReadLimit:    512,              // 读取消息大小限制
		ReadTimeout:  45 * time.Second, // 读取超时时间
		PingInterval: 25 * time.Second, // 心跳间隔
		WriteTimeout: 45 * time.Second, // 写入超时时间
	},
	TimeService: TimeServiceConfig{
		FailureThreshold: 5,                      // 失败阈值，同样本数量一致
		SampleCount:      5,                      // 样本数量
		SampleDelay:      500 * time.Millisecond, // 样本延迟
		SyncInterval:     1 * time.Hour,          // 同步间隔
		RecoveryTimeout:  60 * time.Second,       // 恢复超时
		NTPServers: []TimeServiceNTPServer{ // 使用默认NTP服务器列表初始化
			{Name: "国家授时中心", Address: "ntp.ntsc.ac.cn", Weight: 1.0, IsDomestic: true, MaxDeviation: 5 * time.Second.Nanoseconds(), IsSelected: false},
			{Name: "东北大学", Address: "ntp.neu.edu.cn", Weight: 2.0, IsDomestic: true, MaxDeviation: 5 * time.Second.Nanoseconds(), IsSelected: false},
			{Name: "大连东软信息学院", Address: "ntp.neusoft.edu.cn", Weight: 3.0, IsDomestic: true, MaxDeviation: 5 * time.Second.Nanoseconds(), IsSelected: false},
			{Name: "阿里云", Address: "ntp.aliyun.com", Weight: 4.0, IsDomestic: true, MaxDeviation: 5 * time.Second.Nanoseconds(), IsSelected: false},
			{Name: "海外备用源（微软）", Address: "time.windows.com", Weight: 5.0, IsDomestic: false, MaxDeviation: 5 * time.Second.Nanoseconds(), IsSelected: false},
		},
	},
}

// 全局配置实例
var globalConfig *Config

// InitConfig 初始化全局配置实例（仅在main.go中调用一次）
func InitConfig() Config {
	// 加载配置文件
	_config, err := LoadConfig()
	if err != nil {
		// 如果加载失败，使用默认配置
		_config = config
	}

	// 设置全局配置实例
	globalConfig = &_config
	return _config
}

// GetConfig 获取全局配置实例（供其他模块使用）
func GetConfig() *Config {
	if globalConfig == nil {
		// 如果全局配置未初始化，使用默认配置
		globalConfig = &config
	}
	return globalConfig
}

// LoadConfig 从JSON文件加载配置，如果文件不存在或为空则创建默认配置文件
func LoadConfig() (Config, error) {
	// 检查文件是否存在
	fileInfo, fileCheckErr := os.Stat(config.ConfigPath)
	if os.IsNotExist(fileCheckErr) {
		// 文件不存在，创建目录并保存默认配置
		if err := saveConfig(config); err != nil {
			return config, fmt.Errorf("无法创建默认配置文件: %v", err)
		}
		return config, nil
	}

	// 检查文件是否为空
	if fileInfo != nil && fileInfo.Size() == 0 {
		// 文件为空，保存默认配置
		if err := saveConfig(config); err != nil {
			return config, fmt.Errorf("无法保存默认配置到空文件: %v", err)
		}
		return config, nil
	}

	// 读取文件内容
	data, readErr := os.ReadFile(config.ConfigPath)
	if readErr != nil {
		return config, fmt.Errorf("无法读取配置文件: %v", readErr)
	}

	// 检查读取的内容是否为空
	if len(data) == 0 {
		// 文件内容为空，保存默认配置
		if err := saveConfig(config); err != nil {
			return config, fmt.Errorf("无法保存默认配置到空文件: %v", err)
		}
		return config, nil
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
