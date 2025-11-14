package timeservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"own-1Pixel/backend/go/logger"
	"time"
)

// TimeServiceAPI 时间服务API处理器
type TimeServiceAPI struct {
	timeService *TimeService
}

// NewTimeServiceAPI 创建时间服务API处理器
func NewTimeServiceAPI(ts *TimeService) *TimeServiceAPI {
	return &TimeServiceAPI{
		timeService: ts,
	}
}

// TimeServiceATimeInfoResponse 时间信息响应
type TimeServiceATimeInfoResponse struct {
	TrustedTimestamp int64  `json:"trusted_timestamp"` // 可信时间戳（纳秒）
	TrustedTime      string `json:"trusted_time"`      // 格式化的可信时间
	SystemTime       string `json:"system_time"`       // 系统时间
	SyncTimeOffset   int64  `json:"sync_time_offset"`  // 同步时间偏移量（纳秒）
	IsDegraded       bool   `json:"is_degraded"`       // 是否处于降级模式
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
	Offset    int64  `json:"offset"`    // 时间偏移量（纳秒）
	Delay     int64  `json:"delay"`     // 往返延迟（纳秒）
	Timestamp int64  `json:"timestamp"` // 时间戳（纳秒）
	Status    string `json:"status"`    // 样本状态：成功、失败
}

// TimeServiceACircuitBreakerResponse 熔断器状态响应
type TimeServiceACircuitBreakerResponse struct {
	IsOpen          bool   `json:"is_open"`           // 是否打开（熔断）
	FailureCount    int64  `json:"failure_count"`     // 失败计数
	LastFailureTime string `json:"last_failure_time"` // 最后失败时间
	SuccessCount    int64  `json:"success_count"`     // 成功计数
}

// GetTimeInfo 获取时间信息
func (api *TimeServiceAPI) GetTimeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查时间服务是否可用
	if api.timeService == nil {
		// 时间服务未初始化，返回系统时间（降级模式）
		systemTime := time.Now()
		response := TimeServiceATimeInfoResponse{
			TrustedTimestamp: systemTime.UnixNano(),
			TrustedTime:      systemTime.Format("2006-01-02 15:04:05.000000000"),
			SystemTime:       systemTime.Format("2006-01-02 15:04:05.000000000"),
			SyncTimeOffset:   0,
			IsDegraded:       true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// 获取可信时间
	trustedTimestamp := api.timeService.GetTrustedTimestamp()
	trustedTime := api.timeService.GetTrustedTime()
	systemTime := time.Now()
	syncTimeOffset := api.timeService.GetSyncTimeOffset()
	isDegraded := api.timeService.IsInDegradedMode()

	// 构建响应
	response := TimeServiceATimeInfoResponse{
		TrustedTimestamp: trustedTimestamp,
		TrustedTime:      trustedTime.Format("2006-01-02 15:04:05.000000000"),
		SystemTime:       systemTime.Format("2006-01-02 15:04:05.000000000"),
		SyncTimeOffset:   syncTimeOffset,
		IsDegraded:       isDegraded,
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
func (api *TimeServiceAPI) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查时间服务是否可用
	if api.timeService == nil {
		// 时间服务未初始化，返回未初始化状态
		response := TimeServiceAStatusResponse{
			IsInitialized: false,
			IsDegraded:    true,
			LastSyncTime:  time.Time{}.Format("2006-01-02 15:04:05.000000000"),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// 获取状态
	status := api.timeService.GetStatus()

	// 构建响应
	response := TimeServiceAStatusResponse{
		IsInitialized: status.IsInitialized,
		IsDegraded:    status.IsDegraded,
		LastSyncTime:  status.LastSyncTime.Format("2006-01-02 15:04:05.000000000"),
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
func (api *TimeServiceAPI) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查时间服务是否可用
	if api.timeService == nil {
		// 时间服务未初始化，返回空统计信息
		response := TimeServiceAStatsResponse{
			TotalSyncs:      0,
			SuccessfulSyncs: 0,
			FailedSyncs:     0,
			LastDeviation:   0,
			MaxDeviation:    0,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// 获取统计信息
	stats := api.timeService.GetStats()

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
func (api *TimeServiceAPI) GetCircuitBreakerState(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查时间服务是否可用
	if api.timeService == nil {
		// 时间服务未初始化，返回关闭状态的熔断器
		response := TimeServiceACircuitBreakerResponse{
			IsOpen:          false,
			FailureCount:    0,
			LastFailureTime: time.Time{}.Format("2006-01-02 15:04:05.000000000"),
			SuccessCount:    0,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// 获取熔断器状态
	cbState := api.timeService.GetCircuitBreakerState()

	// 构建响应
	response := TimeServiceACircuitBreakerResponse{
		IsOpen:          cbState.IsOpen,
		FailureCount:    cbState.FailureCount,
		LastFailureTime: cbState.LastFailureTime.Format("2006-01-02 15:04:05.000000000"),
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
func (api *TimeServiceAPI) GetNTPPool(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查时间服务是否可用
	if api.timeService == nil {
		// 时间服务未初始化，返回空NTP池
		response := TimeServiceANTPPoolResponse{
			NTPServers: []TimeServiceANTPServer{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// 获取NTP服务器列表
	ntpServers := api.timeService.GetNTPServers()

	// 转换为响应格式
	var ntpServerInfos []TimeServiceANTPServer
	for _, server := range ntpServers {
		// 查询服务器详细信息
		_, err := api.timeService.QueryNTPServerDetailed(server)

		// 获取该服务器的样本数据
		lastSamples := api.timeService.GetLastNTPSamples()
		var samples []TimeServiceANTPSample
		if serverSamples, exists := lastSamples[server.Address]; exists {
			// 转换为API响应格式
			for _, sample := range serverSamples {
				samples = append(samples, TimeServiceANTPSample{
					Offset:    int64(sample.Deviation), // 使用Deviation作为Offset
					Delay:     sample.RTT,              // 使用RTT作为Delay
					Timestamp: sample.Timestamp,
					Status:    sample.Status, // 使用实际的Status值
				})
			}
		}

		// 基本信息始终填充
		serverInfo := TimeServiceANTPServer{
			Name:         server.Name,
			Address:      server.Address,
			Weight:       server.Weight,
			IsDomestic:   server.IsDomestic,
			MaxDeviation: server.MaxDeviation,
			IsActive:     true, // 默认所有服务器都是活跃的，实际应用中可以根据状态判断
			LastSyncTime: time.Now().Format("2006-01-02 15:04:05.000000000"),
			Samples:      samples,           // 添加样本数据
			IsSelected:   server.IsSelected, // 添加IsSelected字段
		}

		// 如果查询成功，填充基本信息
		if err == nil {
			// 只保留基本信息，不填充已移除的字段
		} else {
			// 查询失败，设置默认值
			serverInfo.IsActive = false
		}

		ntpServerInfos = append(ntpServerInfos, serverInfo)
	}

	// 构建响应
	response := TimeServiceANTPPoolResponse{
		NTPServers: ntpServerInfos,
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
