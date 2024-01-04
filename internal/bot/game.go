package bot

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
	"time"
)

const (
	RedisCurrentIssueNumberKey = "CURRENT_ISSUE_NUMBER:CHAT_GROUP_ID:%s"
)

var (
	stopTaskFlags = make(map[string]chan struct{})
)

func gameStart(bot *tgbotapi.BotAPI, group *model.ChatGroup) {

	issueNumber := time.Now().Format("20060102150405")

	// 查找上个未开奖的期号
	redisKey := fmt.Sprintf(RedisCurrentIssueNumberKey, group.Id)
	issueNumberResult := redisDB.Get(redisDB.Context(), redisKey)
	if errors.Is(issueNumberResult.Err(), redis.Nil) || issueNumberResult == nil {
		lotteryDrawTipMsgConfig := tgbotapi.NewMessage(group.TgChatGroupId, fmt.Sprintf("第%s期 %d分钟后开奖", issueNumber, group.GameDrawCycle))
		_, err := sendMessage(bot, &lotteryDrawTipMsgConfig)
		if err != nil {
			blockedOrKicked(err, group.TgChatGroupId)
			return
		}
		// 存储当前期号和对话ID
		err = redisDB.Set(redisDB.Context(), redisKey, issueNumber, 0).Err()
		if err != nil {
			logrus.WithField("err", err).Error("存储新期号和对话ID异常")
			return
		}
	} else if issueNumberResult.Err() != nil {
		logrus.WithField("err", issueNumberResult.Err()).Error("redis获取期号异常")
		return
	} else {
		result, _ := issueNumberResult.Result()
		issueNumber = result
		lotteryDrawTipMsgConfig := tgbotapi.NewMessage(group.TgChatGroupId, fmt.Sprintf("第%s期 %d分钟后开奖", issueNumber, group.GameDrawCycle))
		_, err := sendMessage(bot, &lotteryDrawTipMsgConfig)
		if err != nil {
			blockedOrKicked(err, group.TgChatGroupId)
			return
		}
	}

	gameTaskStart(bot, group, issueNumber)
}
func gameStop(group *model.ChatGroup) {
	gameTaskStop(group)
}

func gameTaskStart(bot *tgbotapi.BotAPI, group *model.ChatGroup, issueNumber string) {
	gameTaskStop(group)

	chatLock := getChatLock(group.Id)
	chatLock.Lock()
	defer chatLock.Unlock()

	stopTaskFlags[group.Id] = make(chan struct{})
	go func(stopCh <-chan struct{}) {

		ticker := time.NewTicker(time.Duration(group.GameDrawCycle) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if group.GameplayType == enums.QuickThere.Value {
					nextIssueNumber, err := quickThereTask(bot, group, issueNumber)
					if err != nil {
						return
					}
					issueNumber = nextIssueNumber
				} else {
					return
				}
			case <-stopCh:
				logrus.WithField("chatGroupId", group.Id).Info("已关闭任务")
				return
			}
		}

	}(stopTaskFlags[group.Id])
}
func gameTaskStop(group *model.ChatGroup) {
	chatLock := getChatLock(group.Id)
	chatLock.Lock()
	defer chatLock.Unlock()

	if stopFlag, ok := stopTaskFlags[group.Id]; ok {
		logrus.WithField("groupId", group.Id).Info("停止聊天ID的任务")
		close(stopFlag)
		delete(stopTaskFlags, group.Id)
	} else {
		logrus.WithField("groupId", group.Id).Warn("没有要停止的聊天ID的任务")
	}
}
