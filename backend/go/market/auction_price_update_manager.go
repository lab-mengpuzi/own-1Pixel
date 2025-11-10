package market

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"own-1Pixel/backend/go/logger"
)

// WebSocket价格更新管理器
type AuctionPriceUpdateManager struct {
	db               *sql.DB
	auctionWSManager *AuctionWSManager
	isRunning        bool
	mu               sync.Mutex
	stopChan         chan bool
	updateInterval   time.Duration
}

// 创建新的价格更新管理器
func InitAuctionWSPriceUpdateManager(db *sql.DB, auctionWSManager *AuctionWSManager) *AuctionPriceUpdateManager {
	return &AuctionPriceUpdateManager{
		db:               db,
		auctionWSManager: auctionWSManager,
		isRunning:        false,
		stopChan:         make(chan bool),
		updateInterval:   time.Second, // 每秒更新一次
	}
}

// 启动价格更新管理器
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) StartAuctionWSPriceUpdateManager() {
	auctionWSPriceUpdateManager.mu.Lock()
	defer auctionWSPriceUpdateManager.mu.Unlock()

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
	auctionWSPriceUpdateManager.mu.Lock()
	defer auctionWSPriceUpdateManager.mu.Unlock()

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
	// 查询所有活跃的拍卖
	rows, err := auctionWSPriceUpdateManager.db.Query(`
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
		err := rows.Scan(
			&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
			&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
			&auction.Quantity, &startTime, &endTime, &auction.Status,
			&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
		if err != nil {
			logger.Info("auction_price_update_manager", fmt.Sprintf("扫描拍卖数据失败: %v\n", err))
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

	// 更新每个活跃拍卖的价格
	for _, auction := range auctions {
		auctionWSPriceUpdateManager.updateAuctionPrice(auction)
	}

	// 检查是否还有活跃的拍卖，如果没有则停止价格更新管理器
	if len(auctions) == 0 {
		auctionWSPriceUpdateManager.StopAuctionWSPriceUpdateManager()
	}
}

// 更新单个拍卖的价格
func (auctionWSPriceUpdateManager *AuctionPriceUpdateManager) updateAuctionPrice(auction Auction) {
	if auction.StartTime == nil {
		return
	}

	// 计算从开始时间到现在经过了多少个递减间隔
	elapsedTime := time.Since(*auction.StartTime)
	intervalsPassed := int(elapsedTime.Seconds()) / auction.DecrementInterval

	// 计算应该减少的价格总额
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
		_, err := auctionWSPriceUpdateManager.db.Exec("UPDATE auctions SET status = 'completed', current_price = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			newPrice, auction.ID)
		if err != nil {
			logger.Info("auction_price_update_manager", fmt.Sprintf("更新拍卖状态失败: %v\n", err))
			return
		}

		// 获取更新后的拍卖信息
		updatedAuction, err := GetAuctionID(auctionWSPriceUpdateManager.db, auction.ID)
		if err != nil {
			logger.Info("auction_price_update_manager", fmt.Sprintf("获取更新后的拍卖信息失败: %v\n", err))
			return
		}

		// 广播拍卖更新
		auctionWSPriceUpdateManager.auctionWSManager.BroadcastAuctionWSUpdate(updatedAuction, "completed")

		logger.Info("auction_price_update_manager", fmt.Sprintf("拍卖ID %d 已达到最低价格，拍卖结束\n", auction.ID))
		return
	}

	// 如果价格有变化，则更新数据库
	if newPrice != auction.CurrentPrice {
		oldPrice := auction.CurrentPrice
		_, err := auctionWSPriceUpdateManager.db.Exec("UPDATE auctions SET current_price = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			newPrice, auction.ID)
		if err != nil {
			logger.Info("auction_price_update_manager", fmt.Sprintf("更新拍卖价格失败: %v\n", err))
			return
		}

		// 获取更新后的拍卖信息
		updatedAuction, err := GetAuctionID(auctionWSPriceUpdateManager.db, auction.ID)
		if err != nil {
			logger.Info("auction_price_update_manager", fmt.Sprintf("获取更新后的拍卖信息失败: %v\n", err))
			return
		}

		// 计算剩余时间
		timeRemaining := auctionWSPriceUpdateManager.calculateTimeRemaining(updatedAuction)

		// 广播价格更新
		auctionWSPriceUpdateManager.auctionWSManager.BroadcastAuctionWSPriceUpdate(auction.ID, oldPrice, newPrice, timeRemaining)

		// 广播拍卖更新
		auctionWSPriceUpdateManager.auctionWSManager.BroadcastAuctionWSUpdate(updatedAuction, "auction_price_updated")

		logger.Info("auction_price_update_manager", fmt.Sprintf("拍卖ID %d 价格已更新: %.2f -> %.2f\n", auction.ID, oldPrice, newPrice))
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
	auctionWSPriceUpdateManager.mu.Lock()
	defer auctionWSPriceUpdateManager.mu.Unlock()
	return auctionWSPriceUpdateManager.isRunning
}
