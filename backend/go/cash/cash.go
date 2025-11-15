package cash

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"own-1Pixel/backend/go/config"
	"own-1Pixel/backend/go/logger"
	"own-1Pixel/backend/go/timeservice"

	_ "modernc.org/sqlite"
)

// 交易记录结构
type Transaction struct {
	ID                 int       `json:"id"`
	TransactionTime    time.Time `json:"transaction_time"`      // 交易时间
	OurBankAccountName string    `json:"our_bank_account_name"` // 己方银行户名
	CounterpartyAlias  string    `json:"counterparty_alias"`    // 对手方别名
	OurBankName        string    `json:"our_bank_name"`         // 己方开户行
	CounterpartyBank   string    `json:"counterparty_bank"`     // 对手方开户行
	ExpenseAmount      float64   `json:"expense_amount"`        // 支出金额
	IncomeAmount       float64   `json:"income_amount"`         // 收入金额
	Balance            *float64  `json:"balance"`               // 己方账户余额（计算得出）
	Note               string    `json:"note"`                  // 附言（用途）
	CreatedAt          time.Time `json:"created_at"`            // 创建时间
}

// 余额信息结构
type Balance struct {
	ID        int       `json:"id"`
	Amount    float64   `json:"amount"`
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// 初始化数据库
func InitDatabase(db *sql.DB) error {
	// 获取全局配置实例
	_config := config.GetConfig()
	cashConfig := _config.Cash

	logger.Info("cash", fmt.Sprintf("初始化现金数据库，路径: %s\n", cashConfig.DbPath))
	var err error

	// 确保数据库目录存在
	dbDir := filepath.Dir(cashConfig.DbPath)
	if _, dirCheckErr := os.Stat(dbDir); os.IsNotExist(dirCheckErr) {
		os.MkdirAll(dbDir, 0755)
	}

	if _, dbCheckErr := os.Stat(cashConfig.DbPath); dbCheckErr == nil {
		// 数据库文件存在，检查表结构是否匹配
		tempDB, dbOpenErr := sql.Open("sqlite", cashConfig.DbPath)
		if dbOpenErr != nil {
			return dbOpenErr
		}

		// 检查transactions表是否存在
		var tableName string
		err = tempDB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='transactions'").Scan(&tableName)
		tableExists := err == nil

		if tableExists {
			// 检查transactions表结构是否匹配
			rows, pragmaQueryErr := tempDB.Query("PRAGMA table_info(transactions)")
			if pragmaQueryErr != nil {
				tempDB.Close()
				return pragmaQueryErr
			}
			defer rows.Close()

			var columns []string
			var columnTypes map[string]string = make(map[string]string)
			for rows.Next() {
				var cid int
				var name string
				var dataType string
				var notNull int
				var dfltValue interface{}
				var pk int
				err = rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk)
				if err != nil {
					tempDB.Close()
					return err
				}
				columns = append(columns, name)
				columnTypes[name] = dataType
			}

			// 检查是否包含所有必需的列
			requiredColumns := []string{"id", "transaction_time", "our_bank_account_name",
				"counterparty_alias", "our_bank_name", "counterparty_bank", "expense_amount",
				"income_amount", "balance", "note", "created_at"}

			needsMigration := false
			for _, reqCol := range requiredColumns {
				found := false
				for _, col := range columns {
					if col == reqCol {
						found = true
						break
					}
				}
				if !found {
					needsMigration = true
					break
				}
			}

			// 检查balance列是否有NOT NULL约束
			if !needsMigration && columnTypes["balance"] != "" {
				// 检查balance列的NOT NULL约束
				var notNull int
				err = tempDB.QueryRow("SELECT NOT NULL FROM pragma_table_info('transactions') WHERE name='balance'").Scan(&notNull)
				if err == nil && notNull == 1 {
					needsMigration = true
				}
			}

			tempDB.Close()

			if needsMigration {
				// 备份旧数据库文件
				backupTime := timeservice.Now().Format("20060102_150405")
				backupPath := filepath.Join(dbDir, fmt.Sprintf("cash_backup_%s.db", backupTime))

				// 复制旧数据库文件到备份文件
				err = copyFile(cashConfig.DbPath, backupPath)
				if err != nil {
					return fmt.Errorf("备份数据库文件失败: %v", err)
				}

				fmt.Printf("旧数据库文件已备份为: %s\n", backupPath)

				// 删除旧数据库文件，以便创建新的
				err = os.Remove(cashConfig.DbPath)
				if err != nil {
					return fmt.Errorf("删除旧数据库文件失败: %v", err)
				}
			}
		} else {
			tempDB.Close()
		}
	}

	// 创建交易记录表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transaction_time DATETIME NOT NULL,
			our_bank_account_name TEXT,
			counterparty_alias TEXT,
			our_bank_name TEXT,
			counterparty_bank TEXT,
			expense_amount REAL DEFAULT 0,
			income_amount REAL DEFAULT 0,
			balance REAL,
			note TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("创建交易记录表失败: %v\n", err))
		return err
	}

	// 创建余额表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			amount REAL NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("创建余额表失败: %v\n", err))
		return err
	}

	// 检查是否有余额记录，如果没有则初始化
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM balance").Scan(&count)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("查询余额记录数量失败: %v\n", err))
		return err
	}

	if count == 0 {
		_, err = db.Exec("INSERT INTO balance (amount) VALUES (0)")
		if err != nil {
			logger.Info("cash", fmt.Sprintf("初始化余额记录失败: %v\n", err))
			return err
		}
	}

	logger.Info("cash", "现金数据库初始化完成\n")
	return nil
}

// 复制文件的辅助函数
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("打开源文件失败: %v\n", err))
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("创建目标文件失败: %v\n", err))
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("复制文件内容失败: %v\n", err))
		return err
	}

	// 复制文件权限
	sourceInfo, err := os.Stat(src)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("获取源文件信息失败: %v\n", err))
		return err
	}
	err = os.Chmod(dst, sourceInfo.Mode())
	if err != nil {
		logger.Info("cash", fmt.Sprintf("设置文件权限失败: %v\n", err))
		return err
	}
	return nil
}

// 获取当前余额
func GetBalance(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	logger.Info("cash", "获取账户余额请求\n")
	var balance Balance
	err := db.QueryRow("SELECT id, amount, updated_at FROM balance ORDER BY id DESC LIMIT 1").Scan(&balance.ID, &balance.Amount, &balance.UpdatedAt)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("获取账户余额失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "获取账户余额失败",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("cash", fmt.Sprintf("获取账户余额成功，当前余额: %.2f\n", balance.Amount))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"balance": balance,
	})
}

// 更新余额
func UpdateBalance(db *sql.DB, amount float64) error {
	_, err := db.Exec("UPDATE balance SET amount = ?, updated_at = CURRENT_TIMESTAMP", amount)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("更新余额失败: %v\n", err))
		return err
	}
	return nil
}

// 获取所有交易记录
func GetTransactions(db *sql.DB, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	logger.Info("cash", "获取交易记录请求\n")
	// 获取所有交易记录，按交易时间升序排列以便计算余额
	rows, err := db.Query("SELECT id, transaction_time, our_bank_account_name, counterparty_alias, our_bank_name, counterparty_bank, expense_amount, income_amount, note, created_at FROM transactions ORDER BY transaction_time ASC")
	if err != nil {
		logger.Info("cash", fmt.Sprintf("获取交易记录失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "获取交易记录失败",
			"error":   err.Error(),
		})
		return
	}
	defer rows.Close()

	var transactions []Transaction
	var runningBalance float64 = 0

	// 按时间顺序计算余额
	for rows.Next() {
		var t Transaction
		t.Balance = new(float64) // 初始化Balance指针

		err := rows.Scan(&t.ID, &t.TransactionTime, &t.OurBankAccountName, &t.CounterpartyAlias, &t.OurBankName, &t.CounterpartyBank, &t.ExpenseAmount, &t.IncomeAmount, &t.Note, &t.CreatedAt)
		if err != nil {
			logger.Info("cash", fmt.Sprintf("扫描交易记录失败: %v\n", err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "扫描交易记录失败",
				"error":   err.Error(),
			})
			return
		}

		// 计算余额：当前余额 = 上一条记录的余额 + 收入金额 - 支出金额
		runningBalance = runningBalance + t.IncomeAmount - t.ExpenseAmount
		*t.Balance = runningBalance

		transactions = append(transactions, t)
	}

	// 确保总是返回数组，即使没有交易记录
	if transactions == nil {
		transactions = make([]Transaction, 0)
	}

	// 反转数组，使最新的交易记录显示在前面
	for i, j := 0, len(transactions)-1; i < j; i, j = i+1, j-1 {
		transactions[i], transactions[j] = transactions[j], transactions[i]
	}

	logger.Info("cash", fmt.Sprintf("获取交易记录成功，共 %d 条记录\n", len(transactions)))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"transactions": transactions,
	})
}

// 添加交易记录
func AddTransaction(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		logger.Info("cash", fmt.Sprintf("添加交易记录请求失败，不支持的请求方法: %s\n", r.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "不支持的请求方法",
		})
		return
	}

	logger.Info("cash", "添加交易记录请求\n")
	// 使用临时结构体来解析JSON，不包含TransactionTime字段
	type TempTransaction struct {
		OurBankAccountName string  `json:"our_bank_account_name"`
		CounterpartyAlias  string  `json:"counterparty_alias"`
		OurBankName        string  `json:"our_bank_name"`
		CounterpartyBank   string  `json:"counterparty_bank"`
		ExpenseAmount      float64 `json:"expense_amount"`
		IncomeAmount       float64 `json:"income_amount"`
		Note               string  `json:"note"`
	}

	var tempT TempTransaction
	err := json.NewDecoder(r.Body).Decode(&tempT)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("解析交易记录JSON失败: %v\n", err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "解析交易记录JSON失败",
			"error":   err.Error(),
		})
		return
	}

	// 创建Transaction结构体并设置当前时间
	var t Transaction
	t.OurBankAccountName = tempT.OurBankAccountName
	t.CounterpartyAlias = tempT.CounterpartyAlias
	t.OurBankName = tempT.OurBankName
	t.CounterpartyBank = tempT.CounterpartyBank
	t.ExpenseAmount = tempT.ExpenseAmount
	t.IncomeAmount = tempT.IncomeAmount
	t.Note = tempT.Note

	// 使用时间服务提供的可信时间并格式化为"年-月-日 时:分:秒"
	currentTime := timeservice.Now().Format("2006-01-02 15:04:05")
	t.TransactionTime, _ = time.Parse("2006-01-02 15:04:05", currentTime)

	// 获取当前余额
	var currentBalance float64
	err = db.QueryRow("SELECT amount FROM balance ORDER BY id DESC LIMIT 1").Scan(&currentBalance)
	if err != nil {
		// 如果没有余额记录，将余额设为0
		currentBalance = 0
	}

	// 计算新余额
	newBalance := currentBalance + t.IncomeAmount - t.ExpenseAmount
	t.Balance = &newBalance

	// 插入交易记录，不保存balance字段到数据库
	result, err := db.Exec(
		"INSERT INTO transactions (transaction_time, our_bank_account_name, counterparty_alias, our_bank_name, counterparty_bank, expense_amount, income_amount, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		t.TransactionTime, t.OurBankAccountName, t.CounterpartyAlias, t.OurBankName, t.CounterpartyBank, t.ExpenseAmount, t.IncomeAmount, t.Note,
	)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("插入交易记录失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "插入交易记录失败",
			"error":   err.Error(),
		})
		return
	}

	// 获取新插入记录的ID
	id, _ := result.LastInsertId()
	t.ID = int(id)

	// 更新余额
	err = UpdateBalance(db, newBalance)
	if err != nil {
		logger.Info("cash", fmt.Sprintf("更新余额失败: %v\n", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "更新余额失败",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("cash", fmt.Sprintf("添加交易记录成功，ID: %d，金额: %.2f，新余额: %.2f\n", t.ID, t.IncomeAmount-t.ExpenseAmount, newBalance))
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     "交易记录添加成功",
		"transaction": t,
	})
}
