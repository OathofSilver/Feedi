package mysql

import (
	"database/sql"
	"feed/backend/internal/account"
	"feed/backend/internal/comment"
	"feed/backend/internal/config"
	"feed/backend/internal/like"
	"feed/backend/internal/notification"
	"feed/backend/internal/social"
	"feed/backend/internal/video"
	"fmt"
	"time"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// EnsureDatabase 确保目标数据库存在：用不带库名的裸连接执行 CREATE DATABASE IF NOT EXISTS。
// GORM 的 AutoMigrate 只建表不建库，若库不存在连接阶段即失败，故启动建库前先确保库存在。
func EnsureDatabase(dbcfg config.Database) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
		dbcfg.User, dbcfg.Password, dbcfg.Host, dbcfg.Port)

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 库名由调用方配置文件提供，非用户输入注入点；仍以反引号包裹防止非法库名解析错误
	_, err = conn.Exec(fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbcfg.DBName))
	return err
}

func NewDB(dbcfg config.Database) (*gorm.DB, error) {
	// 库不存在则先创建，保证全新环境下 AutoMigrate 能顺利建表
	if err := EnsureDatabase(dbcfg); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbcfg.User, dbcfg.Password, dbcfg.Host, dbcfg.Port, dbcfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// 连接池调优：GORM 默认 MaxIdleConns=2，高并发下会频繁建断 MySQL 短连接，
	// 在本机临时端口有限的环境会形成端口耗尽(TIME_WAIT 洪峰)导致连接失败。
	// 保持常驻长连接池，消除短连接风暴；MaxOpenConns 需低于 MySQL max_connections。
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	const (
		maxOpenConns = 100 // 上限：与 MySQL 服务端 max_connections(默认151)留出余量
		maxIdleConns = 50  // 常驻空闲连接，吸收并发请求波动
		maxLifetime  = time.Hour
	)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(maxLifetime)

	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&account.Account{},
		&social.Social{},
		&video.Video{},
		&video.OutboxMsg{},
		&video.Tag{},
		&video.VideoTag{},
		&like.Like{},
		&comment.Comment{},
		&notification.Notification{},
	)
}

func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
