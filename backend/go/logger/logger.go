package logger

import (
	"fmt"
	"os"
	"own-1Pixel/backend/go/config"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	logFile *os.File
	mutex   sync.Mutex
)

// isValidPath 检查路径是否合法
func isValidPath(path string) bool {
	// 检查路径长度是否合理（Windows路径通常不超过260字符）
	if len(path) > 260 {
		return false
	}

	// 检查路径是否包含非法字符（Windows系统）
	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		if strings.Contains(path, char) {
			return false
		}
	}

	// 检查路径是否为绝对路径
	if !filepath.IsAbs(path) {
		return false
	}

	return true
}

// Init 初始化日志记录器
func Init(_config config.Config) {
	var logPath string

	// 确定使用的日志路径
	if logPath == "" || !isValidPath(logPath) {
		// 使用默认配置中的日志路径
		logPath = _config.LogPath
	}

	// 如果日志文件未打开，则打开或创建它
	if logFile == nil {
		var err error

		// 确保日志目录存在
		logDir := filepath.Dir(logPath)
		if err = os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("failed to create log directory: %v\n", err)
			return
		}

		// 打开或创建日志文件
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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
