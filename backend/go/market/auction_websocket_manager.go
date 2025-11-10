package market

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"own-1Pixel/backend/go/logger"

	"github.com/gorilla/websocket"
)

// WebSocket连接管理器
type AuctionWSManager struct {
	connections map[*websocket.Conn]bool
	mu          sync.Mutex
	db          *sql.DB
}

// WebSocket消息结构
type AuctionWSMessage struct {
	Type      string      `json:"type"`      // 消息类型: auction_update, auction_price_update, bid_result等
	Data      interface{} `json:"data"`      // 消息数据
	Timestamp time.Time   `json:"timestamp"` // 时间戳
}

// 荷兰钟拍卖更新消息
type AuctionWSUpdateMessage struct {
	Auction *Auction `json:"auction"`
	Action  string   `json:"action"` // created, updated, started, completed, cancelled等
}

// 价格更新消息
type AuctionPriceUpdateMessage struct {
	AuctionID     int     `json:"auctionId"`
	OldPrice      float64 `json:"oldPrice"`
	NewPrice      float64 `json:"newPrice"`
	TimeRemaining int     `json:"timeRemaining"` // 剩余时间（秒）
}

// 竞价结果消息
type AuctionWSBidResultMessage struct {
	AuctionID int     `json:"auctionId"`
	UserID    int     `json:"userId"`
	Success   bool    `json:"success"`
	Message   string  `json:"message"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

// 创建新的WebSocket管理器
func InitAuctionWSManager(db *sql.DB) *AuctionWSManager {
	return &AuctionWSManager{
		connections: make(map[*websocket.Conn]bool),
		db:          db,
	}
}

// WebSocket升级器
var auctionWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应该更严格
	},
}

// 处理WebSocket连接
func (auctionWSManager *AuctionWSManager) HandleAuctionWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级HTTP连接到WebSocket
	conn, err := auctionWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Info("websocket", fmt.Sprintf("WebSocket升级失败: %v\n", err))
		return
	}

	// 设置连接参数
	conn.SetReadLimit(512)                                 // 限制读取消息大小
	conn.SetReadDeadline(time.Now().Add(60 * time.Second)) // 设置读取超时
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 添加连接到管理器
	auctionWSManager.mu.Lock()
	auctionWSManager.connections[conn] = true
	connectionCount := len(auctionWSManager.connections)
	auctionWSManager.mu.Unlock()

	logger.Info("websocket", fmt.Sprintf("新的WebSocket连接已建立，当前连接数: %d\n", connectionCount))

	// 发送当前活跃拍卖列表
	auctionWSManager.sendActiveAuctions(conn)

	// 启动心跳检测
	go auctionWSManager.auctionHeartbeatLoop(conn)

	// 处理消息
	for {
		var msg AuctionWSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			// 检查错误类型
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Info("websocket", fmt.Sprintf("WebSocket意外关闭: %v\n", err))
			} else if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				logger.Info("websocket", "WebSocket正常关闭\n")
			} else {
				logger.Info("websocket", fmt.Sprintf("读取WebSocket消息失败: %v\n", err))
			}
			break
		}

		// 处理客户端消息
		auctionWSManager.handleAuctionClientMessage(conn, msg)
	}

	// 连接关闭时清理
	auctionWSManager.mu.Lock()
	delete(auctionWSManager.connections, conn)
	connectionCount = len(auctionWSManager.connections)
	auctionWSManager.mu.Unlock()

	logger.Info("websocket", fmt.Sprintf("WebSocket连接已关闭，当前连接数: %d\n", connectionCount))
}

// 心跳检测循环
func (auctionWSManager *AuctionWSManager) auctionHeartbeatLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second) // 每30秒发送一次心跳
	defer ticker.Stop()

	for range ticker.C {
		// 发送ping
		err := conn.WriteMessage(websocket.PingMessage, nil)
		if err != nil {
			logger.Info("websocket", fmt.Sprintf("发送ping失败: %v\n", err))
			return
		}
	}
}

// 处理客户端消息
func (auctionWSManager *AuctionWSManager) handleAuctionClientMessage(conn *websocket.Conn, msg AuctionWSMessage) {
	switch msg.Type {
	case "get_auction":
		// 获取特定拍卖详情
		if auctionID, ok := msg.Data.(float64); ok {
			auctionWSManager.sendAuctionDetails(conn, int(auctionID))
		}
	case "place_bid":
		// 处理竞价请求
		auctionWSManager.handleAuctionBidRequest(conn, msg.Data)
	case "get_auctions":
		// 获取拍卖列表
		auctionWSManager.sendActiveAuctions(conn)
	}
}

// 发送活跃拍卖列表
func (auctionWSManager *AuctionWSManager) sendActiveAuctions(conn *websocket.Conn) {
	auctions, err := GetActiveAuctions(auctionWSManager.db)
	if err != nil {
		logger.Info("websocket", fmt.Sprintf("获取活跃拍卖失败: %v\n", err))
		return
	}

	msg := AuctionWSMessage{
		Type:      "auction_list",
		Data:      auctions,
		Timestamp: time.Now(),
	}

	err = conn.WriteJSON(msg)
	if err != nil {
		logger.Info("websocket", fmt.Sprintf("发送拍卖列表失败: %v\n", err))
	}
}

// 发送特定拍卖详情
func (auctionWSManager *AuctionWSManager) sendAuctionDetails(conn *websocket.Conn, auctionID int) {
	auction, err := GetAuctionID(auctionWSManager.db, auctionID)
	if err != nil {
		logger.Info("websocket", fmt.Sprintf("获取拍卖详情失败: %v\n", err))
		return
	}

	msg := AuctionWSMessage{
		Type:      "auction_details",
		Data:      auction,
		Timestamp: time.Now(),
	}

	err = conn.WriteJSON(msg)
	if err != nil {
		logger.Info("websocket", fmt.Sprintf("发送拍卖详情失败: %v\n", err))
	}
}

// 处理竞价请求
func (auctionWSManager *AuctionWSManager) handleAuctionBidRequest(conn *websocket.Conn, data interface{}) {
	// 解析竞价数据
	bidData, ok := data.(map[string]interface{})
	if !ok {
		auctionWSManager.sendAuctionWSBidResult(conn, 0, false, "无效的竞价数据", 0, 0)
		return
	}

	auctionID, ok1 := bidData["auctionId"].(float64)
	userID, ok2 := bidData["userId"].(float64)
	price, ok3 := bidData["price"].(float64)
	quantity, ok4 := bidData["quantity"].(float64)

	if !ok1 || !ok2 || !ok3 || !ok4 {
		auctionWSManager.sendAuctionWSBidResult(conn, 0, false, "竞价数据格式错误", 0, 0)
		return
	}

	// 处理竞价
	success, message, err := ProcessAuctionBid(auctionWSManager.db, int(auctionID), int(userID), price, int(quantity))
	if err != nil {
		logger.Info("websocket", fmt.Sprintf("处理竞价失败: %v\n", err))
		auctionWSManager.sendAuctionWSBidResult(conn, int(userID), false, "竞价处理失败", 0, 0)
		return
	}

	// 发送竞价结果
	auctionWSManager.sendAuctionWSBidResult(conn, int(userID), success, message, price, int(quantity))

	// 如果竞价成功，广播拍卖更新
	if success {
		auction, err := GetAuctionID(auctionWSManager.db, int(auctionID))
		if err == nil {
			auctionWSManager.BroadcastAuctionWSUpdate(auction, "bid_placed")
		}
	}
}

// 发送竞价结果
func (auctionWSManager *AuctionWSManager) sendAuctionWSBidResult(conn *websocket.Conn, userID int, success bool, message string, price float64, quantity int) {
	result := AuctionWSBidResultMessage{
		UserID:   userID,
		Success:  success,
		Message:  message,
		Price:    price,
		Quantity: quantity,
	}

	msg := AuctionWSMessage{
		Type:      "bid_result",
		Data:      result,
		Timestamp: time.Now(),
	}

	err := conn.WriteJSON(msg)
	if err != nil {
		logger.Info("websocket", fmt.Sprintf("发送竞价结果失败: %v\n", err))
	}
}

// 广播拍卖更新
func (auctionWSManager *AuctionWSManager) BroadcastAuctionWSUpdate(auction *Auction, action string) {
	update := AuctionWSUpdateMessage{
		Auction: auction,
		Action:  action,
	}

	msg := AuctionWSMessage{
		Type:      "auction_update",
		Data:      update,
		Timestamp: time.Now(),
	}

	auctionWSManager.mu.Lock()
	defer auctionWSManager.mu.Unlock()

	// 创建临时连接列表，避免在迭代过程中修改原map
	connections := make([]*websocket.Conn, 0, len(auctionWSManager.connections))
	for conn := range auctionWSManager.connections {
		connections = append(connections, conn)
	}

	var successCount int
	var failedConnections []*websocket.Conn

	for _, conn := range connections {
		// 检查连接是否还在管理器中
		if _, exists := auctionWSManager.connections[conn]; !exists {
			continue
		}

		// 设置写入超时
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := conn.WriteJSON(msg)
		if err != nil {
			logger.Info("websocket", fmt.Sprintf("广播拍卖更新失败: %v\n", err))
			failedConnections = append(failedConnections, conn)
		} else {
			successCount++
		}
	}

	// 移除失败的连接
	for _, conn := range failedConnections {
		conn.Close()
		delete(auctionWSManager.connections, conn)
	}

	logger.Info("websocket", fmt.Sprintf("广播拍卖更新完成: 成功 %d, 失败 %d\n", successCount, len(failedConnections)))
}

// 广播价格更新
func (auctionWSManager *AuctionWSManager) BroadcastAuctionWSPriceUpdate(auctionID int, oldPrice, newPrice float64, timeRemaining int) {
	update := AuctionPriceUpdateMessage{
		AuctionID:     auctionID,
		OldPrice:      oldPrice,
		NewPrice:      newPrice,
		TimeRemaining: timeRemaining,
	}

	msg := AuctionWSMessage{
		Type:      "auction_price_update",
		Data:      update,
		Timestamp: time.Now(),
	}

	auctionWSManager.mu.Lock()
	defer auctionWSManager.mu.Unlock()

	// 创建临时连接列表，避免在迭代过程中修改原map
	connections := make([]*websocket.Conn, 0, len(auctionWSManager.connections))
	for conn := range auctionWSManager.connections {
		connections = append(connections, conn)
	}

	var successCount int
	var failedConnections []*websocket.Conn

	for _, conn := range connections {
		// 检查连接是否还在管理器中
		if _, exists := auctionWSManager.connections[conn]; !exists {
			continue
		}

		// 设置写入超时
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := conn.WriteJSON(msg)
		if err != nil {
			logger.Info("websocket", fmt.Sprintf("广播价格更新失败: %v\n", err))
			failedConnections = append(failedConnections, conn)
		} else {
			successCount++
		}
	}

	// 移除失败的连接
	for _, conn := range failedConnections {
		conn.Close()
		delete(auctionWSManager.connections, conn)
	}

	logger.Info("websocket", fmt.Sprintf("广播价格更新完成: 成功 %d, 失败 %d\n", successCount, len(failedConnections)))
}

// 获取连接数
func (auctionWSManager *AuctionWSManager) GetAuctionWSConnectionCount() int {
	auctionWSManager.mu.Lock()
	defer auctionWSManager.mu.Unlock()
	return len(auctionWSManager.connections)
}
