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
	FailureThreshold int64         // 失败阈值
	SampleCount      int           // 样本数量
	SampleDelay      time.Duration // 样本延迟
	MaxDeviation     int64         // 最大允许偏差(纳秒)
	SyncInterval     time.Duration // 同步间隔
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
func NewTimeService() *TimeService {
	// 获取全局配置实例
	_config := config.GetConfig()
	timeService := _config.TimeService

	// 转换NTP服务器配置类型
	var ntpServers []TimeServiceNTPServer
	for _, server := range timeService.NTPServers {
		ntpServers = append(ntpServers, TimeServiceNTPServer{
			Name:         server.Name,
			Address:      server.Address,
			Weight:       server.Weight,
			IsDomestic:   server.IsDomestic,
			MaxDeviation: server.MaxDeviation,
			IsSelected:   server.IsSelected,
		})
	}

	return &TimeService{
		config: TimeServiceConfig{
			FailureThreshold: timeService.FailureThreshold,
			SampleCount:      timeService.SampleCount,
			SampleDelay:      timeService.SampleDelay,
			SyncInterval:     timeService.SyncInterval,
			MaxDeviation:     timeService.MaxDeviation,
			RecoveryTimeout:  timeService.RecoveryTimeout,
		},
		ntpServers:     ntpServers,
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
	trustedBaseTime, err := ts.queryMultiSyncTime()

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
	trustedBaseTime, err := ts.queryMultiSyncTime()
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

// queryMultiSyncTime 多源NTP同步
func (ts *TimeService) queryMultiSyncTime() (time.Time, error) {
	logger.Info("TimeService", fmt.Sprintf("开始多源NTP同步（并行查询所有服务器，每个服务器获取%d个样本）...\n", ts.config.SampleCount))

	var bestResult *TimeServiceNTPTimeResult
	maxDeviation := ts.config.MaxDeviation

	// 使用通道和goroutine并行查询所有NTP服务器
	type serverResult struct {
		server TimeServiceNTPServer
		result TimeServiceNTPTimeResult
		err    error
	}

	resultChan := make(chan serverResult, len(ts.ntpServers))

	// 启动goroutine并行查询每个服务器
	for _, server := range ts.ntpServers {
		go func(s TimeServiceNTPServer) {
			result, err := ts.querySingleSyncTime(s)
			resultChan <- serverResult{server: s, result: result, err: err}
		}(server)
	}

	// 收集所有服务器的查询结果
	results := make([]serverResult, 0, len(ts.ntpServers))
	for i := 0; i < len(ts.ntpServers); i++ {
		resultChans := <-resultChan
		// 检查结果是否包含指定数量的样本
		if resultChans.err == nil && resultChans.result.SampleCount != ts.config.SampleCount {
			logger.Info("TimeService", fmt.Sprintf("警告: NTP服务器 %s 返回的样本数(%d)与配置的样本数(%d)不匹配\n",
				resultChans.server.Address, resultChans.result.SampleCount, ts.config.SampleCount))
		}
		results = append(results, resultChans)
	}

	// 按权重对所有有效服务器降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].server.Weight > results[j].server.Weight
	})

	// 分析结果，找到最佳服务器
	var validResults []serverResult // 存储所有有效的查询结果

	// 首先收集所有有效的查询结果
	for _, resultChans := range results {
		// 检查是否查询失败
		if resultChans.err != nil {
			logger.Info("TimeService", fmt.Sprintf("查询NTP服务器 %s 失败: %v\n", resultChans.server.Address, resultChans.err))
			continue
		}

		// 检查偏差是否在允许范围内
		if math.Abs(resultChans.result.Deviation) > float64(resultChans.server.MaxDeviation) {
			logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 偏差过大: %.2f ms (样本数: %d)\n",
				resultChans.server.Address, resultChans.result.Deviation/1e6, resultChans.result.SampleCount))
			continue
		}

		// 检测与本地历史基准的偏差，超出阈值则跳过该服务器
		trustedTime := time.Unix(0, resultChans.result.Timestamp)
		currentTime := ts.GetTrustedTime()
		deviation := math.Abs(float64(trustedTime.UnixNano() - currentTime.UnixNano()))
		if deviation > float64(maxDeviation)*5 { // 偏差超阈值，跳过该服务器
			logger.Info("TimeService", fmt.Sprintf("NTP时间异常跳变: %.2f s，跳过服务器 %s，可能存在入侵风险\n", deviation/1e9, resultChans.server.Address))
			continue
		}

		// 记录采样结果
		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样成功，样本数: %d, 成功样本数: %d, 偏差: %.2f ms, 往返时间: %.2f ms\n",
			resultChans.server.Address, resultChans.result.SampleCount, resultChans.result.SuccessCount, resultChans.result.Deviation/1e6, resultChans.result.RTT/1e6))

		// 添加到有效结果列表
		validResults = append(validResults, resultChans)
	}

	// 选择权重最高的服务器的最后一个样本计算可信时间
	if len(validResults) > 0 {
		bestServer := validResults[0].server
		bestResult = &validResults[0].result

		// 获取该服务器的最后一个样本
		if samples, exists := ts.lastNTPSamples[bestServer.Address]; exists && len(samples) > 0 {
			lastSample := samples[len(samples)-1]
			// 使用最后一个样本的时间戳作为可信时间
			bestResult.Timestamp = lastSample.Timestamp
			bestResult.Deviation = lastSample.Deviation
			bestResult.RTT = float64(lastSample.RTT)

			logger.Info("TimeService", fmt.Sprintf("选择权重最高的NTP服务器 %s（权重: %.1f），使用其最后一个样本计算可信时间\n",
				bestServer.Address, bestServer.Weight))
		} else {
			logger.Info("TimeService", fmt.Sprintf("选择权重最高的NTP服务器 %s（权重: %.1f），但未找到样本数据\n",
				bestServer.Address, bestServer.Weight))
		}
	}

	// 检查是否找到有效的NTP服务器
	if bestResult == nil {
		logger.Info("TimeService", "多源NTP同步失败，没有找到有效的NTP服务器\n")
		return time.Time{}, fmt.Errorf("多源NTP同步失败，没有找到有效的NTP服务器")
	}

	// 使用找到的最佳服务器结果
	trustedTime := time.Unix(0, bestResult.Timestamp)

	// 标记选中的服务器，只对选中的服务器设置IsSelected=true，其他服务器保持不变
	for i, server := range ts.ntpServers {
		if server.Address == bestResult.Server {
			ts.ntpServers[i].IsSelected = true
			logger.Info("TimeService", fmt.Sprintf("已标记NTP服务器 %s 为选中状态\n", server.Address))
			break // 只标记选中的服务器，其他服务器保持不变
		}
	}

	// 更新统计信息
	ts.stats.LastDeviation = bestResult.Deviation
	if int64(bestResult.Deviation) > ts.stats.MaxDeviation {
		ts.stats.MaxDeviation = int64(bestResult.Deviation)
	}

	logger.Info("TimeService", fmt.Sprintf("NTP同步完成，使用服务器 %s，成功样本数: %d, 偏差: %.2f ms, 往返时间: %.2f ms\n",
		bestResult.Server, bestResult.SuccessCount, bestResult.Deviation/1e6, bestResult.RTT/1e6))
	return trustedTime, nil
}

// querySingleSyncTime 查询单个NTP服务器
func (ts *TimeService) querySingleSyncTime(server TimeServiceNTPServer) (TimeServiceNTPTimeResult, error) {
	var samples []TimeServiceNTPSample
	sampleCount := ts.config.SampleCount // 使用配置中的样本数量
	sampleDelay := ts.config.SampleDelay // 使用配置中的样本延迟

	// 获取配置中指定数量的样本
	for i := 0; i < sampleCount; i++ {
		resp, err := ntp.Query(server.Address)
		if err != nil {
			// 添加失败样本，状态为"失败"
			samples = append(samples, TimeServiceNTPSample{
				Timestamp: time.Now().UnixNano(), // 使用当前时间作为时间戳
				RTT:       0,                     // 失败时RTT为0
				Deviation: 0,                     // 失败时偏差为0
				Status:    "Failed",              // 设置状态为失败
			})

			// 只有在不是最后一次循环时才延迟
			if i < sampleCount-1 {
				time.Sleep(sampleDelay)
			}
			continue
		}

		if resp.Stratum == 0 { // Stratum 0为无效源
			// 添加无效源样本，状态为"失败"
			samples = append(samples, TimeServiceNTPSample{
				Timestamp: time.Now().UnixNano(), // 使用当前时间作为时间戳
				RTT:       resp.RTT.Nanoseconds(),
				Deviation: 0,        // 无效源时偏差为0
				Status:    "Failed", // 设置状态为失败
			})

			// 只有在不是最后一次循环时才延迟
			if i < sampleCount-1 {
				time.Sleep(sampleDelay)
			}
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

		// 只有在不是最后一次循环时才延迟
		if i < sampleCount-1 {
			time.Sleep(sampleDelay)
		}
	}

	// 保存样本数据到lastNTPSamples字段
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
			sampleInfo += fmt.Sprintf("[序号%d, 状态%s, RTT%.2fms, 偏差%.2fms] ",
				i+1, sample.Status, float64(sample.RTT)/1e6, sample.Deviation/1e6)
		}

		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样完成，有效样本数: %d, 往返时间: %.2f ms, 偏差: %.2f ms\n%s\n",
			server.Address, len(samples), rtt/1e6, lastSuccessDeviation/1e6, sampleInfo))
	} else {
		// 即使没有有效样本也要记录
		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样完成，有效样本数: 0\n",
			server.Address))
	}

	// 检查是否有足够的有效样本并且满足最少连续成功阈值
	// 注意：即使样本不足或不满足阈值，也要返回结果，确保样本数量与配置一致
	if len(samples) < ts.config.SampleCount {
		// 这种情况不应该发生，因为我们已经确保获取了指定数量的样本
		logger.Info("TimeService", fmt.Sprintf("警告: NTP服务器 %s 样本数量不足，期望: %d, 实际: %d\n",
			server.Address, ts.config.SampleCount, len(samples)))
	}

	// 确保返回的样本数量与配置一致
	if len(samples) != ts.config.SampleCount {
		logger.Info("TimeService", fmt.Sprintf("错误: NTP服务器 %s 样本数量不匹配，期望: %d, 实际: %d\n",
			server.Address, ts.config.SampleCount, len(samples)))
	}

	// 即使没有成功样本，也要返回结果，确保样本数量与配置一致
	if len(samples) == 0 {
		result := TimeServiceNTPTimeResult{
			Timestamp:    time.Now().UnixNano(),
			Deviation:    0,
			Weight:       server.Weight,
			Server:       server.Address,
			SampleCount:  0,
			SuccessCount: 0,
			RTT:          0,
		}
		return result, fmt.Errorf("没有获取到任何样本")
	}

	// 如果有样本但没有成功样本，使用最后一个样本（即使是失败的）
	if successCount == 0 {
		lastSample := samples[len(samples)-1]
		result := TimeServiceNTPTimeResult{
			Timestamp:    lastSample.Timestamp,
			Deviation:    lastSample.Deviation,
			Weight:       server.Weight,
			Server:       server.Address,
			SampleCount:  len(samples),
			SuccessCount: 0,
			RTT:          float64(lastSample.RTT),
		}
		return result, fmt.Errorf("所有样本都失败")
	}

	// 正常情况：有成功样本
	result := TimeServiceNTPTimeResult{
		Timestamp:    medianTimestamp,      // 使用中位数作为最终时间戳
		Deviation:    lastSuccessDeviation, // 修改：使用最后一个成功样本的偏差
		Weight:       server.Weight,
		Server:       server.Address,
		SampleCount:  len(samples),
		SuccessCount: successCount,
		RTT:          rtt,
	}

	return result, nil
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
func InitGlobalTimeService() (*TimeService, error) {
	// 使用获取的NTP服务器配置初始化时间服务
	globalTimeService = NewTimeService()
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

// Now 获取全局格式化的可信时间（别名函数，方便使用）
func Now() time.Time {
	return GetTrustedTime()
}

// GetLastNTPSamples 获取上一次获取的NTP样本数据
func (ts *TimeService) GetLastNTPSamples() map[string][]TimeServiceNTPSample {
	return ts.lastNTPSamples
}
