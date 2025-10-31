package main

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"own-1Pixel/backend/go/cash"
	"own-1Pixel/backend/go/config"
	"own-1Pixel/backend/go/logger"
	"own-1Pixel/backend/go/market"

	_ "modernc.org/sqlite"
)

//go:embed frontend/*
var frontendFS embed.FS          // 静态资源二进制化
var _config = config.GetConfig() // 获取配置
var db *sql.DB                   // 数据库对象

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
	err = market.InitDutchAuctionDatabase(db)
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
func createDutchAuction(w http.ResponseWriter, r *http.Request) {
	market.CreateDutchAuction(db, w, r)
}

// 获取所有荷兰钟拍卖
func getDutchAuctions(w http.ResponseWriter, r *http.Request) {
	market.GetDutchAuctions(db, w, r)
}

// 获取单个荷兰钟拍卖
func getDutchAuction(w http.ResponseWriter, r *http.Request) {
	market.GetDutchAuction(db, w, r)
}

// 开始荷兰钟拍卖
func startDutchAuction(w http.ResponseWriter, r *http.Request) {
	market.StartDutchAuction(db, w, r)
}

// 提交荷兰钟竞价
func placeDutchBid(w http.ResponseWriter, r *http.Request) {
	market.PlaceDutchBid(db, w, r)
}

// 取消荷兰钟拍卖
func cancelDutchAuction(w http.ResponseWriter, r *http.Request) {
	market.CancelDutchAuction(db, w, r)
}

// 暂停荷兰钟拍卖（下架）
func pauseDutchAuction(w http.ResponseWriter, r *http.Request) {
	market.PauseDutchAuction(db, w, r)
}

// 获取卖家荷兰钟拍卖列表
func getSellerDutchAuctions(w http.ResponseWriter, r *http.Request) {
	market.GetSellerDutchAuctions(db, w, r)
}

func main() {
	var err error

	// 初始化日志记录器
	logger.Init("")

	// 打开数据库连接
	db, err = sql.Open("sqlite", _config.DbPath)
	if err != nil {
		logger.Info("main", fmt.Sprintf("打开数据库失败: %v\n", err))
		fmt.Printf("打开数据库失败: %v", err)
		return
	}

	// 初始化数据库
	err = initDatabase()
	if err != nil {
		logger.Info("main", fmt.Sprintf("初始化数据库失败: %v\n", err))
		fmt.Printf("初始化数据库失败: %v", err)
		return
	}
	defer db.Close()

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
	http.HandleFunc("/api/dutch-auction/create", createDutchAuction)
	http.HandleFunc("/api/dutch-auction/list", getDutchAuctions)
	http.HandleFunc("/api/dutch-auction/seller-list", getSellerDutchAuctions)
	http.HandleFunc("/api/dutch-auction/get", getDutchAuction)
	http.HandleFunc("/api/dutch-auction/start", startDutchAuction)
	http.HandleFunc("/api/dutch-auction/bid", placeDutchBid)
	http.HandleFunc("/api/dutch-auction/cancel", cancelDutchAuction)
	http.HandleFunc("/api/dutch-auction/pause", pauseDutchAuction)

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
