package timeservice

import (
	"fmt"
	"math"
	"own-1Pixel/backend/go/config"
	"own-1Pixel/backend/go/logger"
	"own-1Pixel/backend/go/timeservice/clock"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/beevik/ntp"
)

var (
	processStartTimestamp int64                             // 单调时钟起点时间
	timeServiceConfig     config.TimeServiceConfig          // 配置参数
	ntpServers            []TimeServiceNTPServer            // NTP服务器配置
	status                TimeServiceStatus                 // 时间服务状态
	circuitBreaker        TimeServiceCircuitBreakerState    // 熔断器状态
	lastNTPSamples        map[string][]TimeServiceNTPSample // 上一次获取的NTP样本数据，按服务器地址存储
	lastNTPSamplesMutex   sync.RWMutex                      // 保护lastNTPSamples的读写锁
	syncTimestampOffset   int64                             // 同步时间偏移量（syncTimestampBase - processStartTimestamp）
	stats                 TimeServiceStats                  // 统计信息
)

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
	Status    string  // 样本状态：成功、失败
	RTT       int64   // 往返时间（纳秒）
	Deviation float64 // 偏差（纳秒）
}

// TimeServiceNTPTimeResult NTP查询结果（基于多个样本的聚合）
type TimeServiceNTPTimeResult struct {
	Timestamp    int64   // 聚合时间戳（纳秒）
	Address      string  // 服务器地址
	Weight       float64 // 权重
	RTT          float64 // 往返时间（纳秒）
	Deviation    float64 // 最后一个成功样本的偏差（纳秒）
	SampleCount  int     // 样本数量
	SuccessCount int     // 成功样本数量
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

// GetStatus 获取时间服务状态
func GetTimeServiceStatus() TimeServiceStatus {
	return status
}

// GetStats 获取时间服务统计信息
func GetTimeServiceStats() TimeServiceStats {
	return stats
}

// GetCircuitBreakerState 获取熔断器状态
func GetTimeServiceCircuitBreakerState() TimeServiceCircuitBreakerState {
	return circuitBreaker
}

// querySingleSyncTime 查询单个NTP服务器
func querySingleSyncTime(server TimeServiceNTPServer) (TimeServiceNTPTimeResult, error) {
	systemTimestampBase := clock.Now().UnixNano()

	var samples []TimeServiceNTPSample
	sampleCount := timeServiceConfig.SampleCount // 使用配置中的样本数量
	sampleDelay := timeServiceConfig.SampleDelay // 使用配置中的样本延迟

	// 获取配置中指定数量的样本
	for i := 0; i < sampleCount; i++ {
		resp, err := ntp.Query(server.Address)
		if err != nil {
			// 添加失败样本，状态为"失败"
			samples = append(samples, TimeServiceNTPSample{
				Timestamp: systemTimestampBase, // 使用系统时间戳
				Status:    "Failed",            // 设置状态为失败
				RTT:       0,                   // 失败时RTT为0
				Deviation: 0,                   // 失败时偏差为0
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
				Timestamp: systemTimestampBase, // 使用系统时间戳
				Status:    "Failed",            // 设置状态为失败
				RTT:       0,                   // 失败时RTT为0
				Deviation: 0,                   // 失败时偏差为0
			})

			// 只有在不是最后一次循环时才延迟
			if i < sampleCount-1 {
				time.Sleep(sampleDelay)
			}
			continue
		}

		// 计算偏差
		deviation := math.Abs(float64(resp.Time.UnixNano() - systemTimestampBase))

		// 添加成功样本，状态为"成功"
		samples = append(samples, TimeServiceNTPSample{
			Timestamp: resp.Time.UnixNano(),   // 使用NTP服务器返回的时间戳
			Status:    "Success",              // 设置状态为成功
			RTT:       resp.RTT.Nanoseconds(), // 成功时RTT为响应RTT
			Deviation: deviation,              // 成功时偏差为响应偏差
		})

		// 只有在不是最后一次循环时才延迟
		if i < sampleCount-1 {
			time.Sleep(sampleDelay)
		}
	}

	// 保存样本数据到lastNTPSamples字段
	lastNTPSamplesMutex.Lock()
	lastNTPSamples[server.Address] = samples
	lastNTPSamplesMutex.Unlock()

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
	var lastTimestamp int64   // 修改：使用特定样本的时间戳
	var lastAddress string    // 修改：使用最后一个成功样本的地址
	var lastWeight float64    // 修改：使用最后一个成功样本的权重
	var lastRTT float64       // 修改：使用最后一个成功样本的RTT
	var lastDeviation float64 // 修改：使用最后一个成功样本的偏差

	// 记录采样完成后的综合日志，包含失败和无效源统计
	if len(samples) > 0 {
		// 查找最后一个成功样本的时间戳、偏差和RTT
		for i := len(samples) - 1; i >= 0; i-- {
			if samples[i].Status == "Success" {
				lastTimestamp = samples[i].Timestamp // 修改：使用特定样本的时间戳
				lastAddress = server.Address         // 修改：使用最后一个成功样本的地址
				lastWeight = server.Weight           // 修改：使用最后一个成功样本的权重
				lastRTT = float64(samples[i].RTT)    // 修改：使用最后一个成功样本的RTT
				lastDeviation = samples[i].Deviation // 修改：使用最后一个成功样本的偏差
				break
			}
		}

		// 记录样本列表信息
		sampleList := "样本列表: "
		for i, sample := range samples {
			sampleList += fmt.Sprintf("[序号%d，时间戳%d，状态%s，RTT%.2fms，偏差%.2fms] ",
				i+1, sample.Timestamp, sample.Status, float64(sample.RTT)/1e6, sample.Deviation/1e6)
		}

		logger.Info("TimeService", fmt.Sprintf("最后成功NTP服务器 %s，权重: %.1f，往返时间: %.2f ms，偏差: %.2f ms\n%s\n",
			lastAddress, lastWeight, lastRTT/1e6, lastDeviation/1e6, sampleList))
	}

	// 没有获取到任何样本
	if len(samples) == 0 {
		result := TimeServiceNTPTimeResult{
			Timestamp:    systemTimestampBase,
			Address:      server.Address,
			Weight:       server.Weight,
			RTT:          0,
			Deviation:    0,
			SampleCount:  0,
			SuccessCount: 0,
		}
		return result, fmt.Errorf("没有获取到任何样本")
	}

	// 所有样本都失败
	if successCount == 0 {
		lastSample := samples[len(samples)-1]
		result := TimeServiceNTPTimeResult{
			Timestamp:    lastSample.Timestamp,
			Address:      server.Address,
			Weight:       server.Weight,
			RTT:          float64(lastSample.RTT),
			Deviation:    lastSample.Deviation,
			SampleCount:  len(samples),
			SuccessCount: 0,
		}
		return result, fmt.Errorf("所有样本都失败")
	}

	// 正常情况：有成功样本
	result := TimeServiceNTPTimeResult{
		Timestamp:    lastTimestamp, // 修改：使用特定样本的时间戳
		Address:      lastAddress,   // 修改：使用最后一个成功样本的地址
		Weight:       lastWeight,    // 修改：使用最后一个成功样本的权重
		RTT:          lastRTT,       // 修改：使用最后一个成功样本的RTT
		Deviation:    lastDeviation, // 修改：使用最后一个成功样本的偏差
		SampleCount:  len(samples),
		SuccessCount: successCount,
	}

	return result, nil
}

// queryMultiSyncTimestamp 多源NTP同步
func queryMultiSyncTimestamp() (int64, error) {
	logger.Info("TimeService", fmt.Sprintf("开始多源NTP同步（并行查询所有服务器，每个服务器获取%d个样本）...\n", timeServiceConfig.SampleCount))

	var bestResult *TimeServiceNTPTimeResult

	// 使用通道和goroutine并行查询所有NTP服务器
	type serverResult struct {
		server TimeServiceNTPServer
		result TimeServiceNTPTimeResult
		err    error
	}

	resultChan := make(chan serverResult, len(ntpServers))

	// 启动goroutine并行查询每个服务器
	for _, server := range ntpServers {
		go func(s TimeServiceNTPServer) {
			result, err := querySingleSyncTime(s)
			if err != nil {
				// 记录查询结果
				logger.Info("TimeService", fmt.Sprintf("查询NTP服务器 %s 结果: %v, 错误: %v\n", s.Address, result, err))
			}
			resultChan <- serverResult{server: s, result: result, err: err}
		}(server)
	}

	// 收集所有服务器的查询结果
	results := make([]serverResult, 0, len(ntpServers))
	for i := 0; i < len(ntpServers); i++ {
		resultChans := <-resultChan
		// 检查结果是否包含指定数量的样本
		if resultChans.err == nil && resultChans.result.SampleCount != timeServiceConfig.SampleCount {
			logger.Info("TimeService", fmt.Sprintf("警告: NTP服务器 %s 返回的样本数(%d)与配置的样本数(%d)不匹配\n",
				resultChans.server.Address, resultChans.result.SampleCount, timeServiceConfig.SampleCount))
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
			logger.Info("TimeService", fmt.Sprintf("NTP时间异常跳变（偏差过大）：%.2f s，跳过服务器 %s，可能存在入侵风险\n",
				resultChans.result.Deviation/1e9, resultChans.server.Address))
			continue
		}

		// 记录采样结果
		logger.Info("TimeService", fmt.Sprintf("NTP服务器 %s 采样成功，权重: %.1f，样本数: %d，成功样本数: %d，往返时间: %.2f ms，偏差: %.2f ms\n",
			resultChans.server.Address, resultChans.server.Weight, resultChans.result.SampleCount, resultChans.result.SuccessCount, resultChans.result.Deviation/1e6, resultChans.result.RTT/1e6))

		// 添加到有效结果列表
		validResults = append(validResults, resultChans)
	}

	// 优先选择最后一个成功样本
	if len(validResults) > 0 {
		// 查找所有服务器中最新的成功样本时间戳
		var latestTimestamp int64
		var selectedServer TimeServiceNTPServer
		var selectedResult *TimeServiceNTPTimeResult

		// 初始化为第一个有效服务器的最后一个成功样本
		lastNTPSamplesMutex.RLock()
		if samples, exists := lastNTPSamples[validResults[0].server.Address]; exists && len(samples) > 0 {
			for j := len(samples) - 1; j >= 0; j-- {
				if samples[j].Status == "Success" {
					latestTimestamp = samples[j].Timestamp
					selectedServer = validResults[0].server
					selectedResult = &validResults[0].result
					break
				}
			}
		}
		lastNTPSamplesMutex.RUnlock()

		// 遍历所有有效服务器，找到最新的成功样本
		for _, resultChans := range validResults {
			lastNTPSamplesMutex.RLock()
			if samples, exists := lastNTPSamples[resultChans.server.Address]; exists && len(samples) > 0 {
				// 从后往前查找最后一个成功样本
				for i := len(samples) - 1; i >= 0; i-- {
					if samples[i].Status == "Success" {
						// 如果找到更新的成功样本，则更新选择
						if samples[i].Timestamp > latestTimestamp {
							latestTimestamp = samples[i].Timestamp
							selectedServer = resultChans.server
							selectedResult = &resultChans.result
						}
						break // 找到该服务器的最后一个成功样本后，跳出内层循环
					}
				}
			}
			lastNTPSamplesMutex.RUnlock()
		}

		// 使用选中的服务器和其最后一个成功样本
		if selectedResult != nil {
			bestServer := selectedServer
			bestResult = selectedResult

			// 获取该服务器的最后一个样本
			lastNTPSamplesMutex.RLock()
			if samples, exists := lastNTPSamples[bestServer.Address]; exists && len(samples) > 0 {
				// 从后往前查找最后一个成功样本
				for i := len(samples) - 1; i >= 0; i-- {
					if samples[i].Status == "Success" {
						lastSample := samples[i]

						bestResult.Timestamp = lastSample.Timestamp // 使用最后一个成功样本的时间戳作为同步时间
						bestResult.Deviation = lastSample.Deviation
						bestResult.RTT = float64(lastSample.RTT)

						break
					}
				}
			} else {
				logger.Info("TimeService", fmt.Sprintf("选择NTP服务器 %s（权重: %.1f），但未找到样本数据\n",
					bestServer.Address, bestServer.Weight))
			}
			lastNTPSamplesMutex.RUnlock()
		}
	}

	// 检查是否找到有效的NTP服务器
	if bestResult == nil {
		logger.Info("TimeService", "多源NTP同步失败，没有找到有效的NTP服务器\n")
		return int64(0), fmt.Errorf("多源NTP同步失败，没有找到有效的NTP服务器")
	}

	// 使用找到的最佳服务器结果
	syncTimestamp := bestResult.Timestamp

	// 标记选中的服务器，只对选中的服务器设置IsSelected=true，其他服务器保持不变
	for i, server := range ntpServers {
		if server.Address == bestResult.Address {
			ntpServers[i].IsSelected = true
			fmt.Printf("已标记NTP服务器 %s 为选中状态\n", server.Address)
			break // 只标记选中的服务器，其他服务器保持不变
		}
	}

	// 更新统计信息
	stats.LastDeviation = bestResult.Deviation
	if int64(bestResult.Deviation) > stats.MaxDeviation {
		stats.MaxDeviation = int64(bestResult.Deviation)
	}

	logger.Info("TimeService", fmt.Sprintf("NTP同步完成，使用服务器 %s，成功样本数: %d, 偏差: %.2f ms, 往返时间: %.2f ms\n",
		bestResult.Address, bestResult.SuccessCount, bestResult.Deviation/1e6, bestResult.RTT/1e6))
	return syncTimestamp, nil
}

// updateOffset 更新时间偏移量
func updateSyncTimestampOffset() error {
	// 获取多源同步时间戳
	syncTimestampBase, err := queryMultiSyncTimestamp()
	if err != nil {
		return err
	}

	// 计算新的偏移量
	newSyncTimestampOffset := syncTimestampBase - processStartTimestamp

	// 更新偏移量
	atomic.StoreInt64(&syncTimestampOffset, newSyncTimestampOffset)

	return nil
}

// syncWithRetry 带重试的同步
func syncCircuitBreaker() {
	// 检查熔断器状态
	if circuitBreaker.IsOpen {
		// 检查是否可以尝试恢复
		if time.Since(circuitBreaker.LastFailureTime) > timeServiceConfig.RecoveryTimeout {
			logger.Info("TimeService", "尝试从熔断状态恢复...\n")
			circuitBreaker.IsOpen = false
			circuitBreaker.FailureCount = 0
		} else {
			// 仍在熔断状态，跳过本次同步
			return
		}
	}

	// 使用clock包的单调时间戳，确保防重放、防篡改
	syncStartTimestamp := clock.GetMonotonicTimestamp()

	// 执行同步
	err := updateSyncTimestampOffset()
	if err != nil {
		// 使用clock包的单调时间戳，确保防重放、防篡改
		syncEndTimestamp := clock.GetMonotonicTimestamp()
		syncDurationTimestamp := syncEndTimestamp - syncStartTimestamp

		logger.Info("TimeService", fmt.Sprintf("NTP同步失败，耗时: %.3f ms，错误: %v\n", float64(syncDurationTimestamp)/1e6, err))
		atomic.AddInt64(&stats.FailedSyncs, 1)
		atomic.AddInt64(&circuitBreaker.FailureCount, 1)
		circuitBreaker.LastFailureTime = clock.Now()

		// 检查是否需要熔断
		if circuitBreaker.FailureCount >= timeServiceConfig.FailureThreshold {
			logger.Info("TimeService", "NTP同步失败次数过多，触发熔断\n")
			circuitBreaker.IsOpen = true
			status.IsDegraded = true
		}
	} else {
		// 使用clock包的单调时间戳，确保防重放、防篡改
		syncEndTime := clock.GetMonotonicTimestamp()
		syncDuration := syncEndTime - syncStartTimestamp

		// 同步成功
		atomic.AddInt64(&stats.SuccessfulSyncs, 1)
		atomic.AddInt64(&circuitBreaker.SuccessCount, 1)
		status.LastSyncTime = clock.Now()

		logger.Info("TimeService", fmt.Sprintf("NTP同步成功，耗时: %.3f ms\n", float64(syncDuration)/1e6))

		// 如果之前是降级模式，现在恢复
		if status.IsDegraded {
			logger.Info("TimeService", "时间服务已从降级模式恢复\n")
			status.IsDegraded = false
		}

		// 重置熔断器计数器
		if circuitBreaker.FailureCount > 0 {
			circuitBreaker.FailureCount = 0
		}
	}

	atomic.AddInt64(&stats.TotalSyncs, 1)
}

// startNTPSyncLoop 启动NTP同步循环
func startNTPSyncLoop() {
	logger.Info("TimeService", fmt.Sprintf("启动NTP同步循环，间隔: %v\n", timeServiceConfig.SyncInterval))

	ticker := time.NewTicker(timeServiceConfig.SyncInterval)
	defer ticker.Stop()

	for range ticker.C {
		// 使用clock包的单调时间戳，确保防重放、防篡改
		syncStartTime := clock.GetMonotonicTimestamp()

		// 调用原有的同步逻辑
		syncCircuitBreaker()

		// 使用clock包的单调时间戳，确保防重放、防篡改
		syncEndTime := clock.GetMonotonicTimestamp()
		syncDuration := syncEndTime - syncStartTime

		logger.Info("TimeService", fmt.Sprintf("NTP同步循环执行完成，耗时: %.3f ms\n", float64(syncDuration)/1e6))
	}
}

// IsInDegradedMode 检查是否处于降级模式
func IsInDegradedMode() bool {
	return status.IsDegraded
}

// GetNTPServers 获取NTP服务器列表
func GetNTPServers() []TimeServiceNTPServer {
	return ntpServers
}

// GetLastNTPSamples 获取上一次获取的NTP样本数据
func GetLastNTPSamples() map[string][]TimeServiceNTPSample {
	lastNTPSamplesMutex.RLock()
	defer lastNTPSamplesMutex.RUnlock()

	// 创建一个深拷贝以避免并发访问问题
	result := make(map[string][]TimeServiceNTPSample)
	for k, v := range lastNTPSamples {
		samples := make([]TimeServiceNTPSample, len(v))
		copy(samples, v)
		result[k] = samples
	}

	return result
}

// GetSyncTimestampOffset 获取当前时间偏移量
func GetSyncTimestampOffset() int64 {
	return atomic.LoadInt64(&syncTimestampOffset)
}

// GetSyncTimestamp 获取当前同步时间
func GetSyncTimestamp() time.Time {
	// 获取系统时间戳
	systemTimestampBase := clock.Now().UnixNano()

	// 计算同步时间戳：系统时间戳 + 时间偏移量
	syncTimestamp := systemTimestampBase + GetSyncTimestampOffset()

	// 转换为time.Time对象
	return time.Unix(0, syncTimestamp)
}

func SyncNow() time.Time {
	return GetSyncTimestamp()
}

// InitTimeServiceSystem 初始化全局时间服务系统
func InitTimeServiceSystem() error {
	// 初始化全局变量
	lastNTPSamplesMutex.Lock()
	lastNTPSamples = make(map[string][]TimeServiceNTPSample)
	lastNTPSamplesMutex.Unlock()

	// 获取全局配置实例
	_config := config.GetConfig()
	timeServiceConfig = _config.TimeService

	// 转换NTP服务器配置类型
	for _, ntpServer := range timeServiceConfig.NTPServers {
		ntpServers = append(ntpServers, TimeServiceNTPServer{
			Name:         ntpServer.Name,
			Address:      ntpServer.Address,
			Weight:       ntpServer.Weight,
			IsDomestic:   ntpServer.IsDomestic,
			MaxDeviation: ntpServer.MaxDeviation,
			IsSelected:   ntpServer.IsSelected,
		})
	}

	// 初始化状态
	status = TimeServiceStatus{
		IsInitialized: false,
		IsDegraded:    false,
	}

	// 初始化熔断器状态
	circuitBreaker = TimeServiceCircuitBreakerState{
		IsOpen:       false,
		FailureCount: 0,
		SuccessCount: 0,
	}

	// 初始化统计信息
	stats = TimeServiceStats{
		TotalSyncs:      0,
		SuccessfulSyncs: 0,
		FailedSyncs:     0,
		LastDeviation:   0,
		MaxDeviation:    0,
	}

	// 初始化时间服务
	logger.Info("TimeService", "初始化时间服务系统...\n")
	fmt.Printf("初始化时间服务系统...\n")

	// 1. 记录单调时钟起点
	processStartTimestamp = clock.GetMonotonicTimestamp()

	// 2. 同步多源NTP获取初始基准时间
	syncTimestampBase, err := queryMultiSyncTimestamp()

	// 无论成功还是失败，都要更新总同步计数
	atomic.AddInt64(&stats.TotalSyncs, 1)

	if err != nil {
		// 首次同步失败
		atomic.AddInt64(&stats.FailedSyncs, 1)
		logger.Info("TimeService", fmt.Sprintf("初始化NTP同步失败: %v\n", err))
		fmt.Printf("初始化NTP同步失败: %v\n", err)
		return fmt.Errorf("初始化NTP同步失败: %v", err)
	}

	// 计算基准偏移量
	newSyncTimestampOffset := syncTimestampBase - processStartTimestamp
	atomic.StoreInt64(&syncTimestampOffset, newSyncTimestampOffset)

	// 更新统计计数器 - 首次同步成功
	atomic.AddInt64(&stats.SuccessfulSyncs, 1)

	// 4. 更新状态
	status.IsInitialized = true
	status.IsDegraded = false
	status.LastSyncTime = clock.Now()

	logger.Info("TimeService", fmt.Sprintf("时间服务系统初始化成功，初始偏移量: %.7f s\n", float64(newSyncTimestampOffset)/1e9))
	fmt.Printf("时间服务系统初始化成功，初始偏移量: %.7f s\n", float64(newSyncTimestampOffset)/1e9)

	// 5. 启动定时NTP同步
	go startNTPSyncLoop()

	return nil
}
