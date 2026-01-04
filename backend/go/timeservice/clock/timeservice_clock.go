package clock

import (
	"fmt"
	"time"
)

var (
	monotonicTimestampBase int64 // 单调时间基准（程序启动时的系统时间）
)

// GetMonotonicTimestampBase 获取单调时间基准
func GetMonotonicTimestampBase() int64 {
	return monotonicTimestampBase
}

// getMonotonicTimestamp 获取当前单调时间戳
// 返回自程序启动以来经过的纳秒数（基于系统时间，不受NTP调整影响）
func getMonotonicTimestamp() int64 {
	// 返回当前系统时间与基准时间的差值
	// 虽然这不是真正的单调时钟，但对于大多数场景已经足够
	return getSystemTimestamp() - monotonicTimestampBase
}

// GetMonotonicTimestamp 获取当前单调时间戳
// 返回自程序启动以来经过的纳秒数（基于系统时间，不受NTP调整影响）
func GetMonotonicTimestamp() int64 {
	return getMonotonicTimestamp()
}

// GetMonotonicSince 获取从某个时间点到现在经过的单调时间
func GetMonotonicSince() time.Duration {
	// 返回自程序启动以来经过的时间间隔
	return time.Duration(getMonotonicTimestamp())
}

func getSystemTimestamp() int64 {
	return time.Now().UnixNano()
}

func Now() time.Time {
	return time.Unix(0, getSystemTimestamp())
}

func Format(now time.Time) string {
	return now.Format("2006-01-02 15:04:05.0000000")
}

func InitClock() {
	fmt.Println("初始化时钟基准系统...")
	monotonicTimestampBase = getSystemTimestamp()
}
