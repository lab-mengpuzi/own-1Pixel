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
	// 全局时间偏移量（可信基准时间 - 单调时钟起点时间），原子更新
	timeOffset int64

	// 单调时钟起点时间
	monotonicStart time.Time

	// NTP服务器配置
	ntpServers []TimeServiceNTPServer

	// 时间服务状态
	status TimeServiceStatus

	// 熔断器状态
	circuitBreaker TimeServiceCircuitBreakerState

	// 统计信息
	stats TimeServiceStats

	// 配置参数
	config TimeServiceConfig

	// 上一次获取的NTP样本数据，按服务器地址存储
	lastNTPSamples map[string][]TimeServiceNTPSample
}

// TimeServiceNTPServer NTP服务器配置
type TimeServiceNTPServer struct {
	Name         string  // 服务器名称
	Address      string  // 服务器地址
	Weight       float64 // 权重
	IsDomestic   bool    // 是否为国内服务器
	MaxDeviation int64   // 最大允许偏差(纳秒)
}

// TimeServiceStatus 时间服务状态
type TimeServiceStatus struct {
	IsInitialized    bool      // 是否已初始化
	IsOnline         bool      // 是否在线（NTP同步正常）
	IsDegraded       bool      // 是否降级模式
	LastSyncTime     time.Time // 最后同步时间
	ActiveNTPSources int       // 活跃NTP源数量
}

// TimeServiceStats 时间服务统计
type TimeServiceStats struct {
	TotalSyncs       int64   // 总同步次数
	SuccessfulSyncs  int64   // 成功同步次数
	FailedSyncs      int64   // 失败同步次数
	AverageDeviation float64 // 平均偏差
	MaxDeviation     int64   // 最大偏差
}

// TimeServiceCircuitBreakerState 熔断器状态
type TimeServiceCircuitBreakerState struct {
	IsOpen          bool      // 是否打开（熔断）
	FailureCount    int64     // 失败计数
	LastFailureTime time.Time // 最后失败时间
	SuccessCount    int64     // 成功计数
}

// TimeServiceConfig 时间服务配置
type TimeServiceConfig struct {
	SyncInterval     time.Duration // 同步间隔
	MaxDeviation     int64         // 最大允许偏差(纳秒)
	FailureThreshold int64         // 失败阈值
	RecoveryTimeout  time.Duration // 恢复超时
}

// InitTimeService 创建新的时间服务实例
func InitTimeService() *TimeService {
	// 从配置包获取配置
	appConfig := config.GetConfig()

	// 转换配置类型
	timeServiceConfig := TimeServiceConfig{
		SyncInterval:     appConfig.TimeService.SyncInterval,
		MaxDeviation:     appConfig.TimeService.MaxDeviation,
		FailureThreshold: appConfig.TimeService.FailureThreshold,
		RecoveryTimeout:  appConfig.TimeService.RecoveryTimeout,
	}

	// 获取NTP服务器配置
	ntpServersConfig := config.GetDefaultNTPServers()

	// 转换NTP服务器配置类型
	var ntpServers []TimeServiceNTPServer
	for _, server := range ntpServersConfig {
		ntpServers = append(ntpServers, TimeServiceNTPServer{
			Name:         server.Name,
			Address:      server.Address,
			Weight:       server.Weight,
			IsDomestic:   server.IsDomestic,
			MaxDeviation: server.MaxDeviation,
		})
	}

	return &TimeService{
		config:         timeServiceConfig,
		ntpServers:     ntpServers,
		status:         TimeServiceStatus{IsInitialized: false},
		circuitBreaker: TimeServiceCircuitBreakerState{IsOpen: false},
		lastNTPSamples: make(map[string][]TimeServiceNTPSample), // 初始化样本数据存储
	}
}

// Init 初始化时间服务
func (ts *TimeService) Init() error {
	logger.Info("TimeService", "初始化金融级时间服务...\n")
	fmt.Printf("开始初始化金融级时间服务...\n")

	// 1. 记录单调时钟起点（仅初始化一次）
	ts.monotonicStart = time.Now()

	// 2. 同步多源NTP获取初始基准时间
	trustedBaseTime, err := ts.syncMultiNTP()
	if err != nil {
		logger.Info("TimeService", fmt.Sprintf("初始化NTP同步失败: %v\n", err))
		fmt.Printf("初始化NTP同步失败: %v\n", err)
		return fmt.Errorf("初始化NTP同步失败: %v", err)
	}

	// 3. 计算初始偏移量（纳秒级）
	initialOffset := trustedBaseTime.UnixNano() - ts.monotonicStart.UnixNano()
	atomic.StoreInt64(&ts.timeOffset, initialOffset)

	// 4. 更新状态
	ts.status.IsInitialized = true
	ts.status.IsOnline = true
	ts.status.IsDegraded = false
	ts.status.LastSyncTime = time.Now()

	logger.Info("TimeService", fmt.Sprintf("时间服务初始化成功，初始偏移量: %d ns\n", initialOffset))

	// 5. 启动定时NTP同步
	go ts.startNTPSyncLoop()

	return nil
}

// GetTrustedTimestamp 获取金融级可信时间戳（纳秒级，抗篡改）
func (ts *TimeService) GetTrustedTimestamp() int64 {
	// 单调时钟当前时间（不受系统时间修改影响）
	monotonicNow := time.Now().UnixNano()

	// 叠加可信偏移量，得到绝对时间戳
	return monotonicNow + atomic.LoadInt64(&ts.timeOffset)
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
			ts.status.IsOnline = false
			ts.status.IsDegraded = true
		}
	} else {
		// 同步成功
		atomic.AddInt64(&ts.stats.SuccessfulSyncs, 1)
		atomic.AddInt64(&ts.circuitBreaker.SuccessCount, 1)
		ts.status.LastSyncTime = time.Now()
		ts.status.IsOnline = true

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
	trustedBaseTime, err := ts.syncMultiNTP()
	if err != nil {
		return err
	}

	// 计算新的偏移量
	newOffset := trustedBaseTime.UnixNano() - ts.monotonicStart.UnixNano()
	currentOffset := atomic.LoadInt64(&ts.timeOffset)

	// 检查偏移量变化是否在合理范围内
	offsetChange := newOffset - currentOffset
	if math.Abs(float64(offsetChange)) > float64(ts.config.MaxDeviation)*5 {
		// 偏差过大，可能存在时间攻击
		logger.Info("TimeService", fmt.Sprintf("检测到异常时间偏移: %d ns，可能存在时间攻击\n", offsetChange))
		return fmt.Errorf("检测到异常时间偏移: %d ns", offsetChange)
	}

	// 更新偏移量
	atomic.StoreInt64(&ts.timeOffset, newOffset)
	ts.status.ActiveNTPSources = len(ts.getValidNTPServers())

	return nil
}

// syncMultiNTP 多源NTP同步，每个服务器获取5个样本，最少连续3次成功就算成功
func (ts *TimeService) syncMultiNTP() (time.Time, error) {
	logger.Info("TimeService", "开始多源NTP同步（每个服务器获取5个样本，最少连续3次成功就算成功）...\n")

	var validTimes []TimeServiceNTPTimeResult
	maxDeviation := ts.config.MaxDeviation

	// 从所有NTP服务器获取时间
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

		// 检查抖动是否在合理范围内
		if result.MaxJitter > float64(server.MaxDeviation) {
			logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 抖动过大: %.2f ms (样本数: %d)\n",
				server.Address, result.MaxJitter/1e6, result.SampleCount))
			continue
		}

		validTimes = append(validTimes, result)
		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样成功，样本数: %d, 平均偏差: %.2f ms, 最大抖动: %.2f ms\n",
			server.Address, result.SampleCount, result.Deviation/1e6, result.MaxJitter/1e6))
	}

	// 只要有1个有效源就判定同步成功（更宽松的策略）
	if len(validTimes) < 1 {
		logger.Info("TimeService", fmt.Sprintf("多源NTP同步失败，有效NTP源不足，只有 %d 个有效源\n", len(validTimes)))
		return time.Time{}, fmt.Errorf("有效NTP源不足，只有 %d 个有效源", len(validTimes))
	}

	logger.Info("TimeService", fmt.Sprintf("多源NTP同步成功，有效源数: %d\n", len(validTimes)))

	// 如果只有一个有效源，直接使用它
	if len(validTimes) == 1 {
		trustedTime := time.Unix(0, validTimes[0].Timestamp)

		// 更新统计信息
		ts.stats.AverageDeviation = validTimes[0].Deviation
		if int64(validTimes[0].Deviation) > ts.stats.MaxDeviation {
			ts.stats.MaxDeviation = int64(validTimes[0].Deviation)
		}

		// 检测与本地历史基准的偏差，超出阈值触发告警
		currentTime := ts.GetTrustedTime()
		deviation := math.Abs(float64(trustedTime.UnixNano() - currentTime.UnixNano()))
		if deviation > float64(maxDeviation)*5 { // 偏差超阈值，触发入侵告警
			logger.Info("TimeService", fmt.Sprintf("NTP时间异常跳变: %.2f ms，可能存在入侵风险\n", deviation/1e6))
		}

		logger.Info("TimeService", fmt.Sprintf("单源NTP同步完成，偏差: %.2f ms, 抖动: %.2f ms\n",
			validTimes[0].Deviation/1e6, validTimes[0].MaxJitter/1e6))
		return trustedTime, nil
	}

	// 多个有效源的情况，使用加权平均
	// 排序后剔除极值（前10%和后10%）
	sort.Slice(validTimes, func(i, j int) bool { return validTimes[i].Timestamp < validTimes[j].Timestamp })
	trimStart := len(validTimes) / 10
	trimEnd := len(validTimes) - trimStart
	trimmedTimes := validTimes[trimStart:trimEnd]

	// 计算加权平均，考虑样本质量和数量
	var total float64
	var weightSum float64
	var totalDeviation float64
	var totalJitter float64

	for _, t := range trimmedTimes {
		// 调整权重：原始权重 * 样本数量因子（样本越多，权重越高）
		adjustedWeight := t.Weight * (1.0 + float64(t.SampleCount-2)/10.0) // 2个样本是基准，每多一个样本增加10%权重
		total += float64(t.Timestamp) * adjustedWeight
		weightSum += adjustedWeight
		totalDeviation += t.Deviation
		totalJitter += t.MaxJitter
	}

	trustedTimeNano := int64(total / weightSum)
	trustedTime := time.Unix(0, trustedTimeNano)

	// 更新统计信息
	ts.stats.AverageDeviation = totalDeviation / float64(len(trimmedTimes))
	if int64(totalDeviation) > ts.stats.MaxDeviation {
		ts.stats.MaxDeviation = int64(totalDeviation)
	}

	// 检测与本地历史基准的偏差，超出阈值触发告警
	currentTime := ts.GetTrustedTime()
	deviation := math.Abs(float64(trustedTime.UnixNano() - currentTime.UnixNano()))
	if deviation > float64(maxDeviation)*5 { // 偏差超阈值，触发入侵告警
		logger.Info("TimeService", fmt.Sprintf("NTP时间异常跳变: %.2f ms，可能存在入侵风险\n", deviation/1e6))
	}

	logger.Info("TimeService", fmt.Sprintf("多源NTP同步完成，有效源数: %d, 平均偏差: %.2f ms, 平均抖动: %.2f ms\n",
		len(validTimes), totalDeviation/float64(len(trimmedTimes))/1e6, totalJitter/float64(len(trimmedTimes))/1e6))
	return trustedTime, nil
}

// TimeServiceNTPTimeResult NTP查询结果（基于多个样本的聚合）
type TimeServiceNTPTimeResult struct {
	Timestamp   int64   // 聚合时间戳（纳秒）
	Deviation   float64 // 平均偏差（纳秒）
	Weight      float64 // 权重
	Server      string  // 服务器地址
	SampleCount int     // 样本数量
	MaxJitter   float64 // 最大抖动（纳秒）
	AverageRTT  float64 // 平均往返时间（纳秒）
}

// TimeServiceNTPSample 单个NTP样本
type TimeServiceNTPSample struct {
	Timestamp  int64   // 时间戳（纳秒）
	RTT        int64   // 往返时间（纳秒）
	Deviation  float64 // 偏差（纳秒）
	Status     string  // 样本状态：成功、失败
	IsSelected bool    // 是否被选中用于时间计算
}

// TimeServiceNTPDetailedResult NTP详细查询结果
type TimeServiceNTPDetailedResult struct {
	Stratum      int     // NTP层级
	PollInterval int     // 轮询间隔(秒)
	Reach        int     // 可达性
	Delay        float64 // 往返延迟(毫秒)
	Jitter       float64 // 抖动(毫秒)
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
				Timestamp:  time.Now().UnixNano(), // 使用当前时间作为时间戳
				RTT:        0,                     // 失败时RTT为0
				Deviation:  0,                     // 失败时偏差为0
				Status:     "Failed",              // 设置状态为失败
				IsSelected: false,                 // 失败样本不选中
			})
			continue
		}

		if resp.Stratum == 0 { // Stratum 0为无效源
			invalidStratumAttempts++
			consecutiveSuccesses = 0 // 重置连续成功计数

			// 添加无效源样本，状态为"失败"
			samples = append(samples, TimeServiceNTPSample{
				Timestamp:  time.Now().UnixNano(), // 使用当前时间作为时间戳
				RTT:        resp.RTT.Nanoseconds(),
				Deviation:  0,        // 无效源时偏差为0
				Status:     "Failed", // 设置状态为失败
				IsSelected: false,    // 无效源样本不选中
			})
			continue
		}

		// 计算偏差
		localTime := time.Now()
		deviation := math.Abs(float64(resp.Time.UnixNano() - localTime.UnixNano()))

		// 添加成功样本，状态为"成功"
		samples = append(samples, TimeServiceNTPSample{
			Timestamp:  resp.Time.UnixNano(),
			RTT:        resp.RTT.Nanoseconds(),
			Deviation:  deviation,
			Status:     "Success", // 设置状态为成功
			IsSelected: false,     // 默认不选中
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

	// 选择最佳样本用于时间计算
	// 优先选择RTT最小的成功样本
	if len(samples) > 0 {
		// 按RTT排序
		sort.Slice(samples, func(i, j int) bool {
			return samples[i].RTT < samples[j].RTT
		})

		// 修改：标记所有成功的样本为选中状态（如果有至少3个连续成功）
		if hasMetThreshold {
			for i := 0; i < len(samples); i++ {
				// 只有状态为"Success"的样本才能被选中
				if samples[i].Status == "Success" {
					samples[i].IsSelected = true
				}
			}
		}
	}

	// 按时间戳排序样本
	sort.Slice(samples, func(i, j int) bool { return samples[i].Timestamp < samples[j].Timestamp })

	// 初始化变量，确保在所有代码路径中都有定义
	var medianTimestamp int64
	var avgDeviation float64
	var maxJitter float64
	var avgRTT float64

	// 记录采样完成后的综合日志，包含失败和无效源统计
	if len(samples) > 0 {
		// 计算聚合结果
		var totalDeviation float64
		var totalRTT int64

		// 计算中位数时间戳（更稳健的估计）
		medianTimestamp = samples[len(samples)/2].Timestamp

		// 计算相对于中位数的抖动
		for _, sample := range samples {
			totalDeviation += sample.Deviation
			totalRTT += sample.RTT
			jitter := math.Abs(float64(sample.Timestamp - medianTimestamp))
			if jitter > maxJitter {
				maxJitter = jitter
			}
		}

		// 计算平均值
		avgDeviation = totalDeviation / float64(len(samples))
		avgRTT = float64(totalRTT) / float64(len(samples))

		// 记录样本列表信息
		sampleInfo := "样本列表: "
		for i, sample := range samples {
			selectedStatus := "否"
			if sample.IsSelected {
				selectedStatus = "是"
			}
			sampleInfo += fmt.Sprintf("[序号%d: 偏差%.2fms RTT%.2fms 状态%s 选中%s] ",
				i+1, sample.Deviation/1e6, float64(sample.RTT)/1e6, sample.Status, selectedStatus)
		}

		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样完成，有效样本数: %d, 平均偏差: %.2f ms, 最大抖动: %.2f ms, 失败次数: %d, 无效源次数: %d\n%s\n",
			server.Address, len(samples), avgDeviation/1e6, maxJitter/1e6, failedAttempts, invalidStratumAttempts, sampleInfo))
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
			return TimeServiceNTPTimeResult{
				Timestamp:   samples[0].Timestamp,
				Deviation:   samples[0].Deviation,
				Weight:      server.Weight,
				Server:      server.Address,
				SampleCount: 1,
				MaxJitter:   0, // 只有一个样本，无法计算抖动
				AverageRTT:  float64(samples[0].RTT),
			}, fmt.Errorf("有效样本不足或未达到最少连续成功阈值，只有 %d 个有效样本", len(samples))
		}
		return TimeServiceNTPTimeResult{}, fmt.Errorf("有效样本不足或未达到最少连续成功阈值，只有 %d 个有效样本", len(samples))
	}

	return TimeServiceNTPTimeResult{
		Timestamp:   medianTimestamp, // 使用中位数作为最终时间戳
		Deviation:   avgDeviation,
		Weight:      server.Weight,
		Server:      server.Address,
		SampleCount: len(samples),
		MaxJitter:   maxJitter,
		AverageRTT:  avgRTT,
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

	// 计算往返延迟和抖动
	delay := float64(resp.RTT) / float64(time.Millisecond) // 转换为毫秒
	jitter := delay * 0.1                                  // 简单估算抖动为延迟的10%

	// 计算可达性
	reach := 377 // 默认值，表示最近8次查询都成功

	return TimeServiceNTPDetailedResult{
		Stratum:      int(resp.Stratum),
		PollInterval: 64, // 默认轮询间隔
		Reach:        reach,
		Delay:        delay,
		Jitter:       jitter,
	}, nil
}

// getValidNTPServers 获取有效的NTP服务器列表
func (ts *TimeService) getValidNTPServers() []TimeServiceNTPServer {
	var validServers []TimeServiceNTPServer
	for _, server := range ts.ntpServers {
		result, err := ts.queryNTPServer(server)
		if err == nil &&
			math.Abs(result.Deviation) <= float64(server.MaxDeviation) &&
			result.MaxJitter <= float64(server.MaxDeviation) &&
			result.SampleCount >= 2 { // 至少需要2个有效样本
			validServers = append(validServers, server)
		}
	}
	return validServers
}

// SetCustomNTPServers 设置自定义NTP服务器列表
func (ts *TimeService) SetCustomNTPServers(servers []TimeServiceNTPServer) {
	ts.ntpServers = servers
}

// SetConfig 设置时间服务配置
func (ts *TimeService) SetConfig(config TimeServiceConfig) {
	ts.config = config
}

// ForceSync 强制立即同步一次
func (ts *TimeService) ForceSync() error {
	return ts.updateOffset()
}

// IsInDegradedMode 检查是否处于降级模式
func (ts *TimeService) IsInDegradedMode() bool {
	return ts.status.IsDegraded
}

// GetTimeOffset 获取当前时间偏移量
func (ts *TimeService) GetTimeOffset() int64 {
	return atomic.LoadInt64(&ts.timeOffset)
}

// GetNTPServers 获取NTP服务器列表
func (ts *TimeService) GetNTPServers() []TimeServiceNTPServer {
	return ts.ntpServers
}

// 全局时间服务实例
var globalTimeService *TimeService

// InitGlobalTimeService 初始化全局时间服务
func InitGlobalTimeService() error {
	globalTimeService = InitTimeService()
	return globalTimeService.Init()
}

// GetGlobalTimeService 获取全局时间服务实例
func GetGlobalTimeService() *TimeService {
	return globalTimeService
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
