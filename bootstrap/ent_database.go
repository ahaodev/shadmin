package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"shadmin/ent"
	"shadmin/ent/migrate"
	"shadmin/internal/conf"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func NewEntDatabase(env *conf.Env) *ent.Client {
	var client *ent.Client
	var err error

	// Determine database type
	dbType := strings.ToLower(env.DBType)
	if dbType == "" {
		dbType = "sqlite"
	}

	log.Printf("🔗 Database Initialization:")
	log.Printf("  - Type: %s", strings.ToUpper(dbType))

	switch dbType {
	case "postgres", "postgresql":
		client, err = connectPostgreSQL(env)
	case "mysql":
		client, err = connectMySQL(env)
	case "sqlite", "sqlite3":
		client, err = connectSQLite()
	default:
		log.Fatalf("❌ Unsupported database type: %s. Supported types: sqlite, postgres, mysql", dbType)
	}

	if err != nil {
		log.Fatal("❌ Failed to connect to database:", err)
	}

	// Auto migrate schema
	log.Printf("📋 Running database schema migration...")
	ctx := context.Background()
	if err := client.Schema.Create(ctx,
		migrate.WithDropIndex(true),
		migrate.WithDropColumn(true),
	); err != nil {
		log.Fatal("❌ Failed to create schema resources:", err)
	}

	log.Printf("✅ Database schema migration completed")
	log.Printf("✅ Connected to %s database successfully with Ent", strings.ToUpper(dbType))
	return client
}

func connectSQLite() (*ent.Client, error) {
	// 修改这里：确保路径指向 .db 文件
	dbPath := "./.database/data.db"

	log.Printf("📄 SQLite Config:")
	log.Printf("  - Database Path: %s", dbPath)

	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create DB directory %s: %v", dir, err)
		}
		log.Printf("✅ Database directory ensured: %s", dir)
	}

	// SQLite 单写者特性下，并发写会触发 "database is locked"。
	// 通过 DSN 参数缓解：
	//   _fk=1              启用外键约束
	//   _journal_mode=WAL  开启 WAL，读写不互相阻塞（读者不再阻塞唯一写者）
	//   _busy_timeout=5000 遇锁时等待重试最多 5s，而不是立即失败
	//   _txlock=immediate  事务开始即以 BEGIN IMMEDIATE 抢占写锁，串行化写事务，
	//                      避免两个事务各自升级写锁导致的死锁式 locked。
	dsn := fmt.Sprintf("file:%s?_fk=1&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate", dbPath)
	log.Printf("📡 Connecting to SQLite database...")
	return ent.Open("sqlite3", dsn)
}

func connectPostgreSQL(env *conf.Env) (*ent.Client, error) {
	dsn := env.DBDSN
	if dsn == "" {
		return nil, fmt.Errorf("PostgreSQL DSN is required but not provided")
	}

	log.Printf("📄 PostgreSQL Config:")
	log.Printf("  - DSN: %s", maskPassword(dsn))
	log.Printf("📡 Connecting to PostgreSQL database...")

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		log.Printf("❌ PostgreSQL connection failed: %v", err)
		return nil, err
	}

	log.Printf("✅ PostgreSQL connection successful")
	return client, nil
}

func connectMySQL(env *conf.Env) (*ent.Client, error) {
	dsn := env.DBDSN
	if dsn == "" {
		return nil, fmt.Errorf("MySQL DSN is required but not provided")
	}

	log.Printf("📄 MySQL Config:")
	log.Printf("  - DSN: %s", maskPassword(dsn))
	log.Printf("📡 Connecting to MySQL database...")

	client, err := ent.Open("mysql", dsn)
	if err != nil {
		log.Printf("❌ MySQL connection failed: %v", err)
		return nil, err
	}

	log.Printf("✅ MySQL connection successful")
	return client, nil
}

// maskPassword masks the password in DSN for logging
func maskPassword(dsn string) string {
	if strings.Contains(dsn, "@") {
		parts := strings.Split(dsn, "@")
		if len(parts) >= 2 {
			userInfo := parts[0]
			if strings.Contains(userInfo, ":") {
				userParts := strings.Split(userInfo, ":")
				if len(userParts) >= 3 {
					userParts[len(userParts)-1] = "***"
					parts[0] = strings.Join(userParts, ":")
				}
			}
			return strings.Join(parts, "@")
		}
	}
	return dsn
}

// CloseEntConnection 关闭Ent数据库连接
func CloseEntConnection(client *ent.Client) {
	if client == nil {
		return
	}

	err := client.Close()
	if err != nil {
		log.Fatal("Failed to close database connection:", err)
	}

	log.Println("Database connection closed.")
}
