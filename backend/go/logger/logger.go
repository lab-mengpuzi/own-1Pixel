package logger

import (
	"fmt"
	"os"
	"own-1Pixel/backend/go/config"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile *os.File
	mutex   sync.Mutex
)

// Init 初始化日志记录器
func Init() {
	// 获取全局配置实例
	_config := config.GetConfig()
	loggerConfig := _config.Logger

	// 如果日志文件未打开，则打开或创建它
	if logFile == nil {
		var err error

		// 确保日志目录存在
		logDir := filepath.Dir(loggerConfig.Path)
		if err = os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("failed to create log directory: %v\n", err)
			return
		}

		// 打开或创建日志文件
		logFile, err = os.OpenFile(loggerConfig.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			fmt.Printf("failed to open log file: %v\n", err)
			return
		}
	}
}

// Info 记录信息级别的日志
func Info(packageName, message string) {
	// 检查日志文件是否已初始化
	mutex.Lock()
	defer mutex.Unlock()

	if logFile == nil {
		fmt.Printf("Logger not initialized, cannot write log: [%s] %s\n", packageName, message)
		return
	}

	// 获取当前时间，格式化为 年-月-日 时:分:秒.毫秒(保留3位)
	now := time.Now()
	dateFormat := now.Format("2006-01-02 15:04:05.000")

	// 构建日志消息
	logMessage := fmt.Sprintf("%s [%s] %s", dateFormat, packageName, message)

	// 写入日志文件
	_, err := logFile.WriteString(logMessage)
	if err != nil {
		fmt.Printf("Failed to write log: %v\n", err)
		return
	}

	// 确保日志立即写入磁盘
	err = logFile.Sync()
	if err != nil {
		fmt.Printf("Failed to sync log file: %v\n", err)
	}
}

// Close 关闭日志文件
func Close() {
	mutex.Lock()
	defer mutex.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}
