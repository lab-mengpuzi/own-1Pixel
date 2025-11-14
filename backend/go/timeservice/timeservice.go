package timeservice

import (
	"fmt"
	"math"
	"own-1Pixel/backend/go/config"
	"own-1Pixel/backend/go/logger"
	"sort"
	"sync/atomic"
	"time"

	"github.com/beevik/ntp"
)

// TimeService 提供金融级可信时间服务
type TimeService struct {
	config             TimeServiceConfig                 // 配置参数
	ntpServers         []TimeServiceNTPServer            // NTP服务器配置
	status             TimeServiceStatus                 // 时间服务状态
	circuitBreaker     TimeServiceCircuitBreakerState    // 熔断器状态
	lastNTPSamples     map[string][]TimeServiceNTPSample // 上一次获取的NTP样本数据，按服务器地址存储
	syncTimeOffset     int64                             // 全局时间偏移量（可信基准时间 - 单调时钟起点时间），原子更新
	monotonicTimeStart time.Time                         // 单调时钟起点时间
	stats              TimeServiceStats                  // 统计信息
}

// TimeServiceConfig 时间服务配置
type TimeServiceConfig struct {
	SyncInterval     time.Duration // 同步间隔
	MaxDeviation     int64         // 最大允许偏差(纳秒)
	FailureThreshold int64         // 失败阈值
	RecoveryTimeout  time.Duration // 恢复超时
}

// TimeServiceNTPServer NTP服务器配置
type TimeServiceNTPServer struct {
	Name         string  // 服务器名称
	Address      string  // 服务器地址
	Weight       float64 // 权重
	IsDomestic   bool    // 是否为国内服务器
	MaxDeviation int64   // 最大允许偏差(纳秒)
	IsSelected   bool    // 是否被选中用于时间同步
}

// TimeServiceNTPSample 单个NTP样本
type TimeServiceNTPSample struct {
	Timestamp int64   // 时间戳（纳秒）
	RTT       int64   // 往返时间（纳秒）
	Deviation float64 // 偏差（纳秒）
	Status    string  // 样本状态：成功、失败
}

// TimeServiceNTPTimeResult NTP查询结果（基于多个样本的聚合）
type TimeServiceNTPTimeResult struct {
	Timestamp    int64   // 聚合时间戳（纳秒）
	Deviation    float64 // 最后一个成功样本的偏差（纳秒）
	Weight       float64 // 权重
	Server       string  // 服务器地址
	SampleCount  int     // 样本数量
	SuccessCount int     // 成功样本数量
	RTT          float64 // 往返时间（纳秒）
}

// TimeServiceNTPDetailedResult NTP详细查询结果
type TimeServiceNTPDetailedResult struct {
	Stratum      int     // NTP层级
	PollInterval int     // 轮询间隔(秒)
	Reach        int     // 可达性
	Delay        float64 // 往返延迟(毫秒)
}

// TimeServiceStatus 时间服务状态
type TimeServiceStatus struct {
	IsInitialized bool      // 是否已初始化
	IsDegraded    bool      // 是否降级模式
	LastSyncTime  time.Time // 最后同步时间
}

// TimeServiceStats 时间服务统计
type TimeServiceStats struct {
	TotalSyncs      int64   // 总同步次数
	SuccessfulSyncs int64   // 成功同步次数
	FailedSyncs     int64   // 失败同步次数
	LastDeviation   float64 // 最后偏差
	MaxDeviation    int64   // 最大偏差
}

// TimeServiceCircuitBreakerState 熔断器状态
type TimeServiceCircuitBreakerState struct {
	IsOpen          bool      // 是否打开（熔断）
	FailureCount    int64     // 失败计数
	LastFailureTime time.Time // 最后失败时间
	SuccessCount    int64     // 成功计数
}

// NewTimeService 创建新的时间服务实例
func NewTimeService(_config config.Config) *TimeService {
	// 转换NTP服务器配置类型
	var ntpServer []TimeServiceNTPServer
	for _, server := range _config.NTPServer {
		ntpServer = append(ntpServer, TimeServiceNTPServer{
			Name:         server.Name,
			Address:      server.Address,
			Weight:       server.Weight,
			IsDomestic:   server.IsDomestic,
			MaxDeviation: server.MaxDeviation,
			IsSelected:   false, // 初始化为未选中
		})
	}

	return &TimeService{
		config: TimeServiceConfig{
			SyncInterval:     _config.TimeService.SyncInterval,
			MaxDeviation:     _config.TimeService.MaxDeviation,
			FailureThreshold: _config.TimeService.FailureThreshold,
			RecoveryTimeout:  _config.TimeService.RecoveryTimeout,
		},
		ntpServers:     ntpServer,
		syncTimeOffset: 0,
		status: TimeServiceStatus{
			IsInitialized: false,
		},
		circuitBreaker: TimeServiceCircuitBreakerState{
			IsOpen: false,
		},
		lastNTPSamples: make(map[string][]TimeServiceNTPSample),
	}
}

// Init 初始化时间服务
func (ts *TimeService) Init() error {
	logger.Info("TimeService", "初始化金融级时间服务系统...\n")
	fmt.Printf("初始化金融级时间服务系统...\n")

	// 1. 记录单调时钟起点（仅初始化一次）
	ts.monotonicTimeStart = time.Now()

	// 2. 同步多源NTP获取初始基准时间
	trustedBaseTime, err := ts.syncMultiNTPTime()

	// 无论成功还是失败，都要更新总同步计数
	atomic.AddInt64(&ts.stats.TotalSyncs, 1)

	if err != nil {
		// 首次同步失败
		atomic.AddInt64(&ts.stats.FailedSyncs, 1)
		logger.Info("TimeService", fmt.Sprintf("初始化NTP同步失败: %v\n", err))
		fmt.Printf("初始化NTP同步失败: %v\n", err)
		return fmt.Errorf("初始化NTP同步失败: %v", err)
	}

	// 计算初始偏移量
	initialOffset := trustedBaseTime.UnixNano() - ts.monotonicTimeStart.UnixNano()
	atomic.StoreInt64(&ts.syncTimeOffset, initialOffset)

	// 更新统计计数器 - 首次同步成功
	atomic.AddInt64(&ts.stats.SuccessfulSyncs, 1)

	// 4. 更新状态
	ts.status.IsInitialized = true
	ts.status.IsDegraded = false
	ts.status.LastSyncTime = time.Now()

	logger.Info("TimeService", fmt.Sprintf("时间服务初始化成功，初始偏移量: %.7f s\n", float64(initialOffset)/1e9))

	// 5. 启动定时NTP同步
	go ts.startNTPSyncLoop()

	return nil
}

// GetTrustedTimestamp 获取金融级可信时间戳（纳秒级，抗篡改）
func (ts *TimeService) GetTrustedTimestamp() int64 {
	// 单调时钟当前时间（不受系统时间修改影响）
	monotonicNow := time.Now().UnixNano()

	// 叠加可信偏移量，得到绝对时间戳
	return monotonicNow + atomic.LoadInt64(&ts.syncTimeOffset)
}

// GetTrustedTime 获取格式化的可信时间
func (ts *TimeService) GetTrustedTime() time.Time {
	timestamp := ts.GetTrustedTimestamp()
	return time.Unix(0, timestamp)
}

// GetStatus 获取时间服务状态
func (ts *TimeService) GetStatus() TimeServiceStatus {
	return ts.status
}

// GetStats 获取时间服务统计信息
func (ts *TimeService) GetStats() TimeServiceStats {
	return ts.stats
}

// GetCircuitBreakerState 获取熔断器状态
func (ts *TimeService) GetCircuitBreakerState() TimeServiceCircuitBreakerState {
	return ts.circuitBreaker
}

// startNTPSyncLoop 启动NTP同步循环
func (ts *TimeService) startNTPSyncLoop() {
	ticker := time.NewTicker(ts.config.SyncInterval)
	defer ticker.Stop()

	for range ticker.C {
		ts.syncWithRetry()
	}
}

// syncWithRetry 带重试的同步
func (ts *TimeService) syncWithRetry() {
	// 检查熔断器状态
	if ts.circuitBreaker.IsOpen {
		// 检查是否可以尝试恢复
		if time.Since(ts.circuitBreaker.LastFailureTime) > ts.config.RecoveryTimeout {
			logger.Info("TimeService", "尝试从熔断状态恢复...\n")
			ts.circuitBreaker.IsOpen = false
			ts.circuitBreaker.FailureCount = 0
		} else {
			// 仍在熔断状态，跳过本次同步
			return
		}
	}

	// 执行同步
	err := ts.updateOffset()
	if err != nil {
		logger.Info("TimeService", fmt.Sprintf("NTP同步失败: %v\n", err))
		atomic.AddInt64(&ts.stats.FailedSyncs, 1)
		atomic.AddInt64(&ts.circuitBreaker.FailureCount, 1)
		ts.circuitBreaker.LastFailureTime = time.Now()

		// 检查是否需要熔断
		if ts.circuitBreaker.FailureCount >= ts.config.FailureThreshold {
			logger.Info("TimeService", "NTP同步失败次数过多，触发熔断\n")
			ts.circuitBreaker.IsOpen = true
			ts.status.IsDegraded = true
		}
	} else {
		// 同步成功
		atomic.AddInt64(&ts.stats.SuccessfulSyncs, 1)
		atomic.AddInt64(&ts.circuitBreaker.SuccessCount, 1)
		ts.status.LastSyncTime = time.Now()

		// 如果之前是降级模式，现在恢复
		if ts.status.IsDegraded {
			logger.Info("TimeService", "时间服务已从降级模式恢复\n")
			ts.status.IsDegraded = false
		}

		// 重置熔断器计数器
		if ts.circuitBreaker.FailureCount > 0 {
			ts.circuitBreaker.FailureCount = 0
		}
	}

	atomic.AddInt64(&ts.stats.TotalSyncs, 1)
}

// updateOffset 更新时间偏移量
func (ts *TimeService) updateOffset() error {
	// 获取可信基准时间
	trustedBaseTime, err := ts.syncMultiNTPTime()
	if err != nil {
		return err
	}

	// 计算新的偏移量
	newOffset := trustedBaseTime.UnixNano() - ts.monotonicTimeStart.UnixNano()
	currentOffset := atomic.LoadInt64(&ts.syncTimeOffset)

	// 检查偏移量变化是否在合理范围内
	offsetChange := newOffset - currentOffset
	if math.Abs(float64(offsetChange)) > float64(ts.config.MaxDeviation)*5 {
		// 偏差过大，可能存在时间攻击
		logger.Info("TimeService", fmt.Sprintf("检测到异常时间偏移: %d ns，可能存在时间攻击\n", offsetChange))
		return fmt.Errorf("检测到异常时间偏移: %d ns", offsetChange)
	}

	// 更新偏移量
	atomic.StoreInt64(&ts.syncTimeOffset, newOffset)

	return nil
}

// syncMultiNTPTime 多源NTP同步，每个服务器获取5个样本，优先选择5个样本都成功的服务器
func (ts *TimeService) syncMultiNTPTime() (time.Time, error) {
	logger.Info("TimeService", "开始多源NTP同步（每个服务器获取5个样本，优先选择5个样本都成功的服务器）...\n")

	var bestResult *TimeServiceNTPTimeResult
	var bestSuccessCount int = 0 // 最佳成功样本数
	maxDeviation := ts.config.MaxDeviation

	// 从所有NTP服务器获取时间，寻找成功率最高的服务器
	for _, server := range ts.ntpServers {
		result, err := ts.queryNTPServer(server)
		if err != nil {
			logger.Info("TimeService", fmt.Sprintf("查询NTP服务器 %s 失败: %v\n", server.Address, err))
			continue
		}

		// 检查偏差是否在允许范围内
		if math.Abs(result.Deviation) > float64(server.MaxDeviation) {
			logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 偏差过大: %.2f ms (样本数: %d)\n",
				server.Address, result.Deviation/1e6, result.SampleCount))
			continue
		}

		// 记录采样结果
		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样成功，样本数: %d, 成功样本数: %d, 偏差: %.2f ms, 往返时间: %.2f ms\n",
			server.Address, result.SampleCount, result.SuccessCount, result.Deviation/1e6, result.RTT/1e6))

		// 如果找到5个样本都成功的服务器，立即使用它并停止搜索
		if result.SuccessCount == 5 {
			bestResult = &result
			logger.Info("TimeService", fmt.Sprintf("找到5个样本都成功的NTP服务器 %s，立即使用\n", server.Address))
			break
		}

		// 否则，记录当前最好的服务器（成功样本数最多的）
		if result.SuccessCount > bestSuccessCount {
			bestResult = &result
			bestSuccessCount = result.SuccessCount
		}
	}

	// 检查是否找到有效的NTP服务器
	if bestResult == nil {
		logger.Info("TimeService", "多源NTP同步失败，没有找到有效的NTP服务器\n")
		return time.Time{}, fmt.Errorf("没有找到有效的NTP服务器")
	}

	// 使用找到的最佳服务器结果
	trustedTime := time.Unix(0, bestResult.Timestamp)

	// 标记选中的服务器
	for i, server := range ts.ntpServers {
		if server.Address == bestResult.Server {
			ts.ntpServers[i].IsSelected = true
		} else {
			ts.ntpServers[i].IsSelected = false
		}
	}

	// 更新统计信息
	ts.stats.LastDeviation = bestResult.Deviation
	if int64(bestResult.Deviation) > ts.stats.MaxDeviation {
		ts.stats.MaxDeviation = int64(bestResult.Deviation)
	}

	// 检测与本地历史基准的偏差，超出阈值触发告警
	currentTime := ts.GetTrustedTime()
	deviation := math.Abs(float64(trustedTime.UnixNano() - currentTime.UnixNano()))
	if deviation > float64(maxDeviation)*5 { // 偏差超阈值，触发入侵告警
		logger.Info("TimeService", fmt.Sprintf("NTP时间异常跳变: %.2f ms，可能存在入侵风险\n", deviation/1e6))
	}

	logger.Info("TimeService", fmt.Sprintf("NTP同步完成，使用服务器 %s，成功样本数: %d, 偏差: %.2f ms, 往返时间: %.2f ms\n",
		bestResult.Server, bestResult.SuccessCount, bestResult.Deviation/1e6, bestResult.RTT/1e6))
	return trustedTime, nil
}

// queryNTPServer 查询单个NTP服务器，获取5个样本，最少连续3次成功就算成功
func (ts *TimeService) queryNTPServer(server TimeServiceNTPServer) (TimeServiceNTPTimeResult, error) {
	var samples []TimeServiceNTPSample
	const sampleCount = 5
	const successThreshold = 3 // 最少连续3个成功就算成功
	var consecutiveSuccesses = 0
	var failedAttempts = 0
	var invalidStratumAttempts = 0
	var hasMetThreshold = false // 标记是否已达到最少连续成功阈值

	// 获取5个样本，不管成功失败都必须获取5个样本
	for i := 0; i < sampleCount; i++ {
		resp, err := ntp.Query(server.Address)
		if err != nil {
			failedAttempts++
			consecutiveSuccesses = 0 // 重置连续成功计数

			// 添加失败样本，状态为"失败"
			samples = append(samples, TimeServiceNTPSample{
				Timestamp: time.Now().UnixNano(), // 使用当前时间作为时间戳
				RTT:       0,                     // 失败时RTT为0
				Deviation: 0,                     // 失败时偏差为0
				Status:    "Failed",              // 设置状态为失败
			})
			continue
		}

		if resp.Stratum == 0 { // Stratum 0为无效源
			invalidStratumAttempts++
			consecutiveSuccesses = 0 // 重置连续成功计数

			// 添加无效源样本，状态为"失败"
			samples = append(samples, TimeServiceNTPSample{
				Timestamp: time.Now().UnixNano(), // 使用当前时间作为时间戳
				RTT:       resp.RTT.Nanoseconds(),
				Deviation: 0,        // 无效源时偏差为0
				Status:    "Failed", // 设置状态为失败
			})
			continue
		}

		// 计算偏差
		localTime := time.Now()
		deviation := math.Abs(float64(resp.Time.UnixNano() - localTime.UnixNano()))

		// 添加成功样本，状态为"成功"
		samples = append(samples, TimeServiceNTPSample{
			Timestamp: resp.Time.UnixNano(),
			RTT:       resp.RTT.Nanoseconds(),
			Deviation: deviation,
			Status:    "Success", // 设置状态为成功
		})

		consecutiveSuccesses++

		// 检查是否已达到最少连续成功阈值
		if consecutiveSuccesses >= successThreshold {
			hasMetThreshold = true
		}

		// 短暂延迟，避免连续查询
		time.Sleep(50 * time.Millisecond)
	}

	// 保存样本数据到lastNTPSamples字段（即使样本数量不足也要保存）
	ts.lastNTPSamples[server.Address] = samples

	// 计算成功样本数
	successCount := 0
	for _, sample := range samples {
		if sample.Status == "Success" {
			successCount++
		}
	}

	// 选择最佳样本用于时间计算
	// 优先选择RTT最小的成功样本
	if len(samples) > 0 {
	// 按RTT排序
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].RTT < samples[j].RTT
	})
	}

	// 按时间戳排序样本
	sort.Slice(samples, func(i, j int) bool { return samples[i].Timestamp < samples[j].Timestamp })

	// 初始化变量，确保在所有代码路径中都有定义
	var medianTimestamp int64
	var lastSuccessDeviation float64 // 修改：使用最后一个成功样本的偏差
	var lastSuccessRTT int64         // 修改：使用最后一个成功样本的RTT
	var rtt float64                  // 修改：重命名变量，表示往返时间

	// 记录采样完成后的综合日志，包含失败和无效源统计
	if len(samples) > 0 {
		// 计算聚合结果
		var foundSuccessSample bool

		// 计算中位数时间戳（更稳健的估计）
		medianTimestamp = samples[len(samples)/2].Timestamp

		// 查找最后一个成功样本的偏差和RTT
		for i := len(samples) - 1; i >= 0; i-- {
			if samples[i].Status == "Success" {
				lastSuccessDeviation = samples[i].Deviation
				lastSuccessRTT = samples[i].RTT
				foundSuccessSample = true
				break
			}
		}

		// 如果没有找到成功样本，使用最后一个样本的偏差和RTT
		if !foundSuccessSample {
			lastSuccessDeviation = samples[len(samples)-1].Deviation
			lastSuccessRTT = samples[len(samples)-1].RTT
		}

		// 修改：使用最后成功样本作为往返时间
		rtt = float64(lastSuccessRTT)

		// 记录样本列表信息
		sampleInfo := "样本列表: "
		for i, sample := range samples {
			sampleInfo += fmt.Sprintf("[序号%d, 偏差%.2fms, RTT%.2fms, 状态%s] ",
				i+1, sample.Deviation/1e6, float64(sample.RTT)/1e6, sample.Status)
		}

		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样完成，有效样本数: %d, 偏差: %.2f ms, 往返时间: %.2f ms, 失败次数: %d, 无效源次数: %d\n%s\n",
			server.Address, len(samples), lastSuccessDeviation/1e6, rtt/1e6, failedAttempts, invalidStratumAttempts, sampleInfo))
	} else {
		// 即使没有有效样本也要记录
		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样完成，有效样本数: 0, 失败次数: %d, 无效源次数: %d\n",
			server.Address, failedAttempts, invalidStratumAttempts))
	}

	// 检查是否有足够的有效样本并且满足最少连续成功阈值
	if len(samples) < 2 || !hasMetThreshold {
		// 即使样本不足或不满足阈值，也要返回结果（但标记为无效）
		if len(samples) == 1 {
			// 只有一个样本的情况
			// 修改：使用样本的RTT+偏差作为RTT
			sampleRTTPlusDeviation := float64(samples[0].RTT) + samples[0].Deviation
			return TimeServiceNTPTimeResult{
				Timestamp:    samples[0].Timestamp,
				Deviation:    samples[0].Deviation,
				Weight:       server.Weight,
				Server:       server.Address,
				SampleCount:  1,
				SuccessCount: successCount,
				RTT:          sampleRTTPlusDeviation,
			}, fmt.Errorf("有效样本不足或未达到最少连续成功阈值，只有 %d 个有效样本", len(samples))
		}
		return TimeServiceNTPTimeResult{}, fmt.Errorf("有效样本不足或未达到最少连续成功阈值，只有 %d 个有效样本", len(samples))
	}

	return TimeServiceNTPTimeResult{
		Timestamp:    medianTimestamp,      // 使用中位数作为最终时间戳
		Deviation:    lastSuccessDeviation, // 修改：使用最后一个成功样本的偏差
		Weight:       server.Weight,
		Server:       server.Address,
		SampleCount:  len(samples),
		SuccessCount: successCount,
		RTT:          rtt,
	}, nil
}

// QueryNTPServerDetailed 查询单个NTP服务器的详细信息
func (ts *TimeService) QueryNTPServerDetailed(server TimeServiceNTPServer) (TimeServiceNTPDetailedResult, error) {
	resp, err := ntp.Query(server.Address)
	if err != nil {
		return TimeServiceNTPDetailedResult{}, err
	}

	if int(resp.Stratum) == 0 { // Stratum 0为无效源
		return TimeServiceNTPDetailedResult{}, fmt.Errorf("无效的NTP源，Stratum为0")
	}

	// 计算往返延迟
	delay := float64(resp.RTT) / float64(time.Millisecond) // 转换为毫秒

	// 计算可达性
	reach := 377 // 默认值，表示最近8次查询都成功

	return TimeServiceNTPDetailedResult{
		Stratum:      int(resp.Stratum),
		PollInterval: 64, // 默认轮询间隔
		Reach:        reach,
		Delay:        delay,
	}, nil
}

// SetCustomNTPServers 设置自定义NTP服务器列表
func (ts *TimeService) SetCustomNTPServers(servers []TimeServiceNTPServer) {
	ts.ntpServers = servers
}

// SetConfig 设置时间服务配置
func (ts *TimeService) SetConfig(config TimeServiceConfig) {
	ts.config = config
}

// IsInDegradedMode 检查是否处于降级模式
func (ts *TimeService) IsInDegradedMode() bool {
	return ts.status.IsDegraded
}

// GetSyncTimeOffset 获取当前时间偏移量
func (ts *TimeService) GetSyncTimeOffset() int64 {
	return atomic.LoadInt64(&ts.syncTimeOffset)
}

// GetNTPServers 获取NTP服务器列表
func (ts *TimeService) GetNTPServers() []TimeServiceNTPServer {
	return ts.ntpServers
}

// 全局时间服务实例
var globalTimeService *TimeService

// InitGlobalTimeService 初始化全局时间服务并返回实例
func InitGlobalTimeService(_config config.Config) (*TimeService, error) {
	// 使用获取的NTP服务器配置初始化时间服务
	globalTimeService = NewTimeService(_config)
	err := globalTimeService.Init()
	if err != nil {
		return nil, err
	}
	return globalTimeService, nil
}

// GetTrustedTimestamp 获取全局金融级可信时间戳
func GetTrustedTimestamp() int64 {
	if globalTimeService == nil {
		// 如果时间服务未初始化，返回系统时间（降级模式）
		return time.Now().UnixNano()
	}
	return globalTimeService.GetTrustedTimestamp()
}

// GetTrustedTime 获取全局格式化的可信时间
func GetTrustedTime() time.Time {
	if globalTimeService == nil {
		// 如果时间服务未初始化，返回系统时间（降级模式）
		return time.Now()
	}
	return globalTimeService.GetTrustedTime()
}

// GetLastNTPSamples 获取上一次获取的NTP样本数据
func (ts *TimeService) GetLastNTPSamples() map[string][]TimeServiceNTPSample {
	return ts.lastNTPSamples
}
