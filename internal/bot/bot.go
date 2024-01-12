package bot

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"os"
	"telegram-dice-bot/internal/database"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
	"time"
)

const (
	TelegramAPIToken = "TELEGRAM_API_TOKEN"
	WhiteList        = "WHITE_LIST"
)

var (
	db      *gorm.DB
	redisDB *redis.Client
)

func StartBot() {
	initDB()

	bot := initTelegramBot()

	initGameTask(bot)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {

		if update.Message != nil {
			go handleMessage(bot, update.Message)
		} else if update.CallbackQuery != nil {
			go handleCallbackQuery(bot, update.CallbackQuery)
		}
	}
}

func initGameTask(bot *tgbotapi.BotAPI) {
	// 查出所有已开启的对话
	chatGroup := &model.ChatGroup{
		GameplayStatus: enums.GameplayStatusON.Value,
	}

	chatGroups, err := chatGroup.ListByGameplayStatus(db)
	if len(chatGroups) == 0 {
		logrus.Println("暂无可开启的群配置")
		return
	}
	if err != nil {
		logrus.Fatal("初始化任务失败:", err)
	}
	for _, group := range chatGroups {
		// 查询当前对话在缓存中是否有未执行的任务
		redisKey := fmt.Sprintf(RedisCurrentIssueNumberKey, group.Id)
		issueNumberResult := redisDB.Get(redisDB.Context(), redisKey)
		if errors.Is(issueNumberResult.Err(), redis.Nil) || issueNumberResult == nil {
			// 没有未开奖的任务，开始新的期号
			logrus.Printf("键 %s 不存在", redisKey)
			issueNumber := time.Now().Format("20060102150405")

			go gameTaskStart(bot, group, issueNumber)
			continue
		} else if issueNumberResult.Err() != nil {
			logrus.Println("获取值时发生错误:", issueNumberResult.Err())
			continue
		} else {
			// 有未开奖的任务
			result, _ := issueNumberResult.Result()
			logrus.Printf("有未开奖的任务期号:%s", result)
			go gameTaskStart(bot, group, result)
			continue
		}
	}
}

func initDB() {
	var err error
	db, err = database.InitDB(os.Getenv(database.DBConnectionString))
	if err != nil {
		logrus.Fatal("连接数据库失败:", err)
	}

	err = db.AutoMigrate(&model.ChatGroup{})
	if err != nil {
		logrus.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.ChatGroupAdmin{})
	if err != nil {
		logrus.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.QuickThereConfig{})
	if err != nil {
		logrus.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.QuickThereLotteryRecord{})
	if err != nil {
		logrus.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.ChatGroupUser{})
	if err != nil {
		logrus.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.QuickThereBetRecord{})
	if err != nil {
		logrus.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.LotteryRecord{})
	if err != nil {
		logrus.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.BetRecord{})
	if err != nil {
		logrus.Fatal("自动迁移表结构失败:", err)
	}

	redisDB, err = database.InitRedisDB(os.Getenv(database.RedisDBConnectionString))
	if err != nil {
		logrus.Fatal("连接Redis数据库失败:", err)
	}

}

func initTelegramBot() *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(os.Getenv(TelegramAPIToken))
	if err != nil {
		logrus.Panic(err)
	}

	bot.Debug = false
	logrus.WithFields(logrus.Fields{
		"userName": bot.Self.UserName,
	}).Infof("机器人已授权")
	return bot
}
