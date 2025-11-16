package clock

import (
	"fmt"
	"time"
)

var (
	systemTimestampBase    int64 // 系统时间基准（用于单调时间反向映射）
	monotonicTimestampBase int64 // 单调时间基准
)

func GetSystemTimestamp() int64 {
	return systemTimestampBase
}

func GetMonotonicTimestamp() int64 {
	return monotonicTimestampBase
}

func TimeFormat(timestamp int64) string {
	return time.Unix(0, timestamp).Format("2006-01-02 15:04:05.0000000")
}

func InitClock() {
	fmt.Println("初始化时钟基准系统...")
	time := time.Now().UnixNano()
	systemTimestampBase = time
	monotonicTimestampBase = time
}
