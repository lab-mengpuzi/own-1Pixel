package market

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	_ "modernc.org/sqlite"
)

// 市场参数结构
type MarketParams struct {
	ID               int       `json:"id"`
	BalanceRange     float64   `json:"balanceRange"`     // 平衡区间系数
	PriceFluctuation float64   `json:"priceFluctuation"` // 价格波动系数
	MaxPriceChange   float64   `json:"maxPriceChange"`   // 最大价格变动系数
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// 背包结构
type Backpack struct {
	ID        int       `json:"id"`
	Apple     int       `json:"apple"`
	Wood      int       `json:"wood"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 市场物品结构
type MarketItem struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Stock     int       `json:"stock"`
	BasePrice float64   `json:"basePrice"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// 市场物品集合
type MarketItems struct {
	Apple MarketItem `json:"apple"`
	Wood  MarketItem `json:"wood"`
}

// 初始化市场数据库
func InitMarketDatabase(db *sql.DB) error {
	// 创建市场参数表
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS market_params (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			balance_range REAL NOT NULL DEFAULT 1.0,
			price_fluctuation REAL NOT NULL DEFAULT 1.0,
			max_price_change REAL NOT NULL DEFAULT 1.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 创建背包表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS backpack (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			apple INTEGER NOT NULL DEFAULT 0,
			wood INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 创建市场物品表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS market_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			price REAL NOT NULL,
			stock INTEGER NOT NULL DEFAULT 0,
			base_price REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 检查是否有市场参数记录，如果没有则初始化
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM market_params").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		_, err = db.Exec("INSERT INTO market_params (balance_range, price_fluctuation, max_price_change) VALUES (1.0, 1.0, 1.0)")
		if err != nil {
			return err
		}
	}

	// 检查是否有背包记录，如果没有则初始化
	err = db.QueryRow("SELECT COUNT(*) FROM backpack").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		_, err = db.Exec("INSERT INTO backpack (apple, wood) VALUES (0, 0)")
		if err != nil {
			return err
		}
	}

	// 检查是否有市场物品记录，如果没有则初始化
	err = db.QueryRow("SELECT COUNT(*) FROM market_items WHERE name IN ('apple', 'wood')").Scan(&count)
	if err != nil {
		return err
	}

	if count < 2 {
		// 检查是否有苹果记录
		var appleCount int
		db.QueryRow("SELECT COUNT(*) FROM market_items WHERE name = 'apple'").Scan(&appleCount)
		if appleCount == 0 {
			_, err = db.Exec("INSERT INTO market_items (name, price, stock, base_price) VALUES ('apple', 1.0, 0, 1.0)")
			if err != nil {
				return err
			}
		}

		// 检查是否有木材记录
		var woodCount int
		db.QueryRow("SELECT COUNT(*) FROM market_items WHERE name = 'wood'").Scan(&woodCount)
		if woodCount == 0 {
			_, err = db.Exec("INSERT INTO market_items (name, price, stock, base_price) VALUES ('wood', 5.0, 0, 5.0)")
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// 获取市场参数
func GetMarketParams(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var params MarketParams
	err := db.QueryRow("SELECT id, balance_range, price_fluctuation, max_price_change, created_at, updated_at FROM market_params ORDER BY id DESC LIMIT 1").Scan(
		&params.ID, &params.BalanceRange, &params.PriceFluctuation, &params.MaxPriceChange, &params.CreatedAt, &params.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(params)
}

// 更新市场参数
func UpdateMarketParams(db *sql.DB, params MarketParams) error {
	_, err := db.Exec("UPDATE market_params SET balance_range = ?, price_fluctuation = ?, max_price_change = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		params.BalanceRange, params.PriceFluctuation, params.MaxPriceChange, params.ID)
	return err
}

// 保存市场参数
func SaveMarketParams(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var params MarketParams
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 获取当前参数ID
	var currentID int
	err = db.QueryRow("SELECT id FROM market_params ORDER BY id DESC LIMIT 1").Scan(&currentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	params.ID = currentID

	// 更新参数
	err = UpdateMarketParams(db, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(params)
}

// 获取背包状态
func GetBackpack(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var backpack Backpack
	err := db.QueryRow("SELECT id, apple, wood, created_at, updated_at FROM backpack ORDER BY id DESC LIMIT 1").Scan(
		&backpack.ID, &backpack.Apple, &backpack.Wood, &backpack.CreatedAt, &backpack.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(backpack)
}

// 更新背包
func UpdateBackpack(db *sql.DB, backpack Backpack) error {
	_, err := db.Exec("UPDATE backpack SET apple = ?, wood = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		backpack.Apple, &backpack.Wood, backpack.ID)
	return err
}

// 获取市场物品
func GetMarketItems(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// 获取苹果
	var apple MarketItem
	err := db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'apple'").Scan(
		&apple.ID, &apple.Name, &apple.Price, &apple.Stock, &apple.BasePrice, &apple.CreatedAt, &apple.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取木材
	var wood MarketItem
	err = db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'wood'").Scan(
		&wood.ID, &wood.Name, &wood.Price, &wood.Stock, &wood.BasePrice, &wood.CreatedAt, &wood.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := MarketItems{
		Apple: apple,
		Wood:  wood,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// 更新市场物品价格和库存
func UpdateMarketItem(db *sql.DB, item MarketItem) error {
	_, err := db.Exec("UPDATE market_items SET price = ?, stock = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		item.Price, item.Stock, item.ID)
	return err
}

// 计算新价格
func CalculateNewPrice(currentPrice float64, stock int, params MarketParams, basePrice float64) float64 {
	// 计算平衡区间
	balanceRange := params.BalanceRange * 5 // 假设5个物品为平衡点

	// 计算价格变动
	var priceChange float64
	if stock > int(balanceRange) {
		// 供过于求，价格下降
		excess := float64(stock - int(balanceRange))
		priceChange = -excess * params.PriceFluctuation * 0.1
	} else if stock < int(balanceRange) {
		// 供不应求，价格上涨
		shortage := float64(int(balanceRange) - stock)
		priceChange = shortage * params.PriceFluctuation * 0.1
	} else {
		// 供需平衡，价格不变
		return currentPrice
	}

	// 限制价格变动幅度
	if priceChange > params.MaxPriceChange {
		priceChange = params.MaxPriceChange
	} else if priceChange < -params.MaxPriceChange {
		priceChange = -params.MaxPriceChange
	}

	// 计算新价格
	newPrice := currentPrice + priceChange

	// 确保价格不低于基础价格的50%，不高于基础价格的200%
	minPrice := basePrice * 0.5
	maxPrice := basePrice * 2.0

	if newPrice < minPrice {
		newPrice = minPrice
	} else if newPrice > maxPrice {
		newPrice = maxPrice
	}

	return newPrice
}

// 制作物品
func MakeItem(db *sql.DB, w http.ResponseWriter, r *http.Request, itemType string) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取当前背包
	var backpack Backpack
	err := db.QueryRow("SELECT id, apple, wood, created_at, updated_at FROM backpack ORDER BY id DESC LIMIT 1").Scan(
		&backpack.ID, &backpack.Apple, &backpack.Wood, &backpack.CreatedAt, &backpack.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 根据物品类型更新背包
	switch itemType {
	case "apple":
		backpack.Apple++
	case "wood":
		backpack.Wood++
	default:
		http.Error(w, "Invalid item type", http.StatusBadRequest)
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新背包
	_, err = tx.Exec("UPDATE backpack SET apple = ?, wood = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		backpack.Apple, backpack.Wood, backpack.ID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加交易记录，收入和支出都为0，备注为制作苹果或制作木材
	note := ""
	switch itemType {
	case "apple":
		note = "制作苹果"
	case "wood":
		note = "制作木材"
	}

	// 隐私数据
	_, err = tx.Exec(
		"INSERT INTO transactions (transaction_time, our_bank_account_name, counterparty_alias, our_bank_name, counterparty_bank, expense_amount, income_amount, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		time.Now(), "玩家", "系统", "玩家银行", "系统银行", 0, 0, note)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"backpack": backpack,
	})
}

// 卖出物品
func SellItem(db *sql.DB, w http.ResponseWriter, r *http.Request, itemType string) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取当前背包
	var backpack Backpack
	err := db.QueryRow("SELECT id, apple, wood, created_at, updated_at FROM backpack ORDER BY id DESC LIMIT 1").Scan(
		&backpack.ID, &backpack.Apple, &backpack.Wood, &backpack.CreatedAt, &backpack.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 检查背包中是否有足够的物品
	if itemType == "apple" && backpack.Apple <= 0 {
		http.Error(w, "No apples in backpack", http.StatusBadRequest)
		return
	} else if itemType == "wood" && backpack.Wood <= 0 {
		http.Error(w, "No wood in backpack", http.StatusBadRequest)
		return
	}

	// 获取当前市场物品
	var item MarketItem
	switch itemType {
	case "apple":
		err = db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'apple'").Scan(
			&item.ID, &item.Name, &item.Price, &item.Stock, &item.BasePrice, &item.CreatedAt, &item.UpdatedAt)
	case "wood":
		err = db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'wood'").Scan(
			&item.ID, &item.Name, &item.Price, &item.Stock, &item.BasePrice, &item.CreatedAt, &item.UpdatedAt)
	default:
		http.Error(w, "Invalid item type", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取当前余额
	var balance struct {
		ID        int       `json:"id"`
		Amount    float64   `json:"amount"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	err = db.QueryRow("SELECT id, amount, updated_at FROM balance ORDER BY id DESC LIMIT 1").Scan(&balance.ID, &balance.Amount, &balance.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取市场参数
	var params MarketParams
	err = db.QueryRow("SELECT id, balance_range, price_fluctuation, max_price_change, created_at, updated_at FROM market_params ORDER BY id DESC LIMIT 1").Scan(
		&params.ID, &params.BalanceRange, &params.PriceFluctuation, &params.MaxPriceChange, &params.CreatedAt, &params.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新背包
	switch itemType {
	case "apple":
		backpack.Apple--
	case "wood":
		backpack.Wood--
	default:
		http.Error(w, "Invalid item type", http.StatusBadRequest)
		return
	}

	// 更新市场物品库存
	item.Stock++

	// 计算新价格
	item.Price = CalculateNewPrice(item.Price, item.Stock, params, item.BasePrice)

	// 更新余额
	newBalance := balance.Amount + item.Price

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新背包
	_, err = tx.Exec("UPDATE backpack SET apple = ?, wood = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		backpack.Apple, backpack.Wood, backpack.ID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新市场物品
	_, err = tx.Exec("UPDATE market_items SET price = ?, stock = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		item.Price, item.Stock, item.ID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新余额
	_, err = tx.Exec("UPDATE balance SET amount = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		newBalance, balance.ID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加交易记录
	// 隐私数据
	_, err = tx.Exec(
		"INSERT INTO transactions (transaction_time, our_bank_account_name, counterparty_alias, our_bank_name, counterparty_bank, expense_amount, income_amount, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		time.Now(), "萌铺子市场", "玩家", "萌铺子市场银行", "玩家银行", 0, item.Price, fmt.Sprintf("卖出%s", itemType))
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取更新后的市场物品
	var apple MarketItem
	err = db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'apple'").Scan(
		&apple.ID, &apple.Name, &apple.Price, &apple.Stock, &apple.BasePrice, &apple.CreatedAt, &apple.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wood MarketItem
	err = db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'wood'").Scan(
		&wood.ID, &wood.Name, &wood.Price, &wood.Stock, &wood.BasePrice, &wood.CreatedAt, &wood.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := MarketItems{
		Apple: apple,
		Wood:  wood,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"backpack":    backpack,
		"marketItems": items,
	})
}

// 买入物品
func BuyItem(db *sql.DB, w http.ResponseWriter, r *http.Request, itemType string) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取当前市场物品
	var item MarketItem
	switch itemType {
	case "apple":
		err := db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'apple'").Scan(
			&item.ID, &item.Name, &item.Price, &item.Stock, &item.BasePrice, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "wood":
		err := db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'wood'").Scan(
			&item.ID, &item.Name, &item.Price, &item.Stock, &item.BasePrice, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Invalid item type", http.StatusBadRequest)
		return
	}

	// 检查市场物品库存
	if item.Stock <= 0 {
		http.Error(w, fmt.Sprintf("No %s in stock", itemType), http.StatusBadRequest)
		return
	}

	// 获取当前余额
	var balance struct {
		ID        int       `json:"id"`
		Amount    float64   `json:"amount"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	err := db.QueryRow("SELECT id, amount, updated_at FROM balance ORDER BY id DESC LIMIT 1").Scan(&balance.ID, &balance.Amount, &balance.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 检查余额是否足够
	if balance.Amount < item.Price {
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// 获取当前背包
	var backpack Backpack
	err = db.QueryRow("SELECT id, apple, wood, created_at, updated_at FROM backpack ORDER BY id DESC LIMIT 1").Scan(
		&backpack.ID, &backpack.Apple, &backpack.Wood, &backpack.CreatedAt, &backpack.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取市场参数
	var params MarketParams
	err = db.QueryRow("SELECT id, balance_range, price_fluctuation, max_price_change, created_at, updated_at FROM market_params ORDER BY id DESC LIMIT 1").Scan(
		&params.ID, &params.BalanceRange, &params.PriceFluctuation, &params.MaxPriceChange, &params.CreatedAt, &params.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新背包
	switch itemType {
	case "apple":
		backpack.Apple++
	case "wood":
		backpack.Wood++
	default:
		http.Error(w, "Invalid item type", http.StatusBadRequest)
		return
	}

	// 更新市场物品库存
	item.Stock--

	// 计算新价格
	item.Price = CalculateNewPrice(item.Price, item.Stock, params, item.BasePrice)

	// 更新余额
	newBalance := balance.Amount - item.Price

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新背包
	_, err = tx.Exec("UPDATE backpack SET apple = ?, wood = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		backpack.Apple, backpack.Wood, backpack.ID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新市场物品
	_, err = tx.Exec("UPDATE market_items SET price = ?, stock = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		item.Price, item.Stock, item.ID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新余额
	_, err = tx.Exec("UPDATE balance SET amount = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		newBalance, balance.ID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加交易记录
	// 隐私数据
	_, err = tx.Exec(
		"INSERT INTO transactions (transaction_time, our_bank_account_name, counterparty_alias, our_bank_name, counterparty_bank, expense_amount, income_amount, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		time.Now(), "玩家", "萌铺子市场", "玩家银行", "萌铺子市场银行", item.Price, 0, fmt.Sprintf("买入%s", itemType))
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取更新后的市场物品
	var apple MarketItem
	err = db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'apple'").Scan(
		&apple.ID, &apple.Name, &apple.Price, &apple.Stock, &apple.BasePrice, &apple.CreatedAt, &apple.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wood MarketItem
	err = db.QueryRow("SELECT id, name, price, stock, base_price, created_at, updated_at FROM market_items WHERE name = 'wood'").Scan(
		&wood.ID, &wood.Name, &wood.Price, &wood.Stock, &wood.BasePrice, &wood.CreatedAt, &wood.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := MarketItems{
		Apple: apple,
		Wood:  wood,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"backpack":    backpack,
		"marketItems": items,
	})
}
