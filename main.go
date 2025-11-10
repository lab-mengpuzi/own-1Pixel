package main

import (
	"bytes"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"own-1Pixel/backend/go/cash"
	"own-1Pixel/backend/go/config"
	"own-1Pixel/backend/go/logger"
	"own-1Pixel/backend/go/market"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed frontend/*
var frontendFS embed.FS                                         // 静态资源二进制化
var _config = config.GetConfig()                                // 获取配置
var db *sql.DB                                                  // 数据库对象
var auctionWSManager *market.AuctionWSManager                   // 拍卖WebSocket管理器
var auctionPriceUpdateManager *market.AuctionPriceUpdateManager // 价格更新管理器

// 初始化数据库
func initDatabase() error {
	err := cash.InitDatabase(db, _config.DbPath)
	if err != nil {
		logger.Info("initDatabase", fmt.Sprintf("初始化现金数据库失败: %v\n", err))
		return err
	}

	// 初始化市场数据库
	err = market.InitMarketDatabase(db)
	if err != nil {
		logger.Info("initDatabase", fmt.Sprintf("初始化市场数据库失败: %v\n", err))
		return err
	}

	// 初始化荷兰钟拍卖数据库
	err = market.InitAuctionDatabase(db)
	if err != nil {
		logger.Info("initDatabase", fmt.Sprintf("初始化荷兰钟拍卖数据库失败: %v\n", err))
		return err
	}

	return nil
}

// 获取当前余额
func getBalance(w http.ResponseWriter, r *http.Request) {
	cash.GetBalance(db, w, r)
}

// 获取所有交易记录
func getTransactions(w http.ResponseWriter, r *http.Request) {
	cash.GetTransactions(db, w, r)
}

// 添加交易记录
func addTransaction(w http.ResponseWriter, r *http.Request) {
	cash.AddTransaction(db, w, r)
}

// 获取市场参数
func getMarketParams(w http.ResponseWriter, r *http.Request) {
	market.GetMarketParams(db, w, r)
}

// 保存市场参数
func saveMarketParams(w http.ResponseWriter, r *http.Request) {
	market.SaveMarketParams(db, w, r)
}

// 获取背包状态
func getBackpack(w http.ResponseWriter, r *http.Request) {
	market.GetBackpack(db, w, r)
}

// 获取市场物品
func getMarketItems(w http.ResponseWriter, r *http.Request) {
	market.GetMarketItems(db, w, r)
}

// 制作苹果
func makeApple(w http.ResponseWriter, r *http.Request) {
	market.MakeItem(db, w, r, "apple")
}

// 制作木材
func makeWood(w http.ResponseWriter, r *http.Request) {
	market.MakeItem(db, w, r, "wood")
}

// 卖出苹果
func sellApple(w http.ResponseWriter, r *http.Request) {
	market.SellItem(db, w, r, "apple")
}

// 卖出木材
func sellWood(w http.ResponseWriter, r *http.Request) {
	market.SellItem(db, w, r, "wood")
}

// 买入苹果
func buyApple(w http.ResponseWriter, r *http.Request) {
	market.BuyItem(db, w, r, "apple")
}

// 买入木材
func buyWood(w http.ResponseWriter, r *http.Request) {
	market.BuyItem(db, w, r, "wood")
}

// 创建荷兰钟拍卖
func createAuction(w http.ResponseWriter, r *http.Request) {
	market.CreateAuction(db, w, r)

	// 通过WebSocket广播拍卖列表更新
	if auctionWSManager != nil {
		auctions, err := market.GetActiveAuctions(db)
		if err == nil && len(auctions) > 0 {
			// 广播最新创建的拍卖
			auctionWSManager.BroadcastAuctionWSUpdate(&auctions[0], "created")
		}
	}
}

// 获取所有荷兰钟拍卖
func getAuctions(w http.ResponseWriter, r *http.Request) {
	market.GetAuctions(db, w, r)
}

// 获取单个荷兰钟拍卖
func getAuction(w http.ResponseWriter, r *http.Request) {
	market.GetAuction(db, w, r)
}

// 开始荷兰钟拍卖
func startAuction(w http.ResponseWriter, r *http.Request) {
	// 先从请求中获取拍卖ID
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	var auctionID int

	// 尝试解析请求体获取拍卖ID
	body, err := io.ReadAll(r.Body)
	if err == nil {
		if err := json.Unmarshal(body, &data); err == nil {
			auctionID = data.AuctionID
		}
	}

	// 重置请求体，以便market.StartAuction可以读取
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// 调用market.StartAuction
	market.StartAuction(db, w, r)

	// 通过WebSocket广播拍卖更新
	if auctionWSManager != nil && auctionID > 0 {
		auction, err := market.GetAuctionID(db, auctionID)
		if err == nil {
			auctionWSManager.BroadcastAuctionWSUpdate(auction, "started")

			// 启动价格更新管理器
			if auctionPriceUpdateManager != nil && !auctionPriceUpdateManager.IsRunning() {
				auctionPriceUpdateManager.StartAuctionWSPriceUpdateManager()
			}
		}
	}
}

// 提交荷兰钟竞价
func CommitAuctionBid(w http.ResponseWriter, r *http.Request) {
	// 先从请求中获取拍卖ID
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	var auctionID int

	// 尝试解析请求体获取拍卖ID
	body, err := io.ReadAll(r.Body)
	if err == nil {
		if err := json.Unmarshal(body, &data); err == nil {
			auctionID = data.AuctionID
		}
	}

	// 重置请求体，以便market.CommitAuctionBid可以读取
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// 调用market.CommitAuctionBid
	market.CommitAuctionBid(db, w, r)

	// 通过WebSocket广播拍卖更新
	if auctionWSManager != nil && auctionID > 0 {
		auction, err := market.GetAuctionID(db, auctionID)
		if err == nil {
			auctionWSManager.BroadcastAuctionWSUpdate(auction, "bid_placed")
		}
	}
}

// 取消荷兰钟拍卖
func cancelAuction(w http.ResponseWriter, r *http.Request) {
	// 先从请求中获取拍卖ID
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	var auctionID int

	// 尝试解析请求体获取拍卖ID
	body, err := io.ReadAll(r.Body)
	if err == nil {
		if err := json.Unmarshal(body, &data); err == nil {
			auctionID = data.AuctionID
		}
	}

	// 重置请求体，以便market.CancelAuction可以读取
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// 调用market.CancelAuction
	market.CancelAuction(db, w, r)

	// 通过WebSocket广播拍卖更新
	if auctionWSManager != nil && auctionID > 0 {
		auction, err := market.GetAuctionID(db, auctionID)
		if err == nil {
			auctionWSManager.BroadcastAuctionWSUpdate(auction, "cancelled")
		}
	}
}

// 暂停荷兰钟拍卖
func pauseAuction(w http.ResponseWriter, r *http.Request) {
	// 先从请求中获取拍卖ID
	var data struct {
		AuctionID int `json:"auction_id"`
	}
	var auctionID int

	// 尝试解析请求体获取拍卖ID
	body, err := io.ReadAll(r.Body)
	if err == nil {
		if err := json.Unmarshal(body, &data); err == nil {
			auctionID = data.AuctionID
		}
	}

	// 重置请求体，以便market.PauseAuction可以读取
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// 调用market.PauseAuction
	market.PauseAuction(db, w, r)

	// 通过WebSocket广播拍卖更新
	if auctionWSManager != nil && auctionID > 0 {
		auction, err := market.GetAuctionID(db, auctionID)
		if err == nil {
			auctionWSManager.BroadcastAuctionWSUpdate(auction, "paused")
		}
	}
}

// 获取卖家荷兰钟拍卖列表
func getSellerAuctions(w http.ResponseWriter, r *http.Request) {
	market.GetSellerAuctions(db, w, r)
}

func main() {
	var err error

	// 初始化日志记录器
	logger.Init("")

	// 打开数据库连接
	// 添加SQLite特定参数以提高并发性能
	dbPathWithParams := fmt.Sprintf("%s?cache=shared&mode=rwc&_journal_mode=WAL&_synchronous=NORMAL&_timeout=5000", _config.DbPath)
	db, err = sql.Open("sqlite", dbPathWithParams)
	if err != nil {
		logger.Info("main", fmt.Sprintf("打开数据库失败: %v\n", err))
		fmt.Printf("打开数据库失败: %v", err)
		return
	}

	// 设置连接池参数，提高并发性能
	db.SetMaxOpenConns(25)                 // 设置最大打开连接数
	db.SetMaxIdleConns(10)                 // 设置最大空闲连接数
	db.SetConnMaxLifetime(5 * time.Minute) // 设置连接最大生存时间

	// 初始化数据库
	err = initDatabase()
	if err != nil {
		logger.Info("main", fmt.Sprintf("初始化数据库失败: %v\n", err))
		fmt.Printf("初始化数据库失败: %v", err)
		return
	}
	defer db.Close()

	// 初始化WebSocket管理器
	auctionWSManager = market.InitAuctionWSManager(db)

	// 初始化价格更新管理器
	auctionPriceUpdateManager = market.InitAuctionWSPriceUpdateManager(db, auctionWSManager)

	// 处理静态资源二进制化
	staticFS, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		logger.Info("main", fmt.Sprintf("处理静态资源二进制化错误: %v\n", err))
		fmt.Printf("处理静态资源二进制化错误: %v\n", err)
		return
	}

	// 处理根路径请求，重定向到 html/index.html
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 如果请求的是根路径，则重定向到 html/index.html
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/html/index.html", http.StatusFound)
			return
		}
		// 其他路径由静态文件服务器处理
		http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
	})

	// api:cash: 交易记录
	http.HandleFunc("/api/cash/balance", getBalance)
	http.HandleFunc("/api/cash/transactions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			getTransactions(w, r)
		case "POST":
			addTransaction(w, r)
		default:
			http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		}
	})

	// 市场相关路由
	http.HandleFunc("/api/market/balance", getBalance)
	http.HandleFunc("/api/market/params", getMarketParams)
	http.HandleFunc("/api/market/save-params", saveMarketParams)
	http.HandleFunc("/api/market/backpack", getBackpack)
	http.HandleFunc("/api/market/items", getMarketItems)
	http.HandleFunc("/api/market/make-apple", makeApple)
	http.HandleFunc("/api/market/make-wood", makeWood)
	http.HandleFunc("/api/market/sell-apple", sellApple)
	http.HandleFunc("/api/market/sell-wood", sellWood)
	http.HandleFunc("/api/market/buy-apple", buyApple)
	http.HandleFunc("/api/market/buy-wood", buyWood)

	// 荷兰钟拍卖相关路由
	http.HandleFunc("/api/auction/create", createAuction)
	http.HandleFunc("/api/auction/list", getAuctions)
	http.HandleFunc("/api/auction/seller-list", getSellerAuctions)
	http.HandleFunc("/api/auction/get", getAuction)
	http.HandleFunc("/api/auction/start", startAuction)
	http.HandleFunc("/api/auction/bid", CommitAuctionBid)
	http.HandleFunc("/api/auction/cancel", cancelAuction)
	http.HandleFunc("/api/auction/pause", pauseAuction)

	// WebSocket端点
	http.HandleFunc("/ws/auction", auctionWSManager.HandleAuctionWebSocket)

	// 记录服务器启动日志
	logger.Info("main", fmt.Sprintf("own-1Pixel 启动服务器 %d\n", _config.Port))
	logger.Info("main", fmt.Sprintf("访问 http://%s:%d 或 http://localhost:%d\n", _config.Host, _config.Port, _config.Port))

	// 启动服务器
	fmt.Printf("own-1Pixel 启动服务器 %d\n", _config.Port)
	fmt.Printf("访问 http://%s:%d 或 http://localhost:%d\n", _config.Host, _config.Port, _config.Port)

	err = http.ListenAndServe(fmt.Sprintf("%s:%d", _config.Host, _config.Port), nil)
	if err != nil {
		logger.Info("main", fmt.Sprintf("启动服务器错误: %v\n", err))
		fmt.Printf("启动服务器错误: %v\n", err)
	}

	// 关闭日志系统
	logger.Close()
}
