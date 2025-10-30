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

// Info 记录信息级别的日志
func Info(packageName, message string, logPath ...string) {
	var actualLogPath string

	// 确定使用的日志路径
	if len(logPath) > 0 && logPath[0] != "" {
		// 使用用户提供的日志路径
		actualLogPath = logPath[0]
	} else {
		// 使用默认配置中的日志路径
		_config := config.GetConfig()
		actualLogPath = _config.LogPath
	}

	// 检查是否需要重新打开日志文件
	mutex.Lock()
	defer mutex.Unlock()

	// 如果日志文件未打开，则打开或创建它
	if logFile == nil {
		var err error

		// 确保日志目录存在
		logDir := filepath.Dir(actualLogPath)
		if err = os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("failed to create log directory: %v\n", err)
			return
		}

		// 打开或创建日志文件
		logFile, err = os.OpenFile(actualLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			fmt.Printf("failed to open log file: %v\n", err)
			return
		}
	}

	// 获取当前时间，格式化为 年-月-日 时:分:秒.毫秒(保留3位)
	now := time.Now()
	timeStr := now.Format("2006-01-02 15:04:05.000")

	// 构建日志消息
	logMessage := fmt.Sprintf("%s [%s] %s\n", timeStr, packageName, message)

	// 写入日志文件
	logFile.WriteString(logMessage)
	logFile.Sync() // 确保日志立即写入磁盘
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
