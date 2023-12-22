package database

import (
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
)

const (
	DBConnectionString      = "MYSQL_DSN"
	RedisDBConnectionString = "REDIS_CONN_STRING"
)

func InitDB(dsn string) (*gorm.DB, error) {
	var err error
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
		return nil, err
	}

	return db, nil
}
func InitRedisDB(dsn string) (*redis.Client, error) {
	options, err := redis.ParseURL(dsn)
	if err != nil {
		log.Fatal("解析 Redis URL 失败:", err)
	}

	redisDB := redis.NewClient(options)

	_, err = redisDB.Ping(redisDB.Context()).Result()
	if err != nil {
		log.Fatal("连接到 Redis 失败:", err)
	}
	log.Printf("已连接到 Redis %s", dsn)
	return redisDB, nil
}
