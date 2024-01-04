package database

import (
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	DBConnectionString      = "MYSQL_DSN"
	RedisDBConnectionString = "REDIS_CONN_STRING"
)

func InitDB(dsn string) (*gorm.DB, error) {

	var err error
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		logrus.WithField("err", err).Fatal("连接数据库失败")
		return nil, err
	}

	return db, nil
}
func InitRedisDB(dsn string) (*redis.Client, error) {
	options, err := redis.ParseURL(dsn)
	if err != nil {
		logrus.WithField("err", err).Fatal("解析 Redis URL 失败")
	}

	redisDB := redis.NewClient(options)

	_, err = redisDB.Ping(redisDB.Context()).Result()
	if err != nil {
		logrus.WithField("err", err).Fatal("连接到 Redis 失败")
	}
	logrus.WithField("dsn", dsn).Info("已连接到 Redis")
	return redisDB, nil
}
