package market

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"own-1Pixel/backend/go/logger"

	_ "modernc.org/sqlite"
)

// 荷兰钟拍卖结构
type DutchAuction struct {
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
type DutchBid struct {
	ID        int       `json:"id"`
	AuctionID int       `json:"auctionId"` //
	UserID    int       `json:"userId"`
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"` // 状态：pending, accepted, rejected
	CreatedAt time.Time `json:"created_at"`
}

// 初始化荷兰钟拍卖数据库表
func InitDutchAuctionDatabase(db *sql.DB) error {
	logger.Info("dutch_auction", "初始化荷兰钟拍卖数据库表\n")

	// 创建荷兰钟拍卖表
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS dutch_auctions (
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("创建荷兰钟拍卖表失败: %v\n", err))
		return err
	}

	// 创建荷兰钟竞价记录表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS dutch_bids (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			auction_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			price REAL NOT NULL,
			quantity INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (auction_id) REFERENCES dutch_auctions(id)
		)
	`)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("创建荷兰钟竞价记录表失败: %v\n", err))
		return err
	}

	logger.Info("dutch_auction", "荷兰钟拍卖数据库表初始化完成\n")
	return nil
}

// 创建荷兰钟拍卖
func CreateDutchAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("dutch_auction", "创建荷兰钟拍卖请求\n")

	if r.Method != "POST" {
		logger.Info("dutch_auction", fmt.Sprintf("创建荷兰钟拍卖请求失败，不支持的请求方法: %s\n", r.Method))
		http.Error(w, "不支持的请求方法", http.StatusMethodNotAllowed)
		return
	}

	var auction DutchAuction
	err := json.NewDecoder(r.Body).Decode(&auction)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("解析荷兰钟拍卖JSON失败: %v\n", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 验证输入
	if auction.ItemType != "apple" && auction.ItemType != "wood" {
		http.Error(w, "无效的物品类型", http.StatusBadRequest)
		return
	}

	if auction.InitialPrice <= 0 || auction.MinPrice <= 0 || auction.PriceDecrement <= 0 {
		http.Error(w, "初始价格、最低价格和价格递减量必须为正数", http.StatusBadRequest)
		return
	}

	if auction.InitialPrice < auction.MinPrice {
		http.Error(w, "初始价格必须大于或等于最低价格", http.StatusBadRequest)
		return
	}

	if auction.Quantity <= 0 {
		http.Error(w, "数量必须为正数", http.StatusBadRequest)
		return
	}

	if auction.DecrementInterval <= 0 {
		http.Error(w, "价格递减间隔必须为正数", http.StatusBadRequest)
		return
	}

	// 设置默认值
	auction.Status = "pending"
	auction.CurrentPrice = auction.InitialPrice

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("启动事务失败: %v\n", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 插入拍卖记录
	result, err := tx.Exec(`
		INSERT INTO dutch_auctions 
		(item_type, initial_price, current_price, min_price, price_decrement, decrement_interval, quantity, start_time, end_time, status) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		auction.ItemType, auction.InitialPrice, auction.CurrentPrice, auction.MinPrice,
		auction.PriceDecrement, auction.DecrementInterval, auction.Quantity,
		nil, nil, auction.Status)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("插入拍卖记录失败: %v\n", err))
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取新插入的拍卖ID
	auctionID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		logger.Info("dutch_auction", fmt.Sprintf("获取拍卖ID失败: %v\n", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交事务失败: %v\n", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取完整的拍卖信息
	var newAuction DutchAuction
	var startTime, endTime sql.NullTime
	err = db.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions WHERE id = ?`, auctionID).Scan(
		&newAuction.ID, &newAuction.ItemType, &newAuction.InitialPrice, &newAuction.CurrentPrice,
		&newAuction.MinPrice, &newAuction.PriceDecrement, &newAuction.DecrementInterval,
		&newAuction.Quantity, &startTime, &endTime, &newAuction.Status,
		&newAuction.WinnerID, &newAuction.CreatedAt, &newAuction.UpdatedAt)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("查询拍卖信息失败: %v\n", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 处理可能为NULL的时间字段
	if startTime.Valid {
		newAuction.StartTime = &startTime.Time
	}
	if endTime.Valid {
		newAuction.EndTime = &endTime.Time
	}

	w.Header().Set("Content-Type", "application/json")
	logger.Info("dutch_auction", fmt.Sprintf("创建荷兰钟拍卖成功，ID: %d，物品类型: %s，数量: %d\n", newAuction.ID, newAuction.ItemType, newAuction.Quantity))
	json.NewEncoder(w).Encode(newAuction)
}

// 获取所有荷兰钟拍卖
func GetDutchAuctions(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("dutch_auction", "获取荷兰钟拍卖列表请求\n")

	rows, err := db.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions ORDER BY created_at DESC`)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("获取荷兰钟拍卖列表失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("数据库查询失败: %v", err),
		})
		return
	}
	defer rows.Close()

	var auctions []DutchAuction
	for rows.Next() {
		var auction DutchAuction
		var startTime, endTime sql.NullTime
		err := rows.Scan(
			&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
			&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
			&auction.Quantity, &startTime, &endTime, &auction.Status,
			&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
		if err != nil {
			logger.Info("dutch_auction", fmt.Sprintf("处理数据扫描失败: %v\n", err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("处理数据失败: %v", err),
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

	logger.Info("dutch_auction", fmt.Sprintf("获取荷兰钟拍卖列表成功，共 %d 条记录\n", len(jsonAuctions)))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auctions": jsonAuctions,
	})
}

// 获取单个荷兰钟拍卖
func GetDutchAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("dutch_auction", "获取单个荷兰钟拍卖请求\n")

	if r.Method != "POST" {
		logger.Info("dutch_auction", fmt.Sprintf("获取单个荷兰钟拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("获取单个荷兰钟拍卖，解析JSON失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("请求数据解析失败: %v", err),
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("dutch_auction", "获取单个荷兰钟拍卖，拍卖ID无效\n")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖ID无效",
		})
		return
	}

	var auction DutchAuction
	var startTime, endTime sql.NullTime
	err = db.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions WHERE id = ?`, data.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == sql.ErrNoRows {
			logger.Info("dutch_auction", fmt.Sprintf("获取单个荷兰钟拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "拍卖不存在",
			})
		} else {
			logger.Info("dutch_auction", fmt.Sprintf("获取单个荷兰钟拍卖失败，数据库查询错误: %v\n", err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("数据库查询失败: %v", err),
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

	logger.Info("dutch_auction", fmt.Sprintf("获取单个荷兰钟拍卖成功，ID: %d，物品类型: %s\n", auction.ID, auction.ItemType))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auction": jsonAuction,
	})
}

// 开始荷兰钟拍卖
func StartDutchAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("dutch_auction", "启动荷兰钟拍卖请求\n")

	if r.Method != "POST" {
		logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖，解析JSON失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("请求数据解析失败: %v", err),
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("dutch_auction", "启动荷兰钟拍卖失败，拍卖ID无效\n")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖ID无效",
		})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖，事务开始失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("事务开始失败: %v", err),
		})
		return
	}

	// 检查拍卖是否存在
	var auction DutchAuction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions WHERE id = ?`, data.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == sql.ErrNoRows {
			logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
			tx.Rollback()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "拍卖不存在",
			})
		} else {
			logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖，查询拍卖信息失败: %v\n", err))
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("数据库查询失败: %v", err),
			})
		}
		return
	}

	// 检查拍卖状态
	if auction.Status != "pending" {
		logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖失败，拍卖ID %d 状态不是待启动状态\n", data.AuctionID))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖状态不是待启动状态",
		})
		return
	}

	// 设置开始时间和状态
	now := time.Now()
	startTimeValue := now
	endTimeValue := now.Add(time.Duration(auction.DecrementInterval) * time.Second * time.Duration(int((auction.InitialPrice-auction.MinPrice)/auction.PriceDecrement)))

	// 更新拍卖状态
	_, err = tx.Exec(`
		UPDATE dutch_auctions 
		SET status = 'active', start_time = ?, end_time = ?, current_price = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?`,
		startTimeValue, endTimeValue, auction.InitialPrice, data.AuctionID)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖，更新拍卖状态失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("更新拍卖状态失败: %v", err),
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖，事务提交失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("事务提交失败: %v", err),
		})
		return
	}

	// 获取更新后的拍卖信息
	var updatedAuction DutchAuction
	var startTime2, endTime2 sql.NullTime
	err = db.QueryRow(`
	SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
	decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
	FROM dutch_auctions WHERE id = ?`, data.AuctionID).Scan(
		&updatedAuction.ID, &updatedAuction.ItemType, &updatedAuction.InitialPrice, &updatedAuction.CurrentPrice,
		&updatedAuction.MinPrice, &updatedAuction.PriceDecrement, &updatedAuction.DecrementInterval,
		&updatedAuction.Quantity, &startTime2, &endTime2, &updatedAuction.Status,
		&updatedAuction.WinnerID, &updatedAuction.CreatedAt, &updatedAuction.UpdatedAt)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖，获取更新后的拍卖信息失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("获取更新后的拍卖信息失败: %v", err),
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

	logger.Info("dutch_auction", fmt.Sprintf("启动荷兰钟拍卖成功，ID: %d，物品类型: %s，数量: %d\n", updatedAuction.ID, updatedAuction.ItemType, updatedAuction.Quantity))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"auction": jsonAuction,
		"message": "拍卖已开始",
	})
}

// 提交荷兰钟竞价
func PlaceDutchBid(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("dutch_auction", "提交荷兰钟竞价请求\n")

	if r.Method != "POST" {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价失败，不支持的请求方法: %s\n", r.Method))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不支持的请求方法",
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
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，解析JSON失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("请求数据解析失败: %v", err),
		})
		return
	}

	// 验证输入
	if bid.AuctionID <= 0 {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，拍卖ID %d 无效\n", bid.AuctionID))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖ID无效",
		})
		return
	}

	if bid.BidAmount <= 0 {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，竞价金额 %d 无效\n", bid.BidAmount))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "竞价金额必须为正数",
		})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，事务开始失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("事务开始失败: %v", err),
		})
		return
	}

	// 获取拍卖信息
	var auction DutchAuction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions WHERE id = ?`, bid.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == sql.ErrNoRows {
			logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价失败，拍卖ID %d 不存在\n", bid.AuctionID))
			tx.Rollback()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "拍卖不存在",
			})
		} else {
			logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，获取拍卖信息失败: %v\n", err))
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("数据库查询失败: %v", err),
			})
		}
		return
	}

	// 检查拍卖状态
	if auction.Status != "active" {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价失败，拍卖ID %d 未启动\n", bid.AuctionID))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖未启动",
		})
		return
	}

	// 检查拍卖是否已结束
	if auction.EndTime != nil && time.Now().After(*auction.EndTime) {
		// 更新拍卖状态为已完成
		_, err = tx.Exec("UPDATE dutch_auctions SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id = ?", bid.AuctionID)
		if err != nil {
			logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，更新拍卖状态失败: %v\n", err))
			tx.Rollback()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("更新拍卖状态失败: %v", err),
			})
			return
		}

		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，拍卖ID %d 已结束，更新状态为已完成\n", bid.AuctionID))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖已结束",
		})
		return
	}

	// 检查竞价金额是否在有效范围内
	if float64(bid.BidAmount) > auction.CurrentPrice || float64(bid.BidAmount) < auction.MinPrice {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价失败，竞价金额 %d 不在有效价格范围内\n", bid.BidAmount))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "竞价金额不在有效价格范围内",
		})
		return
	}

	// 获取当前价格
	currentPrice := float64(bid.BidAmount)

	// 插入竞价记录
	result, err := tx.Exec(`
		INSERT INTO dutch_bids (auction_id, user_id, price, quantity, status) 
		VALUES (?, ?, ?, ?, 'accepted')`,
		bid.AuctionID, 1, currentPrice, auction.Quantity)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，插入竞价记录失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("插入竞价记录失败: %v", err),
		})
		return
	}

	// 获取竞价ID
	bidID, err := result.LastInsertId()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，获取竞价ID失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("获取竞价ID失败: %v", err),
		})
		return
	}

	// 更新拍卖状态为已完成
	_, err = tx.Exec(`
		UPDATE dutch_auctions 
		SET status = 'completed', winner_id = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?`,
		1, bid.AuctionID)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，更新拍卖状态失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("更新拍卖状态失败: %v", err),
		})
		return
	}

	// 更新用户背包
	var backpack Backpack
	err = tx.QueryRow("SELECT id, apple, wood, created_at, updated_at FROM backpack ORDER BY id DESC LIMIT 1").Scan(
		&backpack.ID, &backpack.Apple, &backpack.Wood, &backpack.CreatedAt, &backpack.UpdatedAt)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，获取用户背包失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("获取用户背包失败: %v", err),
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
	_, err = tx.Exec("UPDATE backpack SET apple = ?, wood = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		backpack.Apple, backpack.Wood, backpack.ID)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，更新用户背包失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("更新用户背包失败: %v", err),
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
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，获取当前余额失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("获取当前余额失败: %v", err),
		})
		return
	}

	// 计算总价格
	totalPrice := currentPrice * float64(auction.Quantity)

	// 检查余额是否足够
	if balance.Amount < totalPrice {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，余额不足: %v\n", totalPrice))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "余额不足",
		})
		return
	}

	// 更新余额
	newBalance := balance.Amount - totalPrice
	_, err = tx.Exec("UPDATE balance SET amount = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		newBalance, balance.ID)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，更新余额失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("更新余额失败: %v", err),
		})
		return
	}

	// 添加交易记录
	// 隐私数据
	_, err = tx.Exec(
		"INSERT INTO transactions (transaction_time, our_bank_account_name, counterparty_alias, our_bank_name, counterparty_bank, expense_amount, income_amount, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		time.Now(), "玩家", "萌铺子市场", "玩家银行", "萌铺子市场银行", totalPrice, 0, fmt.Sprintf("荷兰钟拍卖买入%s", auction.ItemType))
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，添加交易记录失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("添加交易记录失败: %v", err),
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，事务提交失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("事务提交失败: %v", err),
		})
		return
	}

	// 获取竞价记录
	var newBid DutchBid
	err = db.QueryRow(`
		SELECT id, auction_id, user_id, price, quantity, status, created_at 
		FROM dutch_bids WHERE id = ?`, bidID).Scan(
		&newBid.ID, &newBid.AuctionID, &newBid.UserID, &newBid.Price,
		&newBid.Quantity, &newBid.Status, &newBid.CreatedAt)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价，获取竞价记录失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("获取竞价记录失败: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	logger.Info("dutch_auction", fmt.Sprintf("提交荷兰钟竞价成功，ID: %d，价格: %.2f，物品类型: %s，数量: %d\n", newBid.ID, currentPrice, auction.ItemType, auction.Quantity))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"bid":     newBid,
		"message": fmt.Sprintf("成功以 %.2f 的价格买入 %d 个%s", currentPrice, auction.Quantity, auction.ItemType),
	})
}

// 取消荷兰钟拍卖
func CancelDutchAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("dutch_auction", "取消荷兰钟拍卖请求\n")

	if r.Method != "POST" {
		logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖，解析JSON失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("请求数据解析失败: %v", err),
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖失败，拍卖ID %d 无效\n", data.AuctionID))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖ID无效",
		})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖，事务开始失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("事务开始失败: %v", err),
		})
		return
	}

	// 检查拍卖是否存在
	var auction DutchAuction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions WHERE id = ?`, data.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖，获取拍卖信息失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		if err == sql.ErrNoRows {
			logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
			tx.Rollback()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "拍卖不存在",
			})
		} else {
			logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖，获取拍卖信息失败: %v\n", err))
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("数据库查询失败: %v", err),
			})
		}
		return
	}

	// 检查拍卖状态
	if auction.Status == "completed" {
		logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖失败，拍卖ID %d 已完成\n", data.AuctionID))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "无法取消已完成的拍卖",
		})
		return
	}

	// 更新拍卖状态为已取消
	_, err = tx.Exec("UPDATE dutch_auctions SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP WHERE id = ?", data.AuctionID)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖，更新拍卖状态失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("更新拍卖状态失败: %v", err),
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖，事务提交失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("事务提交失败: %v", err),
		})
		return
	}

	logger.Info("dutch_auction", fmt.Sprintf("取消荷兰钟拍卖成功，ID: %d，物品类型: %s，数量: %d\n", auction.ID, auction.ItemType, auction.Quantity))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "拍卖已取消",
	})
}

// 获取卖家荷兰钟拍卖列表
func GetSellerDutchAuctions(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("dutch_auction", "获取卖家荷兰钟拍卖列表请求\n")

	rows, err := db.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions ORDER BY created_at DESC`)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("获取卖家荷兰钟拍卖列表失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("数据库查询失败: %v", err),
		})
		return
	}
	defer rows.Close()

	var auctions []DutchAuction
	for rows.Next() {
		var auction DutchAuction
		var startTime, endTime sql.NullTime
		err := rows.Scan(
			&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
			&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
			&auction.Quantity, &startTime, &endTime, &auction.Status,
			&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
		if err != nil {
			logger.Info("dutch_auction", fmt.Sprintf("获取卖家荷兰钟拍卖列表，处理数据失败: %v\n", err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("处理数据失败: %v", err),
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

	logger.Info("dutch_auction", fmt.Sprintf("获取卖家荷兰钟拍卖列表成功，共 %d 条记录\n", len(jsonAuctions)))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auctions": jsonAuctions,
	})
}

// 暂停荷兰钟拍卖（下架）
func PauseDutchAuction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	logger.Info("dutch_auction", "暂停荷兰钟拍卖请求\n")

	if r.Method != "POST" {
		logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖失败，不支持的请求方法: %s\n", r.Method))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不支持的请求方法",
		})
		return
	}

	// 解析请求数据
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖，解析JSON失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("请求数据解析失败: %v", err),
		})
		return
	}

	// 验证输入
	if data.AuctionID <= 0 {
		logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖失败，拍卖ID %d 无效\n", data.AuctionID))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖ID无效",
		})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖，事务开始失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("事务开始失败: %v", err),
		})
		return
	}

	// 检查拍卖是否存在
	var auction DutchAuction
	var startTime, endTime sql.NullTime
	err = tx.QueryRow(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions WHERE id = ?`, data.AuctionID).Scan(
		&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
		&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
		&auction.Quantity, &startTime, &endTime, &auction.Status,
		&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == sql.ErrNoRows {
			logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖失败，拍卖ID %d 不存在\n", data.AuctionID))
			tx.Rollback()
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "拍卖不存在",
			})
		} else {
			logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖，获取拍卖信息失败: %v\n", err))
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("数据库查询失败: %v", err),
			})
		}
		return
	}

	// 检查拍卖状态
	if auction.Status != "active" {
		logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖失败，拍卖ID %d 状态不是活跃状态\n", data.AuctionID))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "拍卖ID不是活跃状态",
		})
		return
	}

	// 更新拍卖状态为待开始
	_, err = tx.Exec("UPDATE dutch_auctions SET status = 'pending', start_time = NULL, end_time = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?", data.AuctionID)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖，更新拍卖状态失败: %v\n", err))
		tx.Rollback()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("更新拍卖状态失败: %v", err),
		})
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖，事务提交失败: %v\n", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("事务提交失败: %v", err),
		})
		return
	}

	logger.Info("dutch_auction", fmt.Sprintf("暂停荷兰钟拍卖成功，ID: %d，物品类型: %s，数量: %d\n", auction.ID, auction.ItemType, auction.Quantity))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "拍卖已成功暂停",
	})
}

// 更新荷兰钟拍卖价格（定时任务调用）
func UpdateDutchAuctionPrices(db *sql.DB) {
	logger.Info("dutch_auction", "开始更新荷兰钟拍卖价格\n")

	// 获取所有活跃的拍卖
	rows, err := db.Query(`
		SELECT id, item_type, initial_price, current_price, min_price, price_decrement, 
		decrement_interval, quantity, start_time, end_time, status, winner_id, created_at, updated_at 
		FROM dutch_auctions WHERE status = 'active'`)
	if err != nil {
		logger.Info("dutch_auction", fmt.Sprintf("更新荷兰钟拍卖价格，获取活跃拍卖失败: %v\n", err))
		fmt.Printf("获取活跃拍卖失败: %v\n", err)
		return
	}
	defer rows.Close()

	now := time.Now()
	updatedCount := 0

	for rows.Next() {
		var auction DutchAuction
		var startTime, endTime sql.NullTime
		err := rows.Scan(
			&auction.ID, &auction.ItemType, &auction.InitialPrice, &auction.CurrentPrice,
			&auction.MinPrice, &auction.PriceDecrement, &auction.DecrementInterval,
			&auction.Quantity, &startTime, &endTime, &auction.Status,
			&auction.WinnerID, &auction.CreatedAt, &auction.UpdatedAt)
		if err != nil {
			logger.Info("dutch_auction", fmt.Sprintf("更新荷兰钟拍卖价格，扫描拍卖数据失败: %v\n", err))
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
		if auction.EndTime != nil && now.After(*auction.EndTime) {
			// 更新拍卖状态为已完成
			_, err = db.Exec("UPDATE dutch_auctions SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id = ?", auction.ID)
			if err != nil {
				logger.Info("dutch_auction", fmt.Sprintf("更新荷兰钟拍卖价格，更新拍卖状态为已完成失败: %v\n", err))
				fmt.Printf("更新拍卖状态为已完成失败: %v\n", err)
			} else {
				logger.Info("dutch_auction", fmt.Sprintf("拍卖ID %d 已自动结束\n", auction.ID))
				updatedCount++
			}
			continue
		}

		// 计算应该减少的价格
		if auction.StartTime == nil {
			continue
		}
		elapsed := now.Sub(*auction.StartTime)
		intervals := int(elapsed.Seconds()) / auction.DecrementInterval
		newPrice := auction.InitialPrice - float64(intervals)*auction.PriceDecrement

		// 确保价格不低于最低价格
		if newPrice < auction.MinPrice {
			newPrice = auction.MinPrice
		}

		// 如果价格有变化，更新数据库
		if newPrice != auction.CurrentPrice {
			_, err = db.Exec("UPDATE dutch_auctions SET current_price = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newPrice, auction.ID)
			if err != nil {
				logger.Info("dutch_auction", fmt.Sprintf("更新荷兰钟拍卖价格，更新拍卖价格失败: %v\n", err))
				fmt.Printf("更新拍卖价格失败: %v\n", err)
			} else {
				logger.Info("dutch_auction", fmt.Sprintf("拍卖ID %d 价格已更新: %.2f -> %.2f\n", auction.ID, auction.CurrentPrice, newPrice))
				updatedCount++
			}
		}
	}

	logger.Info("dutch_auction", fmt.Sprintf("荷兰钟拍卖价格更新完成，共更新 %d 个拍卖\n", updatedCount))
}
