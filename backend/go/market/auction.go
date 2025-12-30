package market

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"own-1Pixel/backend/go/config"
	"own-1Pixel/backend/go/logger"
	"own-1Pixel/backend/go/timeservice"
)

// 全局变量，用于存储价格递减定时器
var auctionPriceDecrementTimer *time.Timer
var isTimerRunning = false

// 荷兰钟拍卖结构
type Auction struct {
	ID                int           `json:"id"`
	ItemType          string        `json:"itemType"`          // 物品类型
	InitialPrice      float64       `json:"initialPrice"`      // 初始价格
	CurrentPrice      float64       `json:"currentPrice"`      // 当前价格
	MinPrice          float64       `json:"minPrice"`          // 最低价格
	PriceDecrement    float64       `json:"priceDecrement"`    // 价格递减量
	DecrementInterval int           `json:"decrementInterval"` // 价格递减间隔（秒）
	Quantity          int           `json:"quantity"`          // 数量
	StartTime         *time.Time    `json:"startTime"`         // 开始时间
	EndTime           *time.Time    `json:"endTime"`           // 结束时间
	Status            string        `json:"status"`            // 状态：pending, active, completed, cancelled
	WinnerID          sql.NullInt64 `json:"winnerId"`          // 中标者ID（用户ID）
	CreatedAt         time.Time     `json:"created_at"`        // 创建时间
	UpdatedAt         time.Time     `json:"updated_at"`        // 更新时间
}

// 荷兰钟竞价记录
type AuctionBid struct {
	ID        int       `json:"id"`
	AuctionID int       `json:"auctionId"` //
	UserID    int       `json:"userId"`
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"` // 状态：pending, accepted, rejected
	CreatedAt time.Time `json:"created_at"`
}

// 初始化荷兰钟拍卖数据库表
func InitAuctionDatabase(dbConn *sql.DB) error {
	logger.Info("auction", "初始化荷兰钟拍卖数据库表\n")

	// 创建荷兰钟拍卖表
	_, err := dbConn.Exec(`
		CREATE TABLE IF NOT EXISTS auctions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_type TEXT NOT NULL,
			initial_price REAL NOT NULL,
			current_price REAL NOT NULL,
			min_price REAL NOT NULL,
			price_decrement REAL NOT NULL,
			decrement_interval INTEGER NOT NULL,
			quantity INTEGER NOT NULL,
			start_time DATETIME,
			end_time DATETIME,
			status TEXT NOT NULL DEFAULT 'pending',
			winner_id INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("创建荷兰钟拍卖表失败: %v\n", err))
		return err
	}

	// 创建荷兰钟竞价记录表
	_, err = dbConn.Exec(`
		CREATE TABLE IF NOT EXISTS auction_bids (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			auction_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			price REAL NOT NULL,
			quantity INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME,
			FOREIGN KEY (auction_id) REFERENCES auctions(id)
		)
	`)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("创建荷兰钟竞价记录表失败: %v\n", err))
		return err
	}

	logger.Info("auction", "荷兰钟拍卖数据库表初始化完成\n")

	// 恢复进行中的拍卖
	recoverActiveAuctions(dbConn)

	return nil
}

// 启动价格递减定时器
func StartAuctionPriceDecrementTimer(db *sql.DB) {
	if isTimerRunning {
		return
	}

	isTimerRunning = true

	// 立即执行一次价格更新
	updateActiveAuctionPrices(db)

	// 获取全局配置实例
	_config := config.GetConfig()
	auctionConfig := _config.Auction

	// 设置定时器，使用配置中的默认间隔
	auctionPriceDecrementTimer = time.AfterFunc(time.Duration(auctionConfig.DefaultDecrementInterval)*time.Second, func() {
		updateActiveAuctionPrices(db)
		// 递归调用，保持定时器运行
		if isTimerRunning {
			StartAuctionPriceDecrementTimer(db)
		}
	})
}

// 停止价格递减定时器
func StopAuctionPriceDecrementTimer() {
	if auctionPriceDecrementTimer != nil {
		auctionPriceDecrementTimer.Stop()
	}
	isTimerRunning = false
}

// 恢复进行中的拍卖
func recoverActiveAuctions(db *sql.DB) {
	logger.Info("auction", "检查并恢复进行中的拍卖...\n")

	// 获取所有活跃拍卖
	activeAuctions, err := GetActiveAuctions(db)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("获取活跃拍卖失败: %v\n", err))
		return
	}

	if len(activeAuctions) == 0 {
		logger.Info("auction", "没有进行中的拍卖需要恢复\n")
		return
	}

	logger.Info("auction", fmt.Sprintf("发现 %d 个进行中的拍卖，开始恢复...\n", len(activeAuctions)))

	// 启动价格递减定时器
	StartAuctionPriceDecrementTimer(db)
	logger.Info("auction", "价格递减定时器已启动，将自动更新活跃拍卖价格\n")
}

// 更新活跃拍卖的价格
func updateActiveAuctionPrices(db *sql.DB) {
	// 查询所有活跃的拍卖
	rows, err := db.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE status = 'active'`)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("查询活跃拍卖失败: %v\n", err))
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
			logger.Info("auction", fmt.Sprintf("扫描拍卖数据失败: %v\n", err))
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
		updateAuctionPrice(db, auction)
	}
}

// 更新单个拍卖的价格
func updateAuctionPrice(db *sql.DB, auction Auction) {
	if auction.StartTime == nil {
		return
	}

	var currentTime time.Time

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

	// 如果价格已经达到最低价格，则取消拍卖并退还物品
	if newPrice <= auction.MinPrice {
		// 开始事务
		tx, err := db.Begin()
		if err != nil {
			logger.Info("auction", fmt.Sprintf("开始事务失败: %v\n", err))
			return
		}

		// 更新拍卖状态为已取消
		currentTime = timeservice.SyncNow()
		_, err = tx.Exec("UPDATE auctions SET status = 'cancelled', current_price = ?, updated_at = ? WHERE id = ?",
			newPrice, currentTime, auction.ID)
		if err != nil {
			logger.Info("auction", fmt.Sprintf("更新拍卖状态失败: %v\n", err))
			tx.Rollback()
			return
		}

		// 退还物品至背包
		err = UnlockBackpackItems(tx, auction.ItemType, auction.Quantity)
		if err != nil {
			logger.Info("auction", fmt.Sprintf("退还物品至背包失败: %v\n", err))
			tx.Rollback()
			return
		}

		// 提交事务
		err = tx.Commit()
		if err != nil {
			logger.Info("auction", fmt.Sprintf("提交事务失败: %v\n", err))
			return
		}

		logger.Info("auction", fmt.Sprintf("拍卖ID %d 已达到最低价格但无人竞价，拍卖已取消并退还物品\n", auction.ID))

		// 检查是否还有其他活跃的拍卖，如果没有则停止定时器
		var activeAuctionCount int
		err = db.QueryRow("SELECT COUNT(*) FROM auctions WHERE status = 'active'").Scan(&activeAuctionCount)
		if err != nil {
			logger.Info("auction", fmt.Sprintf("检查活跃拍卖数量失败: %v\n", err))
			return
		}

		if activeAuctionCount == 0 {
			StopAuctionPriceDecrementTimer()
			logger.Info("auction", "没有活跃的拍卖，停止价格递减定时器\n")
		}

		return
	}

	// 只有当价格有变化且变化方向正确（递减）时，才更新数据库
	// 添加价格变化方向检查，防止价格波动
	if newPrice != auction.CurrentPrice && newPrice <= auction.CurrentPrice {
		_, err := db.Exec("UPDATE auctions SET current_price = ?, updated_at = ? WHERE id = ?",
			newPrice, currentTime, auction.ID)
		if err != nil {
			logger.Info("auction", fmt.Sprintf("更新拍卖价格失败: %v\n", err))
			return
		}
		logger.Info("auction", fmt.Sprintf("拍卖ID %d 价格已更新: %.2f -> %.2f\n", auction.ID, auction.CurrentPrice, newPrice))
	} else if newPrice > auction.CurrentPrice {
		// 记录价格异常上涨的情况
		logger.Info("auction", fmt.Sprintf("价格更新异常：计算价格 %.2f 高于当前价格 %.2f，跳过更新\n", newPrice, auction.CurrentPrice))
	}
}

// 检查并锁定背包中的物品（事务版本）
func LockBackpackItems(tx *sql.Tx, itemType string, quantity int) error {
	// 获取当前背包
	var backpack struct {
		ID        int       `json:"id"`
		Apple     int       `json:"apple"`
		Wood      int       `json:"wood"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	var currentTime time.Time

	err := tx.QueryRow("SELECT id, apple, wood, created_at, updated_at FROM backpack ORDER BY id DESC LIMIT 1").Scan(
		&backpack.ID, &backpack.Apple, &backpack.Wood, &backpack.CreatedAt, &backpack.UpdatedAt)
	if err != nil {
		return fmt.Errorf("获取背包状态失败: %v", err)
	}

	// 检查背包中是否有足够的物品
	switch itemType {
	case "apple":
		if backpack.Apple < quantity {
			return fmt.Errorf("背包中的苹果数量不足，需要 %d 个，当前 %d 个", quantity, backpack.Apple)
		}
		// 更新背包中的苹果数量
		currentTime = timeservice.SyncNow()
		_, err = tx.Exec("UPDATE backpack SET apple = apple - ?, updated_at = ? WHERE id = ?",
			quantity, currentTime, backpack.ID)
	case "wood":
		if backpack.Wood < quantity {
			return fmt.Errorf("背包中的木材数量不足，需要 %d 个，当前 %d 个", quantity, backpack.Wood)
		}
		// 更新背包中的木材数量
		currentTime = timeservice.SyncNow()
		_, err = tx.Exec("UPDATE backpack SET wood = wood - ?, updated_at = ? WHERE id = ?",
			quantity, currentTime, backpack.ID)
	default:
		return fmt.Errorf("无效的物品类型: %s", itemType)
	}

	if err != nil {
		return fmt.Errorf("更新背包失败: %v", err)
	}

	return nil
}

// 解锁背包中的物品（事务版本，当拍卖被取消时调用）
func UnlockBackpackItems(tx *sql.Tx, itemType string, quantity int) error {
	// 获取当前背包
	var backpack struct {
		ID        int       `json:"id"`
		Apple     int       `json:"apple"`
		Wood      int       `json:"wood"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	var currentTime time.Time

	err := tx.QueryRow("SELECT id, apple, wood, created_at, updated_at FROM backpack ORDER BY id DESC LIMIT 1").Scan(
		&backpack.ID, &backpack.Apple, &backpack.Wood, &backpack.CreatedAt, &backpack.UpdatedAt)
	if err != nil {
		return fmt.Errorf("获取背包状态失败: %v", err)
	}

	// 更新背包中的物品数量
	switch itemType {
	case "apple":
		currentTime = timeservice.SyncNow()
		_, err = tx.Exec("UPDATE backpack SET apple = apple + ?, updated_at = ? WHERE id = ?",
			quantity, currentTime, backpack.ID)
	case "wood":
		currentTime = timeservice.SyncNow()
		_, err = tx.Exec("UPDATE backpack SET wood = wood + ?, updated_at = ? WHERE id = ?",
			quantity, currentTime, backpack.ID)
	default:
		return fmt.Errorf("无效的物品类型: %s", itemType)
	}

	if err != nil {
		return fmt.Errorf("更新背包失败: %v", err)
	}

	return nil
}

// 创建荷兰钟拍卖
func CreateAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "创建荷兰钟拍卖请求\n")

	var currentTime time.Time

	// 设置响应头为JSON格式
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		logger.Info("auction", fmt.Sprintf("创建荷兰钟拍卖请求失败，不支持的请求方法: %s\n", r.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "不支持的请求方法",
		})
		return
	}

	var auction Auction
	err := json.NewDecoder(r.Body).Decode(&auction)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("解析荷兰钟拍卖JSON失败: %v\n", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("解析请求数据失败: %v", err),
		})
		return
	}

	// 验证输入
	if auction.ItemType != "apple" && auction.ItemType != "wood" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "无效的物品类型",
		})
		return
	}

	if auction.InitialPrice <= 0 || auction.MinPrice < 0 || auction.PriceDecrement <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "初始价格、最低价格和价格递减量必须为正数",
		})
		return
	}

	if auction.InitialPrice < auction.MinPrice {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "初始价格必须大于或等于最低价格",
		})
		return
	}

	if auction.Quantity <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "数量必须为正数",
		})
		return
	}

	if auction.DecrementInterval <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "价格递减间隔必须为正数",
		})
		return
	}

	// 设置默认值
	auction.Status = "pending"
	auction.CurrentPrice = auction.InitialPrice

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("启动事务失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("启动事务失败: %v", err),
		})
		return
	}

	// 检查并锁定背包中的物品
	err = LockBackpackItems(tx, auction.ItemType, auction.Quantity)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("锁定背包物品失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 插入拍卖记录
	currentTime = timeservice.SyncNow()
	result, err := tx.Exec(`
		INSERT INTO auctions 
		(item_type, initial_price, current_price, min_price, price_decrement, decrement_interval, quantity, start_time, end_time, status, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		auction.ItemType, auction.InitialPrice, auction.CurrentPrice, auction.MinPrice,
		auction.PriceDecrement, auction.DecrementInterval, auction.Quantity,
		nil, nil, auction.Status, currentTime, currentTime)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("插入拍卖记录失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("插入拍卖记录失败: %v", err),
		})
		return
	}

	// 获取新插入的拍卖ID
	auctionID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		logger.Info("auction", fmt.Sprintf("获取拍卖ID失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("获取拍卖ID失败: %v", err),
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交事务失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("提交事务失败: %v", err),
		})
		return
	}

	// 获取完整的拍卖信息
	var newAuction Auction
	var startTime, endTime sql.NullTime
	err = db.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE id = ?`, auctionID).Scan(
		&newAuction.ID, &newAuction.ItemType, &newAuction.InitialPrice, &newAuction.CurrentPrice,
		&newAuction.MinPrice, &newAuction.PriceDecrement, &newAuction.DecrementInterval,
		&newAuction.Quantity, &startTime, &endTime, &newAuction.Status,
		&newAuction.WinnerID, &newAuction.CreatedAt, &newAuction.UpdatedAt)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("查询拍卖信息失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("查询拍卖信息失败: %v", err),
		})
		return
	}

	// 处理可能为NULL的时间字段
	if startTime.Valid {
		newAuction.StartTime = &startTime.Time
	}
	if endTime.Valid {
		newAuction.EndTime = &endTime.Time
	}

	logger.Info("auction", fmt.Sprintf("创建荷兰钟拍卖成功，ID: %d，物品类型: %s，数量: %d\n", newAuction.ID, newAuction.ItemType, newAuction.Quantity))

	// 返回成功的JSON响应
	response := map[string]interface{}{
		"success": true,
		"message": "拍卖创建成功",
		"auction": newAuction,
	}
	json.NewEncoder(w).Encode(response)
}

// 获取所有荷兰钟拍卖
func GetAuctions(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "获取荷兰钟拍卖列表请求\n")

	// 统一设置响应头
	w.Header().Set("Content-Type", "application/json")

	rows, err := db.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions ORDER BY created_at DESC`)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("获取荷兰钟拍卖列表失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "数据库查询失败",
		})
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
			logger.Info("auction", fmt.Sprintf("处理数据扫描失败: %v\n", err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "处理数据失败",
			})
			return
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

	// 创建一个自定义的拍卖结构用于JSON序列化，处理WinnerID的NULL值
	type JSONAuction struct {
		ID                int        `json:"id"`
		ItemType          string     `json:"itemType"`
		InitialPrice      float64    `json:"initialPrice"`
		CurrentPrice      float64    `json:"currentPrice"`
		MinPrice          float64    `json:"minPrice"`
		PriceDecrement    float64    `json:"priceDecrement"`
		DecrementInterval int        `json:"decrementInterval"`
		Quantity          int        `json:"quantity"`
		StartTime         *time.Time `json:"startTime"`
		EndTime           *time.Time `json:"endTime"`
		Status            string     `json:"status"`
		WinnerID          *int       `json:"winnerId"`
		CreatedAt         time.Time  `json:"created_at"`
		UpdatedAt         time.Time  `json:"updated_at"`
	}

	var jsonAuctions []JSONAuction
	for _, auction := range auctions {
		var winnerIDPtr *int
		if auction.WinnerID.Valid {
			winnerID := int(auction.WinnerID.Int64)
			winnerIDPtr = &winnerID
		}

		jsonAuction := JSONAuction{
			ID:                auction.ID,
			ItemType:          auction.ItemType,
			InitialPrice:      auction.InitialPrice,
			CurrentPrice:      auction.CurrentPrice,
			MinPrice:          auction.MinPrice,
			PriceDecrement:    auction.PriceDecrement,
			DecrementInterval: auction.DecrementInterval,
			Quantity:          auction.Quantity,
			StartTime:         auction.StartTime,
			EndTime:           auction.EndTime,
			Status:            auction.Status,
			WinnerID:          winnerIDPtr,
			CreatedAt:         auction.CreatedAt,
			UpdatedAt:         auction.UpdatedAt,
		}

		jsonAuctions = append(jsonAuctions, jsonAuction)
	}

	logger.Info("auction", fmt.Sprintf("获取荷兰钟拍卖列表成功，共 %d 条记录\n", len(jsonAuctions)))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"auctions": jsonAuctions,
	})
}

// 获取单个荷兰钟拍卖
func GetAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "获取单个荷兰钟拍卖请求\n")

	// 统一设置响应头
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		logger.Info("auction", fmt.Sprintf("获取单个荷兰钟拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("获取单个荷兰钟拍卖，解析JSON失败: %v\n", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "请求数据解析失败",
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("auction", "获取单个荷兰钟拍卖，拍卖ID无效\n")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖ID无效",
		})
		return
	}

	var auction Auction
	var startTime, endTime sql.NullTime
	err = db.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE id = ?`, data.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Info("auction", fmt.Sprintf("获取单个荷兰钟拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "拍卖不存在",
			})
		} else {
			logger.Info("auction", fmt.Sprintf("获取单个荷兰钟拍卖失败，数据库查询错误: %v\n", err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "数据库查询失败",
			})
		}
		return
	}

	// 处理可能为NULL的时间字段
	if startTime.Valid {
		auction.StartTime = &startTime.Time
	}
	if endTime.Valid {
		auction.EndTime = &endTime.Time
	}

	// 创建一个自定义的拍卖结构用于JSON序列化，处理WinnerID的NULL值
	type JSONAuction struct {
		ID                int        `json:"id"`
		ItemType          string     `json:"itemType"`
		InitialPrice      float64    `json:"initialPrice"`
		CurrentPrice      float64    `json:"currentPrice"`
		MinPrice          float64    `json:"minPrice"`
		PriceDecrement    float64    `json:"priceDecrement"`
		DecrementInterval int        `json:"decrementInterval"`
		Quantity          int        `json:"quantity"`
		StartTime         *time.Time `json:"startTime"`
		EndTime           *time.Time `json:"endTime"`
		Status            string     `json:"status"`
		WinnerID          *int       `json:"winnerId"`
		CreatedAt         time.Time  `json:"created_at"`
		UpdatedAt         time.Time  `json:"updated_at"`
	}

	var winnerIDPtr *int
	if auction.WinnerID.Valid {
		winnerID := int(auction.WinnerID.Int64)
		winnerIDPtr = &winnerID
	}

	jsonAuction := JSONAuction{
		ID:                auction.ID,
		ItemType:          auction.ItemType,
		InitialPrice:      auction.InitialPrice,
		CurrentPrice:      auction.CurrentPrice,
		MinPrice:          auction.MinPrice,
		PriceDecrement:    auction.PriceDecrement,
		DecrementInterval: auction.DecrementInterval,
		Quantity:          auction.Quantity,
		StartTime:         auction.StartTime,
		EndTime:           auction.EndTime,
		Status:            auction.Status,
		WinnerID:          winnerIDPtr,
		CreatedAt:         auction.CreatedAt,
		UpdatedAt:         auction.UpdatedAt,
	}

	logger.Info("auction", fmt.Sprintf("获取单个荷兰钟拍卖成功，ID: %d，物品类型: %s\n", auction.ID, auction.ItemType))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "获取拍卖成功",
		"auction": jsonAuction,
	})
}

// 开始荷兰钟拍卖
func StartAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "启动荷兰钟拍卖请求\n")

	var currentTime time.Time

	// 统一设置响应头
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖，解析JSON失败: %v\n", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "请求数据解析失败",
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("auction", "启动荷兰钟拍卖失败，拍卖ID无效\n")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖ID无效",
		})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖，事务开始失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "事务开始失败",
		})
		return
	}

	// 检查拍卖是否存在
	var auction Auction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE id = ?`, data.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
			tx.Rollback()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "拍卖不存在",
			})
		} else {
			logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖，查询拍卖信息失败: %v\n", err))
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "数据库查询失败",
			})
		}
		return
	}

	// 检查拍卖状态
	if auction.Status != "pending" {
		logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖失败，拍卖ID %d 状态不是待启动状态\n", data.AuctionID))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖状态不是待启动状态",
		})
		return
	}

	// 设置开始时间和状态
	currentTime = timeservice.SyncNow()
	startTimeValue := currentTime
	endTimeValue := currentTime.Add(time.Duration(auction.DecrementInterval) * time.Second * time.Duration(int((auction.InitialPrice-auction.MinPrice)/auction.PriceDecrement)))

	// 更新拍卖状态
	currentTime = timeservice.SyncNow()
	_, err = tx.Exec(`
		UPDATE auctions 
		SET status = 'active', start_time = ?, end_time = ?, current_price = ?, updated_at = ? 
		WHERE id = ?`,
		startTimeValue, endTimeValue, auction.InitialPrice, currentTime, data.AuctionID)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖，更新拍卖状态失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "更新拍卖状态失败",
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖，事务提交失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "事务提交失败",
		})
		return
	}

	// 获取更新后的拍卖信息
	var updatedAuction Auction
	var startTime2, endTime2 sql.NullTime
	err = db.QueryRow(`
	SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
	decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
	FROM auctions WHERE id = ?`, data.AuctionID).Scan(
		&updatedAuction.ID, &updatedAuction.ItemType, &updatedAuction.InitialPrice, &updatedAuction.CurrentPrice,
		&updatedAuction.MinPrice, &updatedAuction.PriceDecrement, &updatedAuction.DecrementInterval,
		&updatedAuction.Quantity, &startTime2, &endTime2, &updatedAuction.Status,
		&updatedAuction.WinnerID, &updatedAuction.CreatedAt, &updatedAuction.UpdatedAt)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖，获取更新后的拍卖信息失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "获取更新后的拍卖信息失败",
		})
		return
	}

	// 处理可能为NULL的时间字段
	if startTime2.Valid {
		updatedAuction.StartTime = &startTime2.Time
	}
	if endTime2.Valid {
		updatedAuction.EndTime = &endTime2.Time
	}

	// 创建一个自定义的拍卖结构用于JSON序列化，处理WinnerID的NULL值
	type JSONAuction struct {
		ID                int        `json:"id"`
		ItemType          string     `json:"itemType"`
		InitialPrice      float64    `json:"initialPrice"`
		CurrentPrice      float64    `json:"currentPrice"`
		MinPrice          float64    `json:"minPrice"`
		PriceDecrement    float64    `json:"priceDecrement"`
		DecrementInterval int        `json:"decrementInterval"`
		Quantity          int        `json:"quantity"`
		StartTime         *time.Time `json:"startTime"`
		EndTime           *time.Time `json:"endTime"`
		Status            string     `json:"status"`
		WinnerID          *int       `json:"winnerId"`
		CreatedAt         time.Time  `json:"created_at"`
		UpdatedAt         time.Time  `json:"updated_at"`
	}

	var winnerIDPtr *int
	if updatedAuction.WinnerID.Valid {
		winnerID := int(updatedAuction.WinnerID.Int64)
		winnerIDPtr = &winnerID
	}

	jsonAuction := JSONAuction{
		ID:                updatedAuction.ID,
		ItemType:          updatedAuction.ItemType,
		InitialPrice:      updatedAuction.InitialPrice,
		CurrentPrice:      updatedAuction.CurrentPrice,
		MinPrice:          updatedAuction.MinPrice,
		PriceDecrement:    updatedAuction.PriceDecrement,
		DecrementInterval: updatedAuction.DecrementInterval,
		Quantity:          updatedAuction.Quantity,
		StartTime:         updatedAuction.StartTime,
		EndTime:           updatedAuction.EndTime,
		Status:            updatedAuction.Status,
		WinnerID:          winnerIDPtr,
		CreatedAt:         updatedAuction.CreatedAt,
		UpdatedAt:         updatedAuction.UpdatedAt,
	}

	logger.Info("auction", fmt.Sprintf("启动荷兰钟拍卖成功，ID: %d，物品类型: %s，数量: %d\n", updatedAuction.ID, updatedAuction.ItemType, updatedAuction.Quantity))

	// 启动价格递减定时器
	StartAuctionPriceDecrementTimer(db)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"auction": jsonAuction,
		"message": "拍卖已开始",
	})
}

// 提交荷兰钟竞价
func CommitAuctionBid(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "提交荷兰钟竞价请求\n")

	var currentTime time.Time

	// 统一设置响应头
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价失败，不支持的请求方法: %s\n", r.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "不支持的请求方法",
		})
		return
	}

	// 解析竞价数据
	var bid struct {
		AuctionID int `json:"auction_id"`
		BidAmount int `json:"bid_amount"`
	}
	err := json.NewDecoder(r.Body).Decode(&bid)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，解析JSON失败: %v\n", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "请求数据解析失败",
		})
		return
	}

	// 验证输入
	if bid.AuctionID <= 0 {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，拍卖ID %d 无效\n", bid.AuctionID))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖ID无效",
		})
		return
	}

	if bid.BidAmount <= 0 {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，竞价金额 %d 无效\n", bid.BidAmount))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "竞价金额必须为正数",
		})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，事务开始失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "事务开始失败",
		})
		return
	}

	// 获取拍卖信息
	var auction Auction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE id = ?`, bid.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价失败，拍卖ID %d 不存在\n", bid.AuctionID))
			tx.Rollback()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "拍卖不存在",
			})
		} else {
			logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，获取拍卖信息失败: %v\n", err))
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "数据库查询失败",
			})
		}
		return
	}

	// 检查拍卖状态
	if auction.Status != "active" {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价失败，拍卖ID %d 未启动\n", bid.AuctionID))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖未启动",
		})
		return
	}

	// 检查拍卖是否已结束
	if auction.EndTime != nil && timeservice.SyncNow().After(*auction.EndTime) {
		// 更新拍卖状态为已完成
		currentTime = timeservice.SyncNow()
		_, err = tx.Exec("UPDATE auctions SET status = 'completed', updated_at = ? WHERE id = ?", currentTime, bid.AuctionID)
		if err != nil {
			logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，更新拍卖状态失败: %v\n", err))
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "更新拍卖状态失败",
			})
			return
		}

		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，拍卖ID %d 已结束，更新状态为已完成\n", bid.AuctionID))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖已结束",
		})
		return
	}

	// 检查竞价金额是否在有效范围内
	if float64(bid.BidAmount) > auction.CurrentPrice || float64(bid.BidAmount) < auction.MinPrice {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价失败，竞价金额 %d 不在有效价格范围内\n", bid.BidAmount))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "竞价金额不在有效价格范围内",
		})
		return
	}

	// 获取当前价格
	currentPrice := float64(bid.BidAmount)

	// 插入竞价记录
	result, err := tx.Exec(`
		INSERT INTO auction_bids (auction_id, user_id, price, quantity, status) 
		VALUES (?, ?, ?, ?, 'accepted')`,
		bid.AuctionID, 1, currentPrice, auction.Quantity)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，插入竞价记录失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "插入竞价记录失败",
		})
		return
	}

	// 获取竞价ID
	bidID, err := result.LastInsertId()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，获取竞价ID失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "获取竞价ID失败",
		})
		return
	}

	// 更新拍卖状态为已完成
	currentTime = timeservice.SyncNow()
	_, err = tx.Exec(`
		UPDATE auctions 
		SET status = 'completed', winner_id = ?, updated_at = ? 
		WHERE id = ?`,
		1, currentTime, bid.AuctionID)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，更新拍卖状态失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "更新拍卖状态失败",
		})
		return
	}

	// 更新用户背包
	var backpack Backpack
	err = tx.QueryRow("SELECT id, apple, wood, created_at, updated_at FROM backpack ORDER BY id DESC LIMIT 1").Scan(
		&backpack.ID, &backpack.Apple, &backpack.Wood, &backpack.CreatedAt, &backpack.UpdatedAt)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，获取用户背包失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "获取用户背包失败",
		})
		return
	}

	// 根据物品类型更新背包
	switch auction.ItemType {
	case "apple":
		backpack.Apple += auction.Quantity
	case "wood":
		backpack.Wood += auction.Quantity
	}

	// 更新背包
	currentTime = timeservice.SyncNow()
	_, err = tx.Exec("UPDATE backpack SET apple = ?, wood = ?, updated_at = ? WHERE id = ?",
		backpack.Apple, backpack.Wood, currentTime, backpack.ID)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，更新用户背包失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "更新用户背包失败",
		})
		return
	}

	// 获取当前余额
	var balance struct {
		ID        int       `json:"id"`
		Amount    float64   `json:"amount"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	err = tx.QueryRow("SELECT id, amount, updated_at FROM balance ORDER BY id DESC LIMIT 1").Scan(&balance.ID, &balance.Amount, &balance.UpdatedAt)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，获取当前余额失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "获取当前余额失败",
		})
		return
	}

	// 计算总价格
	totalPrice := currentPrice * float64(auction.Quantity)

	// 检查余额是否足够
	if balance.Amount < totalPrice {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，余额不足: %v\n", totalPrice))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "余额不足",
		})
		return
	}

	// 更新余额
	currentTime = timeservice.SyncNow()
	newBalance := balance.Amount - totalPrice
	_, err = tx.Exec("UPDATE balance SET amount = ?, updated_at = ? WHERE id = ?",
		newBalance, currentTime, balance.ID)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，更新余额失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "更新余额失败",
		})
		return
	}

	// 添加交易记录
	// 隐私数据
	currentTime = timeservice.SyncNow()
	_, err = tx.Exec(
		"INSERT INTO transactions (transaction_time, our_bank_account_name, counterparty_alias, our_bank_name, counterparty_bank, expense_amount, income_amount, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		currentTime, "玩家", "萌铺子市场", "玩家银行", "萌铺子市场银行", totalPrice, 0, fmt.Sprintf("荷兰钟拍卖买入%s", auction.ItemType), currentTime)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，添加交易记录失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "添加交易记录失败",
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，事务提交失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "事务提交失败",
		})
		return
	}

	// 获取竞价记录
	var newBid AuctionBid
	err = db.QueryRow(`
		SELECT id, auction_id, user_id, price, quantity, status, created_at 
		FROM auction_bids WHERE id = ?`, bidID).Scan(
		&newBid.ID, &newBid.AuctionID, &newBid.UserID, &newBid.Price,
		&newBid.Quantity, &newBid.Status, &newBid.CreatedAt)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价，获取竞价记录失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "获取竞价记录失败",
		})
		return
	}

	logger.Info("auction", fmt.Sprintf("提交荷兰钟竞价成功，ID: %d，价格: %.2f，物品类型: %s，数量: %d\n", newBid.ID, currentPrice, auction.ItemType, auction.Quantity))

	// 停止价格递减定时器
	StopAuctionPriceDecrementTimer()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"bid":     newBid,
		"message": fmt.Sprintf("成功以 %.2f 的价格买入 %d 个%s", currentPrice, auction.Quantity, auction.ItemType),
	})
}

// 取消荷兰钟拍卖
func CancelAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "取消荷兰钟拍卖请求\n")

	var currentTime time.Time

	// 统一设置响应头
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖，解析JSON失败: %v\n", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "请求数据解析失败",
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖失败，拍卖ID %d 无效\n", data.AuctionID))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖ID无效",
		})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖，事务开始失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "事务开始失败",
		})
		return
	}

	// 检查拍卖是否存在
	var auction Auction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE id = ?`, data.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖，获取拍卖信息失败: %v\n", err))
		if err == sql.ErrNoRows {
			logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
			tx.Rollback()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "拍卖不存在",
			})
		} else {
			logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖，获取拍卖信息失败: %v\n", err))
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "数据库查询失败",
			})
		}
		return
	}

	// 检查拍卖状态
	if auction.Status == "completed" {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖失败，拍卖ID %d 已完成\n", data.AuctionID))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "无法取消已完成的拍卖",
		})
		return
	}

	// 更新拍卖状态为已取消
	currentTime = timeservice.SyncNow()
	_, err = tx.Exec("UPDATE auctions SET status = 'cancelled', updated_at = ? WHERE id = ?", currentTime, data.AuctionID)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖，更新拍卖状态失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "更新拍卖状态失败",
		})
		return
	}

	// 解锁背包中的物品
	err = UnlockBackpackItems(tx, auction.ItemType, auction.Quantity)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖，解锁背包物品失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "解锁背包物品失败",
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖，事务提交失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "事务提交失败",
		})
		return
	}

	logger.Info("auction", fmt.Sprintf("取消荷兰钟拍卖成功，ID: %d，物品类型: %s，数量: %d\n", auction.ID, auction.ItemType, auction.Quantity))

	// 停止价格递减定时器
	StopAuctionPriceDecrementTimer()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "拍卖已取消，物品已返还到背包",
	})
}

// 获取卖家荷兰钟拍卖列表
func GetSellerAuctions(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "获取卖家荷兰钟拍卖列表请求\n")

	// 统一设置响应头
	w.Header().Set("Content-Type", "application/json")

	rows, err := db.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions ORDER BY created_at DESC`)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("获取卖家荷兰钟拍卖列表失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "数据库查询失败",
		})
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
			logger.Info("auction", fmt.Sprintf("获取卖家荷兰钟拍卖列表，处理数据失败: %v\n", err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "处理数据失败",
			})
			return
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

	// 创建一个自定义的拍卖结构用于JSON序列化，处理WinnerID的NULL值
	type JSONAuction struct {
		ID                int        `json:"id"`
		ItemType          string     `json:"itemType"`
		InitialPrice      float64    `json:"initialPrice"`
		CurrentPrice      float64    `json:"currentPrice"`
		MinPrice          float64    `json:"minPrice"`
		PriceDecrement    float64    `json:"priceDecrement"`
		DecrementInterval int        `json:"decrementInterval"`
		Quantity          int        `json:"quantity"`
		StartTime         *time.Time `json:"startTime"`
		EndTime           *time.Time `json:"endTime"`
		Status            string     `json:"status"`
		WinnerID          *int       `json:"winnerId"`
		CreatedAt         time.Time  `json:"created_at"`
		UpdatedAt         time.Time  `json:"updated_at"`
	}

	var jsonAuctions []JSONAuction
	for _, auction := range auctions {
		var winnerIDPtr *int
		if auction.WinnerID.Valid {
			winnerID := int(auction.WinnerID.Int64)
			winnerIDPtr = &winnerID
		}

		jsonAuction := JSONAuction{
			ID:                auction.ID,
			ItemType:          auction.ItemType,
			InitialPrice:      auction.InitialPrice,
			CurrentPrice:      auction.CurrentPrice,
			MinPrice:          auction.MinPrice,
			PriceDecrement:    auction.PriceDecrement,
			DecrementInterval: auction.DecrementInterval,
			Quantity:          auction.Quantity,
			StartTime:         auction.StartTime,
			EndTime:           auction.EndTime,
			Status:            auction.Status,
			WinnerID:          winnerIDPtr,
			CreatedAt:         auction.CreatedAt,
			UpdatedAt:         auction.UpdatedAt,
		}

		jsonAuctions = append(jsonAuctions, jsonAuction)
	}

	logger.Info("auction", fmt.Sprintf("获取卖家荷兰钟拍卖列表成功，共 %d 条记录\n", len(jsonAuctions)))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"auctions": jsonAuctions,
	})
}

// 暂停荷兰钟拍卖（下架）
func PauseAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "暂停荷兰钟拍卖请求\n")

	var currentTime time.Time

	// 统一设置响应头
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖，解析JSON失败: %v\n", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "请求数据解析失败",
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖失败，拍卖ID %d 无效\n", data.AuctionID))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖ID无效",
		})
		return
	}

	// 添加重试机制，最多重试3次
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		// 开始事务
		tx, err := db.Begin()
		if err != nil {
			logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖，事务开始失败: %v\n", err))
			if retry == maxRetries-1 {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "事务开始失败",
				})
				return
			}
			time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond) // 指数退避
			continue
		}

		// 检查拍卖是否存在
		var auction Auction
		var startTime, endTime sql.NullTime
		err = tx.QueryRow(`
			SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
			decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
			FROM auctions WHERE id = ?`, data.AuctionID).Scan(
			&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
			&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
			&auction.Quantity, &startTime, &endTime, &auction.Status,
			&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
		if err != nil {
			tx.Rollback()
			if err == sql.ErrNoRows {
				logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "拍卖不存在",
				})
			} else {
				logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖，获取拍卖信息失败: %v\n", err))
				if retry == maxRetries-1 {
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"message": "数据库查询失败",
					})
					return
				}
				time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond) // 指数退避
				continue
			}
			return
		}

		// 检查拍卖状态
		if auction.Status != "active" {
			tx.Rollback()
			logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖失败，拍卖ID %d 状态不是活跃状态\n", data.AuctionID))
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "拍卖ID不是活跃状态",
			})
			return
		}

		// 更新拍卖状态为待开始
		currentTime = timeservice.SyncNow()
		_, err = tx.Exec("UPDATE auctions SET status = 'pending', start_time = NULL, end_time = NULL, updated_at = ? WHERE id = ?", currentTime, data.AuctionID)
		if err != nil {
			tx.Rollback()
			logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖，更新拍卖状态失败: %v\n", err))
			if retry == maxRetries-1 {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "更新拍卖状态失败",
				})
				return
			}
			time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond) // 指数退避
			continue
		}

		// 提交事务
		err = tx.Commit()
		if err != nil {
			logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖，事务提交失败: %v\n", err))
			if retry == maxRetries-1 {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "事务提交失败",
				})
				return
			}
			time.Sleep(time.Duration(retry+1) * 200 * time.Millisecond) // 指数退避，时间稍长
			continue
		}

		// 事务成功提交，退出重试循环
		logger.Info("auction", fmt.Sprintf("暂停荷兰钟拍卖成功，ID: %d，物品类型: %s，数量: %d\n", auction.ID, auction.ItemType, auction.Quantity))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "拍卖已成功暂停",
		})
		return
	}
}

// 更新荷兰钟拍卖价格（定时任务调用）
func UpdateAuctionPrices(db *sql.DB) {
	logger.Info("auction", "开始更新荷兰钟拍卖价格\n")

	var currentTime time.Time

	// 获取所有活跃的拍卖
	rows, err := db.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE status = 'active'`)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("更新荷兰钟拍卖价格，获取活跃拍卖失败: %v\n", err))
		fmt.Printf("获取活跃拍卖失败: %v\n", err)
		return
	}
	defer rows.Close()

	currentTime = timeservice.SyncNow()
	updatedCount := 0

	for rows.Next() {
		var auction Auction
		var startTime, endTime sql.NullTime
		err := rows.Scan(
			&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
			&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
			&auction.Quantity, &startTime, &endTime, &auction.Status,
			&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
		if err != nil {
			logger.Info("auction", fmt.Sprintf("更新荷兰钟拍卖价格，扫描拍卖数据失败: %v\n", err))
			fmt.Printf("扫描拍卖数据失败: %v\n", err)
			continue
		}

		// 处理可能为NULL的时间字段
		if startTime.Valid {
			auction.StartTime = &startTime.Time
		}
		if endTime.Valid {
			auction.EndTime = &endTime.Time
		}

		// 检查拍卖是否已结束
		if auction.EndTime != nil && currentTime.After(*auction.EndTime) {
			// 更新拍卖状态为已完成
			currentTime = timeservice.SyncNow()
			_, err = db.Exec("UPDATE auctions SET status = 'completed', updated_at = ? WHERE id = ?", currentTime, auction.ID)
			if err != nil {
				logger.Info("auction", fmt.Sprintf("更新荷兰钟拍卖价格，更新拍卖状态为已完成失败: %v\n", err))
				fmt.Printf("更新拍卖状态为已完成失败: %v\n", err)
			} else {
				logger.Info("auction", fmt.Sprintf("拍卖ID %d 已自动结束\n", auction.ID))
				updatedCount++
			}
			continue
		}

		// 计算应该减少的价格
		if auction.StartTime == nil {
			continue
		}
		elapsed := currentTime.Sub(*auction.StartTime)
		intervals := int(elapsed.Seconds()) / auction.DecrementInterval
		newPrice := auction.InitialPrice - float64(intervals)*auction.PriceDecrement

		// 确保价格不低于最低价格
		if newPrice < auction.MinPrice {
			newPrice = auction.MinPrice
		}

		// 只有当价格有变化且变化方向正确（递减）时，才更新数据库
		// 添加价格变化方向检查，防止价格波动
		if newPrice != auction.CurrentPrice && newPrice <= auction.CurrentPrice {
			currentTime = timeservice.SyncNow()
			_, err = db.Exec("UPDATE auctions SET current_price = ?, updated_at = ? WHERE id = ?", newPrice, currentTime, auction.ID)
			if err != nil {
				logger.Info("auction", fmt.Sprintf("更新荷兰钟拍卖价格，更新拍卖价格失败: %v\n", err))
				fmt.Printf("更新拍卖价格失败: %v\n", err)
			} else {
				logger.Info("auction", fmt.Sprintf("拍卖ID %d 价格已更新: %.2f -> %.2f\n", auction.ID, auction.CurrentPrice, newPrice))
				updatedCount++
			}
		} else if newPrice > auction.CurrentPrice {
			// 记录价格异常上涨的情况
			logger.Info("auction", fmt.Sprintf("价格更新异常：计算价格 %.2f 高于当前价格 %.2f，跳过更新\n", newPrice, auction.CurrentPrice))
		}
	}

	logger.Info("auction", fmt.Sprintf("荷兰钟拍卖价格更新完成，共更新 %d 个拍卖\n", updatedCount))
}

// 获取活跃的荷兰钟拍卖列表（WebSocket使用）
func GetActiveAuctions(db *sql.DB) ([]Auction, error) {
	rows, err := db.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE status IN ('pending', 'active') ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
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
			return nil, err
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

	return auctions, nil
}

// 根据ID获取荷兰钟拍卖详情（WebSocket使用）
func GetAuctionID(db *sql.DB, auctionID int) (*Auction, error) {
	var auction Auction
	var startTime, endTime sql.NullTime

	err := db.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE id = ?`, auctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// 处理可能为NULL的时间字段
	if startTime.Valid {
		auction.StartTime = &startTime.Time
	}
	if endTime.Valid {
		auction.EndTime = &endTime.Time
	}

	return &auction, nil
}

// 处理荷兰钟竞价（WebSocket使用）
func ProcessAuctionBid(db *sql.DB, auctionID, userID int, price float64, quantity int) (bool, string, error) {
	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		return false, "事务开始失败", err
	}

	var currentTime time.Time

	// 检查拍卖是否存在且处于活跃状态
	var auction Auction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE id = ?`, auctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		tx.Rollback()
		return false, "拍卖不存在", err
	}

	// 检查拍卖状态
	if auction.Status != "active" {
		tx.Rollback()
		return false, "拍卖未开始或已结束", nil
	}

	// 检查价格是否有效
	if price < auction.CurrentPrice {
		tx.Rollback()
		return false, "竞价价格低于当前价格", nil
	}

	// 检查数量是否有效
	if quantity <= 0 || quantity > auction.Quantity {
		tx.Rollback()
		return false, "竞价数量无效", nil
	}

	// 记录竞价
	currentTime = timeservice.SyncNow()
	result, err := tx.Exec(`
		INSERT INTO auction_bids (auction_id, user_id, price, quantity, status, created_at) 
		VALUES (?, ?, ?, ?, 'accepted', ?)`,
		auctionID, userID, price, quantity, currentTime)
	if err != nil {
		tx.Rollback()
		return false, "记录竞价失败", err
	}

	bidID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return false, "获取竞价ID失败", err
	}

	// 更新拍卖状态为已完成，设置中标者
	currentTime = timeservice.SyncNow()
	_, err = tx.Exec(`
		UPDATE auctions 
		SET status = 'completed', winner_id = ?, end_time = ?, updated_at = ? 
		WHERE id = ?`, userID, currentTime, currentTime, auctionID)
	if err != nil {
		tx.Rollback()
		return false, "更新拍卖状态失败", err
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		return false, "事务提交失败", err
	}

	logger.Info("auction", fmt.Sprintf("荷兰钟竞价成功，拍卖ID: %d，用户ID: %d，价格: %.2f，数量: %d，竞价ID: %d\n",
		auctionID, userID, price, quantity, bidID))

	return true, "竞价成功", nil
}

// 重新激活拍卖 - 允许卖家将已完成、已取消的拍卖状态更新为pending
func ReactivateAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("auction", "重新激活拍卖请求\n")

	var currentTime time.Time

	// 统一设置响应头
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖，解析JSON失败: %v\n", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "请求数据解析失败",
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖失败，拍卖ID %d 无效\n", data.AuctionID))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "拍卖ID无效",
		})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖，事务开始失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "事务开始失败",
		})
		return
	}

	// 检查拍卖是否存在
	var auction Auction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM auctions WHERE id = ?`, data.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖，获取拍卖信息失败: %v\n", err))
		if err == sql.ErrNoRows {
			logger.Info("auction", fmt.Sprintf("重新激活拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
			tx.Rollback()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "拍卖不存在",
			})
		} else {
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "数据库查询失败",
			})
		}
		return
	}

	// 检查拍卖状态，只允许重新激活已完成或已取消的拍卖
	if auction.Status != "completed" && auction.Status != "cancelled" {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖失败，拍卖ID %d 状态为 %s，不允许重新激活\n", data.AuctionID, auction.Status))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "只能重新激活已完成或已取消的拍卖",
		})
		return
	}

	// 检查背包中是否有足够的物品
	err = LockBackpackItems(tx, auction.ItemType, auction.Quantity)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖，锁定背包物品失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 重置拍卖状态为pending，并重置当前价格为初始价格
	currentTime = timeservice.SyncNow()
	_, err = tx.Exec(`
		UPDATE auctions 
		SET status = 'pending', current_price = initial_price, start_time = NULL, end_time = NULL, winner_id = NULL, updated_at = ? 
		WHERE id = ?`, currentTime, data.AuctionID)
	if err != nil {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖，更新拍卖状态失败: %v\n", err))
		tx.Rollback()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "更新拍卖状态失败",
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("auction", fmt.Sprintf("重新激活拍卖，事务提交失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "事务提交失败",
		})
		return
	}

	logger.Info("auction", fmt.Sprintf("重新激活拍卖成功，ID: %d，物品类型: %s，数量: %d\n", auction.ID, auction.ItemType, auction.Quantity))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "拍卖已重新激活，可以再次开始",
	})
}
