package config

// Config 配置结构
type Config struct {
	Host    string `json:"host"`    // 服务主机
	Port    int    `json:"port"`    // 服务端口
	DbPath  string `json:"dbPath"`  // 数据库路径
	LogPath string `json:"logPath"` // 日志路径
}

// GetConfig 获取默认配置
func GetConfig() Config {
	return Config{
		Host:    "0.0.0.0",                // 监听IP地址
		Port:    8080,                     // 监听端口
		DbPath:  "./backend/data/cash.db", // 数据库路径
		LogPath: "./backend/logs/app.log", // 日志路径
	}
}