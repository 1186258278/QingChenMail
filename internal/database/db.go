package database

import (
	"crypto/rand"
	"log"
	"math/big"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// generateInitialPassword 生成随机初始密码 (8位字母数字)
func generateInitialPassword() string {
	const letters = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz"
	ret := make([]byte, 8)
	for i := 0; i < 8; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			ret[i] = letters[i%len(letters)]
			continue
		}
		ret[i] = letters[num.Int64()]
	}
	return string(ret)
}

var DB *gorm.DB

// HashPassword 生成 Bcrypt 哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash 验证密码
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// InitDB 初始化并校准数据库
func InitDB() {
	var err error
	
	// 1. 连接数据库
	// 使用自定义 Logger 以便在迁移时能看到关键信息
	newLogger := logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,   // Slow SQL threshold
			LogLevel:                  logger.Warn,   // Log level
			IgnoreRecordNotFoundError: true,          // Ignore ErrRecordNotFound error for logger
			ParameterizedQueries:      true,          // Don't include params in the SQL log
			Colorful:                  true,          // Disable color
		},
	)

	DB, err = gorm.Open(sqlite.Open("goemail.db"), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatalf("[DB] Failed to connect: %v", err)
	}

	// 优化 SQLite 并发性能
	sqlDB, err := DB.DB()
	if err == nil {
		sqlDB.Exec("PRAGMA journal_mode=WAL")
		sqlDB.Exec("PRAGMA busy_timeout=5000")
		sqlDB.Exec("PRAGMA synchronous=NORMAL")
		sqlDB.SetMaxOpenConns(1) // SQLite 单写者模型
	}

	log.Println("[DB] Connection established. Starting calibration...")

	// 2. 注册所有模型 (用于 AutoMigrate)
	models := []interface{}{
		&SchemaVersion{}, // 核心版本控制表
		&User{},
		&SMTPConfig{},
		&Certificate{}, // 证书管理 (需要在 Domain 之前创建，因为 Domain 引用它)
		&Domain{},
		&Template{},
		&EmailLog{},
		&Sender{},
		&APIKey{},
		&EmailQueue{},
		&AttachmentFile{},
		&ForwardRule{},
		&ForwardLog{},
		&ContactGroup{},
		&Contact{},
		&Campaign{},
		&Inbox{},
	}

	// 3. 执行基础结构校准 (AutoMigrate)
	// 这会创建不存在的表，添加缺失的列
	if err := DB.AutoMigrate(models...); err != nil {
		log.Fatalf("[DB] Schema calibration failed: %v", err)
	}
	log.Println("[DB] Schema structure calibrated.")

	// 4. 版本化迁移与数据清洗
	runMigrations()

	// 5. 数据种子 (Seeding)
	runSeeding()

	log.Println("[DB] Database ready.")
}

// runMigrations 执行版本化迁移
func runMigrations() {
	// 定义迁移步骤
	// 每次代码更新涉及无法自动处理的变更时，在此添加新步骤
	migrations := []struct {
		Version     int
		Description string
		Action      func(*gorm.DB) error
	}{
		{
			Version:     1,
			Description: "Initial Setup",
			Action: func(db *gorm.DB) error {
				// 可以在这里执行一些初始化 SQL，或者什么都不做(因为 AutoMigrate 已经处理了基础结构)
				return nil
			},
		},
		{
			Version:     2,
			Description: "Clean Orphaned Forward Rules",
			Action: func(db *gorm.DB) error {
				// 示例：清理没有对应域名的孤儿转发规则
				result := db.Exec("DELETE FROM forward_rules WHERE domain_id NOT IN (SELECT id FROM domains)")
				if result.RowsAffected > 0 {
					log.Printf("[DB] Cleaned %d orphaned forward rules", result.RowsAffected)
				}
				return nil
			},
		},
		// 未来示例：如果需要将 email_logs 的 recipient 字段长度扩大，或者做数据转换
		// {
		// 	Version: 3,
		// 	Description: "Migrate Status Code",
		// 	Action: func(db *gorm.DB) error { ... },
		// },
	}

	var currentVer SchemaVersion
	// 获取当前最新版本
	if err := DB.Order("version desc").First(&currentVer).Error; err != nil {
		// 如果没有记录，说明是新库或未初始化版本表
		currentVer.Version = 0
	}

	for _, m := range migrations {
		if m.Version > currentVer.Version {
			log.Printf("[DB] Applying migration v%d: %s...", m.Version, m.Description)
			if err := m.Action(DB); err != nil {
				log.Fatalf("[DB] Migration v%d failed: %v", m.Version, err)
			}
			
			// 记录新版本
			DB.Create(&SchemaVersion{
				Version:     m.Version,
				Description: m.Description,
				AppliedAt:   time.Now(),
			})
		}
	}
}

// runSeeding 填充/校准基础数据
func runSeeding() {
	// 1. 校准默认管理员
	var adminCount int64
	DB.Model(&User{}).Where("username = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		// 生成随机初始密码并打印到控制台
		defaultPass := generateInitialPassword()
		log.Println("[DB] Seeding default admin user...")
		log.Printf("╔══════════════════════════════════════════════╗")
		log.Printf("║  Default admin password: %-20s ║", defaultPass)
		log.Printf("║  Please change it after first login!        ║")
		log.Printf("╚══════════════════════════════════════════════╝")

		hashedPassword, err := HashPassword(defaultPass)
		if err != nil {
			log.Printf("[DB] Warning: Failed to hash default password: %v, using plain text", err)
			DB.Create(&User{Username: "admin", Password: defaultPass})
		} else {
			DB.Create(&User{Username: "admin", Password: hashedPassword})
		}
	}

	// 2. 校准示例模板
	var tplCount int64
	DB.Model(&Template{}).Count(&tplCount)
	if tplCount == 0 {
		log.Println("[DB] Seeding example templates...")
		DB.Create(&Template{
			Name:    "欢迎邮件 (示例)",
			Subject: "欢迎使用 QingChen Mail",
			Body:    "<h1>你好 {username},</h1><p>感谢选择 QingChen Mail 自建邮局系统。</p>",
		})
	}

	// 3. 数据完整性检查 (Data Integrity Check)
	// 检查是否有缺失文件的附件记录
	// var lostFiles int64
	// 这里只做统计，不贸然删除，以免误删
	// 实际生产中可以标记状态
	// DB.Model(&AttachmentFile{}).Where("file_path NOT LIKE ?", "http%").Count(&lostFiles)
	// if lostFiles > 0 {
	// 	log.Printf("[DB] Warning: Found %d attachment records. Please verify file storage.", lostFiles)
	// }
}

// GetStats 获取统计信息
func GetStats() (Stats, error) {
	var stats Stats
	var err error

	// 总发送量
	if err = DB.Model(&EmailLog{}).Count(&stats.TotalSent).Error; err != nil {
		return stats, err
	}

	// 今日发送量
	startOfDay := time.Now().Truncate(24 * time.Hour)
	if err = DB.Model(&EmailLog{}).Where("created_at >= ?", startOfDay).Count(&stats.TodaySent).Error; err != nil {
		return stats, err
	}

	// 成功数量
	if err = DB.Model(&EmailLog{}).Where("status = ?", "success").Count(&stats.SuccessCount).Error; err != nil {
		return stats, err
	}

	// 失败数量
	if err = DB.Model(&EmailLog{}).Where("status = ?", "failed").Count(&stats.FailureCount).Error; err != nil {
		return stats, err
	}

	// 最后发送时间
	var lastLog EmailLog
	if err = DB.Order("created_at desc").First(&lastLog).Error; err == nil {
		stats.LastSentTime = &lastLog.CreatedAt
	}

	// 趋势数据
	type TrendResult struct {
		Hour  string
		Count int64
	}
	var results []TrendResult
	
	now := time.Now()
	startTime := now.Add(-12 * time.Hour)

	err = DB.Model(&EmailLog{}).
		Select("strftime('%H:00', created_at) as hour, count(*) as count").
		Where("created_at >= ?", startTime).
		Group("hour").
		Order("hour asc").
		Scan(&results).Error
	
	if err == nil {
		stats.Trend = make([]TrendPoint, 0)
		resultMap := make(map[string]int64)
		for _, r := range results {
			resultMap[r.Hour] = r.Count
		}

		for i := 12; i >= 0; i-- {
			t := now.Add(time.Duration(-i) * time.Hour)
			hourKey := t.Format("15:00")
			count := resultMap[hourKey]
			stats.Trend = append(stats.Trend, TrendPoint{
				Time:  hourKey,
				Count: count,
			})
		}
	}

	return stats, nil
}
