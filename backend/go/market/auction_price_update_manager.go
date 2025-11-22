package market

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"own-1Pixel/backend/go/config"
	"own-1Pixel/backend/go/logger"
	"own-1Pixel/backend/go/timeservice"
)

// 拍卖缓存项
type AuctionCacheItem struct {
	Auction      *Auction
	LastUpdate   time.Time
	NextUpdate   time.Time
	LastPrice    float64
	NeedsRefresh bool
}

// WebSocket价格更新管理器
type AuctionPriceUpdateManager struct {
	dbConn           *sql.DB
	mutex            sync.Mutex
	auctionWSManager *AuctionWSManager
	isRunning        bool
	stopChan         chan bool
	updateInterval   time.Duration
	// 添加拍卖缓存
	auctionCache map[int]*AuctionCacheItem
	cacheMutex   sync.RWMutex
}

// 创建新的价格更新管理器
func InitAuctionWSPriceUpdateManager(dbConn *sql.DB, auctionWSManager *AuctionWSManager) *AuctionPriceUpdateManager {
	// 获取全局配置实例
	_config := config.GetConfig()
	auctionConfig := _config.Auction

	return &AuctionPriceUpdateManager{
		dbConn:           dbConn,
		auctionWSManager: auctionWSManager,
		isRunning:        false,
		stopChan:         make(chan bool),
		updateInterval:   time.Duration(auctionConfig.DefaultDecrementInterval) * time.Second, // 使用配置中的默认间隔
		auctionCache:     make(map[int]*AuctionCacheItem),
	}
}

// 启动价格更新管理器
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) StartAuctionWSPriceUpdateManager() {
	auctionWSPriceUpdateManager.mutex.Lock()
	defer auctionWSPriceUpdateManager.mutex.Unlock()

	if auctionWSPriceUpdateManager.isRunning {
		return
	}

	auctionWSPriceUpdateManager.isRunning = true
	auctionWSPriceUpdateManager.stopChan = make(chan bool)

	go auctionWSPriceUpdateManager.handleAuctionPriceUpdateLoop()

	logger.Info("auction_price_update_manager", "WebSocket价格更新管理器已启动\n")
}

// 停止价格更新管理器
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) StopAuctionWSPriceUpdateManager() {
	auctionWSPriceUpdateManager.mutex.Lock()
	defer auctionWSPriceUpdateManager.mutex.Unlock()

	if !auctionWSPriceUpdateManager.isRunning {
		return
	}

	auctionWSPriceUpdateManager.isRunning = false
	close(auctionWSPriceUpdateManager.stopChan)

	logger.Info("auction_price_update_manager", "WebSocket价格更新管理器已停止\n")
}

// 处理价格更新循环
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) handleAuctionPriceUpdateLoop() {
	ticker := time.NewTicker(auctionWSPriceUpdateManager.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			auctionWSPriceUpdateManager.updateActiveAuctionPrices()
		case <-auctionWSPriceUpdateManager.stopChan:
			return
		}
	}
}

// 更新活跃拍卖的价格
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) updateActiveAuctionPrices() {
	// 使用事务来减少数据库锁定时间
	tx, err := auctionWSPriceUpdateManager.dbConn.Begin()
	if err != nil {
		logger.Info("auction_price_update_manager", fmt.Sprintf("开始事务失败: %v\n", err))
		return
	}
	defer func() {
		// 如果发生错误，回滚事务
		if err != nil {
			tx.Rollback()
		} else {
			// 提交事务
			err = tx.Commit()
			if err != nil {
				logger.Info("auction_price_update_manager", fmt.Sprintf("提交事务失败: %v\n", err))
			}
		}
	}()

	// 查询所有活跃的拍卖
	rows, err := tx.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE status = 'active'`)
	if err != nil {
		logger.Info("auction_price_update_manager", fmt.Sprintf("查询活跃拍卖失败: %v\n", err))
		return
	}
	defer rows.Close()

	var auctions []Auction
	for rows.Next() {
		var auction Auction
		var startTime, endTime sql.NullTime
		scanErr := rows.Scan(
			&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
			&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
			&auction.Quantity, &startTime, &endTime, &auction.Status,
			&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
		if scanErr != nil {
			logger.Info("auction_price_update_manager", fmt.Sprintf("扫描拍卖数据失败: %v\n", scanErr))
			continue
		}

		// 处理可能为NULL的时间字段
		if startTime.Valid {
			auction.StartTime = &startTime.Time
		}
		if endTime.Valid {
			auction.EndTime = &endTime.Time
		}

		auctions = append(auctions, auction)
	}

	// 更新缓存中的拍卖信息
	auctionWSPriceUpdateManager.updateAuctionCache(auctions)

	// 只更新需要更新的拍卖价格
	for _, auction := range auctions {
		if auctionWSPriceUpdateManager.shouldUpdateAuctionPrice(auction) {
			// 在事务内更新价格
			err = auctionWSPriceUpdateManager.updateAuctionPrice(tx, auction)
			if err != nil {
				logger.Info("auction_price_update_manager", fmt.Sprintf("更新拍卖价格失败: %v\n", err))
				continue
			}
		}
	}

	// 检查是否还有活跃的拍卖，如果没有则停止价格更新管理器
	if len(auctions) == 0 {
		auctionWSPriceUpdateManager.StopAuctionWSPriceUpdateManager()
	}
}

// 更新拍卖缓存
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) updateAuctionCache(auctions []Auction) {
	auctionWSPriceUpdateManager.cacheMutex.Lock()
	defer auctionWSPriceUpdateManager.cacheMutex.Unlock()

	now := timeservice.SyncNow()

	// 创建当前活跃拍卖ID的映射
	activeAuctionIDs := make(map[int]bool)
	for _, auction := range auctions {
		activeAuctionIDs[auction.ID] = true

		// 如果拍卖不在缓存中，添加到缓存
		if _, exists := auctionWSPriceUpdateManager.auctionCache[auction.ID]; !exists {
			auctionWSPriceUpdateManager.auctionCache[auction.ID] = &AuctionCacheItem{
				Auction:      &auction,
				LastUpdate:   now,
				NextUpdate:   now,
				LastPrice:    auction.CurrentPrice,
				NeedsRefresh: true,
			}
		} else {
			// 更新缓存中的拍卖信息
			cacheItem := auctionWSPriceUpdateManager.auctionCache[auction.ID]
			cacheItem.Auction = &auction
			cacheItem.LastUpdate = now
		}
	}

	// 移除不再活跃的拍卖缓存
	for id := range auctionWSPriceUpdateManager.auctionCache {
		if !activeAuctionIDs[id] {
			delete(auctionWSPriceUpdateManager.auctionCache, id)
		}
	}
}

// 判断是否应该更新拍卖价格
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) shouldUpdateAuctionPrice(auction Auction) bool {
	auctionWSPriceUpdateManager.cacheMutex.RLock()
	defer auctionWSPriceUpdateManager.cacheMutex.RUnlock()

	now := timeservice.SyncNow()

	// 如果拍卖不在缓存中，需要更新
	cacheItem, exists := auctionWSPriceUpdateManager.auctionCache[auction.ID]
	if !exists {
		return true
	}

	// 如果到了下次更新时间，需要更新
	if now.After(cacheItem.NextUpdate) {
		return true
	}

	// 如果价格发生了变化，需要更新
	if cacheItem.LastPrice != auction.CurrentPrice {
		return true
	}

	return false
}

// 在事务内更新单个拍卖的价格
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) updateAuctionPrice(tx *sql.Tx, auction Auction) error {
	if auction.StartTime == nil {
		return nil
	}

	// 计算从开始时间到现在经过了多少个递减间隔
	elapsedTime := time.Since(*auction.StartTime)
	intervalsPassed := int(elapsedTime.Seconds()) / auction.DecrementInterval

	// 使用拍卖自身配置的价格递减量，而不是硬编码的1.0
	totalDecrement := float64(intervalsPassed) * auction.PriceDecrement

	// 计算新的当前价格
	newPrice := auction.InitialPrice - totalDecrement

	// 如果新价格低于最低价格，则设置为最低价格
	if newPrice < auction.MinPrice {
		newPrice = auction.MinPrice
	}

	// 如果价格已经达到最低价格，则结束拍卖
	if newPrice <= auction.MinPrice {
		// 更新拍卖状态为已完成
		_, err := tx.Exec("UPDATE auctions SET status = 'completed', current_price = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			newPrice, auction.ID)
		if err != nil {
			return err
		}

		// 更新缓存中的拍卖信息
		auctionWSPriceUpdateManager.updateCacheAfterPriceUpdate(auction.ID, newPrice, true)

		// 创建更新后的拍卖对象，避免再次查询数据库
		updatedAuction := auction
		updatedAuction.CurrentPrice = newPrice
		updatedAuction.Status = "completed"

		// 广播拍卖更新
		auctionWSPriceUpdateManager.auctionWSManager.BroadcastAuctionWSUpdate(&updatedAuction, "completed")

		logger.Info("auction_price_update_manager", fmt.Sprintf("拍卖ID %d 已达到最低价格，拍卖结束\n", auction.ID))
		return nil
	}

	// 只有当价格有变化且变化方向正确（递减）时，才更新数据库
	// 添加价格变化方向检查，防止价格波动
	if newPrice != auction.CurrentPrice && newPrice < auction.CurrentPrice {
		oldPrice := auction.CurrentPrice
		_, err := tx.Exec("UPDATE auctions SET current_price = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			newPrice, auction.ID)
		if err != nil {
			return err
		}

		// 更新缓存中的拍卖信息
		auctionWSPriceUpdateManager.updateCacheAfterPriceUpdate(auction.ID, newPrice, false)

		// 创建更新后的拍卖对象，避免再次查询数据库
		updatedAuction := auction
		updatedAuction.CurrentPrice = newPrice

		// 计算剩余时间
		timeRemaining := auctionWSPriceUpdateManager.calculateTimeRemaining(&updatedAuction)

		// 广播价格更新
		auctionWSPriceUpdateManager.auctionWSManager.BroadcastAuctionWSPriceUpdate(auction.ID, oldPrice, newPrice, timeRemaining)

		// 广播拍卖更新
		auctionWSPriceUpdateManager.auctionWSManager.BroadcastAuctionWSUpdate(&updatedAuction, "auction_price_updated")

		logger.Info("auction_price_update_manager", fmt.Sprintf("拍卖ID %d 价格已更新: %.2f -> %.2f\n", auction.ID, oldPrice, newPrice))
	} else if newPrice >= auction.CurrentPrice {
		// 记录价格异常上涨或不变的情况
		logger.Info("auction_price_update_manager", fmt.Sprintf("价格更新异常：计算价格 %.2f 不低于当前价格 %.2f，跳过更新\n", newPrice, auction.CurrentPrice))

		// 即使价格没有更新，也要更新缓存中的下次更新时间
		auctionWSPriceUpdateManager.updateCacheNextUpdateTime(auction.ID)
	}

	return nil
}

// 更新价格后的缓存更新
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) updateCacheAfterPriceUpdate(auctionID int, newPrice float64, isCompleted bool) {
	auctionWSPriceUpdateManager.cacheMutex.Lock()
	defer auctionWSPriceUpdateManager.cacheMutex.Unlock()

	if cacheItem, exists := auctionWSPriceUpdateManager.auctionCache[auctionID]; exists {
		cacheItem.LastPrice = newPrice
		cacheItem.LastUpdate = timeservice.SyncNow()

		// 如果拍卖已完成，设置下次更新时间为很久以后
		if isCompleted {
			cacheItem.NextUpdate = timeservice.SyncNow().Add(24 * time.Hour)
		} else {
			// 根据拍卖的递减间隔设置下次更新时间
			if cacheItem.Auction != nil {
				cacheItem.NextUpdate = timeservice.SyncNow().Add(time.Duration(cacheItem.Auction.DecrementInterval/2) * time.Second)
			} else {
				// 默认1秒后更新
				cacheItem.NextUpdate = timeservice.SyncNow().Add(time.Second)
			}
		}
	}
}

// 更新缓存中的下次更新时间
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) updateCacheNextUpdateTime(auctionID int) {
	auctionWSPriceUpdateManager.cacheMutex.Lock()
	defer auctionWSPriceUpdateManager.cacheMutex.Unlock()

	if cacheItem, exists := auctionWSPriceUpdateManager.auctionCache[auctionID]; exists {
		// 根据拍卖的递减间隔设置下次更新时间
		if cacheItem.Auction != nil {
			cacheItem.NextUpdate = timeservice.SyncNow().Add(time.Duration(cacheItem.Auction.DecrementInterval/2) * time.Second)
		} else {
			// 默认1秒后更新
			cacheItem.NextUpdate = timeservice.SyncNow().Add(time.Second)
		}
	}
}

// 计算拍卖剩余时间（秒）
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) calculateTimeRemaining(auction *Auction) int {
	if auction.StartTime == nil {
		return 0
	}

	// 计算从初始价格到最低价格需要多少个递减间隔
	totalDecrementNeeded := auction.InitialPrice - auction.MinPrice
	intervalsNeeded := int(totalDecrementNeeded / auction.PriceDecrement)

	// 计算总时间
	totalTime := time.Duration(intervalsNeeded * auction.DecrementInterval)

	// 计算已经过的时间
	elapsedTime := time.Since(*auction.StartTime)

	// 计算剩余时间
	remainingTime := totalTime - elapsedTime

	if remainingTime < 0 {
		return 0
	}

	return int(remainingTime.Seconds())
}

// 检查管理器是否正在运行
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) IsRunning() bool {
	auctionWSPriceUpdateManager.mutex.Lock()
	defer auctionWSPriceUpdateManager.mutex.Unlock()
	return auctionWSPriceUpdateManager.isRunning
}

// 更新拍卖价格缓存
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) UpdateAuctionPriceCache(auctionID int, currentPrice float64) {
	auctionWSPriceUpdateManager.cacheMutex.Lock()
	defer auctionWSPriceUpdateManager.cacheMutex.Unlock()

	// 如果拍卖不在缓存中，添加到缓存
	if _, exists := auctionWSPriceUpdateManager.auctionCache[auctionID]; !exists {
		auctionWSPriceUpdateManager.auctionCache[auctionID] = &AuctionCacheItem{
			LastUpdate:   timeservice.SyncNow(),
			NextUpdate:   timeservice.SyncNow(),
			LastPrice:    currentPrice,
			NeedsRefresh: true,
		}
	} else {
		// 更新缓存中的价格信息
		cacheItem := auctionWSPriceUpdateManager.auctionCache[auctionID]
		cacheItem.LastPrice = currentPrice
		cacheItem.LastUpdate = timeservice.SyncNow()
	}
}
