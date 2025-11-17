package timeservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"own-1Pixel/backend/go/logger"
	"own-1Pixel/backend/go/timeservice/clock"
)

// TimeServiceASyncTimeResponse 时间信息响应
type TimeServiceASyncTimeResponse struct {
	SystemTime     string `json:"system_time"`      // 系统时间
	SyncTimestamp  int64  `json:"sync_timestamp"`   // 同步时间戳（纳秒）
	SyncTime       string `json:"sync_time"`        // 格式化的同步时间
	SyncTimeOffset int64  `json:"sync_time_offset"` // 同步时间偏移量（纳秒）
	IsDegraded     bool   `json:"is_degraded"`      // 是否处于降级模式
}

// TimeServiceAStatusResponse 状态响应
type TimeServiceAStatusResponse struct {
	IsInitialized bool   `json:"is_initialized"` // 是否已初始化
	IsDegraded    bool   `json:"is_degraded"`    // 是否降级模式
	LastSyncTime  string `json:"last_sync_time"` // 最后同步时间
}

// TimeServiceAStatsResponse 统计信息响应
type TimeServiceAStatsResponse struct {
	TotalSyncs      int64   `json:"total_syncs"`      // 总同步次数
	SuccessfulSyncs int64   `json:"successful_syncs"` // 成功同步次数
	FailedSyncs     int64   `json:"failed_syncs"`     // 失败同步次数
	LastDeviation   float64 `json:"last_deviation"`   // 最后偏差（纳秒）
	MaxDeviation    int64   `json:"max_deviation"`    // 最大偏差（纳秒）
}

// TimeServiceANTPPoolResponse NTP池信息响应
type TimeServiceANTPPoolResponse struct {
	NTPServers []TimeServiceANTPServer `json:"ntp_servers"` // NTP服务器列表
}

// TimeServiceANTPServer NTP服务器信息
type TimeServiceANTPServer struct {
	Name         string                  `json:"name"`           // 服务器名称
	Address      string                  `json:"address"`        // 服务器地址
	Weight       float64                 `json:"weight"`         // 权重
	IsDomestic   bool                    `json:"is_domestic"`    // 是否为国内服务器
	MaxDeviation int64                   `json:"max_deviation"`  // 最大允许偏差(纳秒)
	IsActive     bool                    `json:"is_active"`      // 是否活跃
	LastSyncTime string                  `json:"last_sync_time"` // 最后同步时间
	Samples      []TimeServiceANTPSample `json:"samples"`        // 上一次获取的样本数据
	IsSelected   bool                    `json:"is_selected"`    // 是否被选中用于时间同步
}

// TimeServiceANTPSample NTP样本数据
type TimeServiceANTPSample struct {
	Timestamp int64  `json:"timestamp"` // 时间戳（纳秒）
	Status    string `json:"status"`    // 样本状态：成功、失败
	Delay     int64  `json:"delay"`     // 往返延迟（纳秒）
	Offset    int64  `json:"offset"`    // 时间偏移量（纳秒）
}

// TimeServiceACircuitBreakerResponse 熔断器状态响应
type TimeServiceACircuitBreakerResponse struct {
	IsOpen          bool   `json:"is_open"`           // 是否打开（熔断）
	FailureCount    int64  `json:"failure_count"`     // 失败计数
	LastFailureTime string `json:"last_failure_time"` // 最后失败时间
	SuccessCount    int64  `json:"success_count"`     // 成功计数
}

// GetSyncTime 获取同步时间
func GetSyncTime(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查时间服务是否已初始化
	status := GetTimeServiceStatus()
	if !status.IsInitialized {
		// 时间服务未初始化，返回系统时间（降级模式）
		systemTime := SyncNow()
		response := TimeServiceASyncTimeResponse{
			SystemTime:     clock.Format(systemTime),
			SyncTimestamp:  systemTime.UnixNano(),
			SyncTime:       clock.Format(systemTime),
			SyncTimeOffset: 0,
			IsDegraded:     true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// 获取同步时间
	systemTime := clock.Format(clock.Now())
	syncTimestamp := GetSyncTimestamp()
	syncTime := syncTimestamp.UnixNano()
	syncTimeOffset := GetSyncTimestampOffset()
	isDegraded := IsInDegradedMode()
	syncTimeFormatted := clock.Format(syncTimestamp)

	// 构建响应
	response := TimeServiceASyncTimeResponse{
		SystemTime:     systemTime,
		SyncTimestamp:  syncTime,
		SyncTime:       syncTimeFormatted,
		SyncTimeOffset: syncTimeOffset,
		IsDegraded:     isDegraded,
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 返回JSON响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Info("TimeServiceAPI", fmt.Sprintf("编码时间信息响应失败: %v\n", err))
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
}

// GetStatus 获取时间服务状态
func GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 获取状态
	status := GetTimeServiceStatus()

	// 构建响应
	response := TimeServiceAStatusResponse{
		IsInitialized: status.IsInitialized,
		IsDegraded:    status.IsDegraded,
		LastSyncTime:  clock.Format(status.LastSyncTime),
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 返回JSON响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Info("TimeServiceAPI", fmt.Sprintf("编码状态响应失败: %v\n", err))
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
}

// GetStats 获取时间服务统计信息
func GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 获取统计信息
	stats := GetTimeServiceStats()

	// 构建响应
	response := TimeServiceAStatsResponse(stats)

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 返回JSON响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Info("TimeServiceAPI", fmt.Sprintf("编码统计信息响应失败: %v\n", err))
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
}

// GetCircuitBreakerState 获取熔断器状态
func GetCircuitBreakerState(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 获取熔断器状态
	cbState := GetTimeServiceCircuitBreakerState()

	// 构建响应
	response := TimeServiceACircuitBreakerResponse{
		IsOpen:          cbState.IsOpen,
		FailureCount:    cbState.FailureCount,
		LastFailureTime: clock.Format(cbState.LastFailureTime),
		SuccessCount:    cbState.SuccessCount,
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 返回JSON响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Info("TimeServiceAPI", fmt.Sprintf("编码熔断器状态响应失败: %v\n", err))
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
}

// GetNTPPool 获取NTP池信息
func GetNTPPool(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 获取NTP服务器列表
	ntpServers := GetNTPServers()

	// 获取lastNTPSamples数据
	lastSamples := GetLastNTPSamples()

	// 转换为响应格式
	var ntpServerResponse []TimeServiceANTPServer
	for _, server := range ntpServers {
		var samples []TimeServiceANTPSample
		if serverSamples, exists := lastSamples[server.Address]; exists {
			// 转换为API响应格式
			for _, sample := range serverSamples {
				samples = append(samples, TimeServiceANTPSample{
					Timestamp: sample.Timestamp,        // 使用实际的时间戳
					Status:    sample.Status,           // 使用实际的Status值
					Delay:     sample.RTT,              // 使用RTT作为Delay
					Offset:    int64(sample.Deviation), // 使用Deviation作为Offset
				})
			}
		}

		// 基本信息始终填充
		serverResponse := TimeServiceANTPServer{
			Name:         server.Name,
			Address:      server.Address,
			Weight:       server.Weight,
			IsDomestic:   server.IsDomestic,
			MaxDeviation: server.MaxDeviation,
			IsActive:     len(samples) > 0,          // 如果有样本数据，则认为服务器是活跃的
			LastSyncTime: clock.Format(clock.Now()), // 使用系统时间
			Samples:      samples,
			IsSelected:   server.IsSelected,
		}

		ntpServerResponse = append(ntpServerResponse, serverResponse)
	}

	// 构建响应
	response := TimeServiceANTPPoolResponse{
		NTPServers: ntpServerResponse,
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 返回JSON响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Info("TimeServiceAPI", fmt.Sprintf("编码NTP池信息响应失败: %v\n", err))
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
}

// InitTimeServiceAPI 初始化时间服务API处理器
func InitTimeServiceAPI() error {
	return nil
}
