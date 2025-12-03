package clock

import (
	"fmt"
	"time"
)

var (
	monotonicTimestampBase int64 // 单调时间基准
)

func getSystemTimestamp() int64 {
	return time.Now().UnixNano()
}

func GetMonotonicTimestamp() int64 {
	return monotonicTimestampBase
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
