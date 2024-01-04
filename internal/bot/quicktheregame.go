package bot

import (
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
	"telegram-dice-bot/internal/utils"
	"time"
)

func quickThereTask(bot *tgbotapi.BotAPI, group *model.ChatGroup, issueNumber string) (nextIssueNumber string, err error) {
	// 执行任务前对群组校验 如果只剩1个人那必然是自己
	chatMembersLen, err := bot.GetChatMembersCount(tgbotapi.ChatMemberCountConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: group.TgChatGroupId,
		},
	})
	if chatMembersLen == 1 {
		logrus.WithField("GroupId", group.Id).Warn("群内只剩机器人")
		// 更新群状态
		group.GameplayStatus = 0
		db.Save(group)
		return "", errors.New("群内只剩机器人")
	}

	redisKey := fmt.Sprintf(RedisCurrentIssueNumberKey, group.Id)
	// 删除当前期号和对话ID
	err = redisDB.Del(redisDB.Context(), redisKey).Err()
	if err != nil {
		logrus.WithField("redisKey", redisKey).Error("删除当前期号和对话ID异常")
		return "", err
	}

	currentTime := time.Now().Format("2006-01-02 15:04:05")

	diceValues, err := rollDice(bot, group.TgChatGroupId, 3)
	if err != nil {
		blockedOrKicked(err, group.TgChatGroupId)
		return "", err
	}
	count := sumDiceValues(diceValues)
	singleOrDouble, bigOrSmall := determineResult(count)

	time.Sleep(3 * time.Second)
	triplet := 0
	if diceValues[0] == diceValues[1] && diceValues[1] == diceValues[2] {
		triplet = 1
	}
	message, err := formatMessage(diceValues[0], diceValues[1], diceValues[2], count, singleOrDouble, bigOrSmall, triplet, issueNumber)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"issueNumber": issueNumber,
			"err":         err,
		}).Warn("开奖结果消息格式化异常")
	}

	tx := db.Begin()

	id, err := utils.NextID()
	if err != nil {
		logrus.Error("SnowFlakeId create error")
		return "", err
	}

	// 插入开奖主表
	record := &model.LotteryRecord{
		Id:           id,
		ChatGroupId:  group.Id,
		IssueNumber:  issueNumber,
		GameplayType: enums.QuickThere.Value,
		CreateTime:   currentTime,
	}
	err = record.Create(tx)
	if err != nil {
		logrus.WithField("err", err).Error("开奖记录插入异常")
		tx.Rollback()
		return "", err
	}

	// 插入快三开奖表
	lotteryRecord := &model.QuickThereLotteryRecord{
		Id:           id,
		ChatGroupId:  group.Id,
		IssueNumber:  issueNumber,
		ValueA:       diceValues[0],
		ValueB:       diceValues[1],
		ValueC:       diceValues[2],
		Total:        count,
		SingleDouble: singleOrDouble,
		BigSmall:     bigOrSmall,
		Triplet:      triplet,
		CreateTime:   currentTime,
	}

	err = lotteryRecord.Create(tx)
	if err != nil {
		logrus.WithField("err", err).Error("开奖记录插入异常")
		tx.Rollback()
		return "", err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		// 提交事务时出现异常，回滚事务
		tx.Rollback()
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("开奖历史", enums.CallbackLotteryHistory.Value),
		),
	)

	msg := tgbotapi.NewMessage(group.TgChatGroupId, message)
	msg.ReplyMarkup = keyboard
	_, err = sendMessage(bot, &msg)
	if err != nil {
		blockedOrKicked(err, group.TgChatGroupId)
		return "", err
	}

	nextIssueNumber = time.Now().Format("20060102150405")

	lotteryDrawTipMsgConfig := tgbotapi.NewMessage(group.TgChatGroupId, fmt.Sprintf("第%s期 %d分钟后开奖", nextIssueNumber, group.GameDrawCycle))
	_, err = sendMessage(bot, &lotteryDrawTipMsgConfig)
	if err != nil {
		blockedOrKicked(err, group.TgChatGroupId)
		return
	}

	// 设置新的期号和对话ID
	err = redisDB.Set(redisDB.Context(), redisKey, nextIssueNumber, 0).Err()
	if err != nil {
		logrus.WithField("err", err).Warn("存储新期号和对话ID异常")
	}

	// 遍历下注记录，计算竞猜结果
	go func() {
		// 获取所有参与竞猜的用户下注记录
		quickThereBetRecord := &model.QuickThereBetRecord{
			ChatGroupId: group.Id,
			IssueNumber: issueNumber,
		}
		quickThereBetRecords, err := quickThereBetRecord.ListByChatGroupIdAndIssueNumber(db)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ChatGroupId": group.Id,
				"IssueNumber": issueNumber,
				"err":         err,
			}).Error("获取用户下注记录异常")
			return
		}
		// 查询此群的快三配置
		quickThereConfig, err := model.QueryQuickThereConfigByChatGroupId(db, group.Id)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ChatGroupId": group.Id,
				"err":         err,
			}).Error("查询群的快三配置异常")
			return
		}

		for _, betRecord := range quickThereBetRecords {
			// 更新用户余额
			updateBalanceByQuickThere(bot, quickThereConfig, betRecord, lotteryRecord)
		}
	}()

	return nextIssueNumber, nil
}

// rollDice 模拟多次掷骰子。
func rollDice(bot *tgbotapi.BotAPI, chatID int64, numDice int) ([]int, error) {
	diceValues := make([]int, numDice)
	diceConfig := tgbotapi.NewDiceWithEmoji(chatID, "🎲")

	for i := 0; i < numDice; i++ {
		diceMsg, err := bot.Send(diceConfig)
		if err != nil {
			logrus.WithField("err", err).Error("发送骰子消息异常")
			return nil, err
		}
		diceValues[i] = diceMsg.Dice.Value
	}

	return diceValues, nil
}

func sumDiceValues(diceValues []int) int {
	sum := 0
	for _, value := range diceValues {
		sum += value
	}
	return sum
}

// determineResult 根据骰子值的总和确定结果（单/双，大/小）。
func determineResult(count int) (string, string) {
	var singleOrDouble string
	var bigOrSmall string

	if count <= 10 {
		bigOrSmall = enums.Small.Value
	} else {
		bigOrSmall = enums.Big.Value
	}

	if count%2 == 1 {
		singleOrDouble = enums.Single.Value
	} else {
		singleOrDouble = enums.Double.Value
	}

	return singleOrDouble, bigOrSmall
}

func formatMessage(valueA int, valueB int, valueC int, count int, singleOrDouble, bigOrSmall string, triplet int, issueNumber string) (string, error) {
	tripletStr := ""
	if triplet == 1 {
		tripletStr = "【豹子】"
	}

	singleOrDoubleType, b := enums.GetGameLotteryType(singleOrDouble)
	if !b {
		logrus.WithFields(logrus.Fields{
			"singleOrDouble": singleOrDouble,
		}).Error("开奖结果映射异常")
		return "", errors.New("开奖结果映射异常")
	}
	bigOrSmallType, b := enums.GetGameLotteryType(bigOrSmall)
	if !b {
		logrus.WithFields(logrus.Fields{
			"bigOrSmall": bigOrSmall,
		}).Error("开奖结果映射异常")
		return "", errors.New("开奖结果映射异常")
	}

	return fmt.Sprintf(""+
		"点数: %d %d %d %s\n"+
		"总点数: %d \n"+
		"[单/双]: %s \n"+
		"[大/小]: %s \n"+
		"期号: %s ",
		valueA, valueB, valueC, tripletStr,
		count,
		singleOrDoubleType.Name,
		bigOrSmallType.Name,
		issueNumber,
	), nil
}

// updateBalance 更新用户余额
func updateBalanceByQuickThere(bot *tgbotapi.BotAPI, quickThereConfig *model.QuickThereConfig, betRecord *model.QuickThereBetRecord, lotteryRecord *model.QuickThereLotteryRecord) {

	// 查找该用户信息
	chatGroupUser := &model.ChatGroupUser{Id: betRecord.ChatGroupUserId}
	chatGroupUser, err := chatGroupUser.QueryById(db)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"ChatGroupUserId": betRecord.ChatGroupUserId,
		}).Error("未查询到该用户信息")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"ChatGroupUserId": betRecord.ChatGroupUserId,
			"err":             err,
		}).Error("查询该用户信息异常")
		return
	}

	// 查找该用户所属群
	ChatGroup, err := model.QueryChatGroupById(db, chatGroupUser.ChatGroupId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"ChatGroupId": chatGroupUser.ChatGroupId,
		}).Error("未查询到群信息")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"ChatGroupId": chatGroupUser.ChatGroupId,
			"err":         err,
		}).Error("查询群信息异常")
		return
	}

	// 获取用户对应的互斥锁
	userLockKey := fmt.Sprintf(ChatGroupUserLockKey, ChatGroup.TgChatGroupId, chatGroupUser.TgUserId)
	userLock := getUserLock(userLockKey)
	userLock.Lock()
	defer userLock.Unlock()

	tx := db.Begin()

	var betResultTypeName string
	if betRecord.BetType == lotteryRecord.SingleDouble ||
		betRecord.BetType == lotteryRecord.BigSmall {
		betRecord.BetResultAmount = fmt.Sprintf("+%.2f", betRecord.BetAmount*quickThereConfig.SimpleOdds)
		chatGroupUser.Balance += betRecord.BetAmount * quickThereConfig.SimpleOdds
		betResultType := 1
		betResultTypeName = "赢"
		betRecord.BetResultType = &betResultType
	} else if betRecord.BetType == enums.Triplet.Value && lotteryRecord.Triplet == 1 {
		betRecord.BetResultAmount = fmt.Sprintf("+%.2f", betRecord.BetAmount*quickThereConfig.SimpleOdds)
		chatGroupUser.Balance += betRecord.BetAmount * quickThereConfig.TripletOdds
		betResultType := 1
		betResultTypeName = "赢"
		betRecord.BetResultType = &betResultType
	} else {
		betRecord.BetResultAmount = fmt.Sprintf("-%.2f", betRecord.BetAmount)
		betResultType := 0
		betResultTypeName = "输"
		betRecord.BetResultType = &betResultType
	}

	result := tx.Save(&chatGroupUser)
	if result.Error != nil {
		logrus.WithField("err", result.Error).Error("更新用户余额异常")
		tx.Rollback()
		return
	}

	// 更新下注记录表
	betRecord.SettleStatus = 1
	betRecord.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
	result = tx.Save(&betRecord)
	if result.Error != nil {
		logrus.WithField("err", result.Error).Error("更新下注记录异常")
		tx.Rollback()
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		// 提交事务时出现异常，回滚事务
		tx.Rollback()
	}

	lotteryType, _ := enums.GetGameLotteryType(betRecord.BetType)

	// 消息提醒
	sendMsg := tgbotapi.NewMessage(chatGroupUser.TgUserId,
		fmt.Sprintf("您在【%s】第%s期下注%v积分猜【%s】,竞猜结果为【%s】,积分余额%.2f。",
			ChatGroup.TgChatGroupTitle,
			betRecord.IssueNumber,
			betRecord.BetAmount,
			lotteryType.Name,
			betResultTypeName,
			chatGroupUser.Balance))
	_, err = sendMessage(bot, &sendMsg)
	blockedOrKicked(err, chatGroupUser.TgUserId)
	return
}
