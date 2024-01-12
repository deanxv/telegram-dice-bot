package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"os"
	"strconv"
	"strings"
	"telegram-dice-bot/internal/common"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
)

var whiteList = os.Getenv(WhiteList)

// 处理私有Command消息
func handlePrivateCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	switch message.Command() {
	case "start", "menu":
		handlePrivateStartCommand(bot, message)
	}
}

func handlePrivateStartCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatId := message.Chat.ID
	fromUser := message.From

	// 白名单
	if whiteList != "" && !strings.Contains(whiteList, fmt.Sprintf("@%s", fromUser.UserName)) {
		return
	}

	member, err := getChatMember(bot, chatId, fromUser.ID)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatId":     chatId,
			"fromUserId": fromUser.ID,
			"err":        err,
		}).Error("获取聊天成员异常")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf("您好,%s!", member.User.FirstName))
	sendMsg.ReplyMarkup = buildDefaultInlineKeyboardMarkup(bot)

	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

// 处理私有Text消息
func handlePrivateText(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userId := message.From.ID

	// 检查当前botChatStatus
	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, userId)
	result := redisDB.Get(redisDB.Context(), redisKey)
	if errors.Is(result.Err(), redis.Nil) || result == nil {
		//logrus.WithFields(logrus.Fields{
		//	"redisKey": redisKey,
		//}).Info("当前机器人暂无对话状态")
		return
	} else if result.Err() != nil {
		logrus.WithFields(logrus.Fields{
			"redisKey": redisKey,
			"err":      result.Err(),
		}).Error("获取机器人对话状态缓存异常")
		return
	} else {
		var botPrivateChatCache common.BotPrivateChatCache
		botPrivateChatCacheString, _ := result.Result()
		err := json.Unmarshal([]byte(botPrivateChatCacheString), &botPrivateChatCache)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"botPrivateChatCacheString": botPrivateChatCacheString,
				"err":                       result.Err(),
			}).Error("BotPrivateChatCache 解析异常")
			return
		}
		if enums.WaitGameDrawCycle.Value == botPrivateChatCache.ChatStatus {
			// 开奖周期设置
			updateGameDrawCycle(bot, message, &botPrivateChatCache)
		} else if enums.WaitQuickThereSimpleOdds.Value == botPrivateChatCache.ChatStatus {
			// 快三简易倍率设置
			updateQuickThereSimpleOdds(bot, message, &botPrivateChatCache)
		} else if enums.WaitQuickThereTripletOdds.Value == botPrivateChatCache.ChatStatus {
			// 快三豹子倍率设置
			updateQuickThereTripletOdds(bot, message, &botPrivateChatCache)
		} else if enums.WaitQueryUser.Value == botPrivateChatCache.ChatStatus {
			// 查询用户信息
			queryUser(bot, message, &botPrivateChatCache)
		} else if enums.WaitUpdateUserBalance.Value == botPrivateChatCache.ChatStatus {
			// 修改用户余额
			updateUserBalance(bot, message, &botPrivateChatCache)
		} else if enums.WaitTransferBalance.Value == botPrivateChatCache.ChatStatus {
			// 转让用户积分
			transferBalance(bot, message, &botPrivateChatCache)
		}

	}
}

func transferBalance(bot *tgbotapi.BotAPI, message *tgbotapi.Message, botPrivateChatCache *common.BotPrivateChatCache) {
	text := message.Text
	chatId := message.Chat.ID
	fromUser := message.From

	var operator string
	var index int

	// 检查字符串中的运算符
	if strings.Contains(text, "+") {
		operator = "+"
		index = strings.Index(text, "+")
	} else {
		logrus.WithField("text", text).Warn("当前对话状态为[转让积分] 用户输入内容非指定格式")
		return
	}

	var sendMsg tgbotapi.MessageConfig

	// 分割字符串
	chatGroupUserId := text[:index]
	updateBalanceStr := text[index+1:]
	updateBalance, err := strconv.ParseFloat(updateBalanceStr, 64)
	if err != nil {
		logrus.WithField("updateBalanceStr", updateBalanceStr).Error("updateBalance转int异常")
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("积分存在非法字符:%s", updateBalanceStr))
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	} else if updateBalance <= 0 || updateBalance > 9999999999 {
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("积分不合法,可转让积分范围[0-9999999999]"))
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	}

	// 查询被转让用户信息
	chatGroupUser := &model.ChatGroupUser{
		Id:          chatGroupUserId,
		ChatGroupId: botPrivateChatCache.ChatGroupId,
	}
	groupUser, err := chatGroupUser.QueryByIdAndChatGroupId(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupUserId": chatGroupUserId,
			"ChatGroupId":     botPrivateChatCache.ChatGroupId,
			"err":             err,
		}).Warn("查询用户信息异常")
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("当前群组内未查询到该用户,用户Id:%s", chatGroupUserId))
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	}

	// 查询被转让用户群信息
	chatGroup := &model.ChatGroup{
		Id: groupUser.ChatGroupId,
	}
	group, err := model.QueryChatGroupById(db, chatGroup.Id)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"ChatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("查找群配置信息异常")
		return
	}

	// 获取被转让用户对应的互斥锁
	userLockKey := fmt.Sprintf(ChatGroupUserLockKey, group.TgChatGroupId, groupUser.TgUserId)
	userLock := getUserLock(userLockKey)
	userLock.Lock()
	defer userLock.Unlock()

	// 查询发起转让用户信息
	sendChatGroupUser := &model.ChatGroupUser{
		TgUserId:    fromUser.ID,
		ChatGroupId: chatGroup.Id,
	}
	sendGroupUser, err := sendChatGroupUser.QueryByTgUserIdAndChatGroupId(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    fromUser.ID,
			"ChatGroupId": chatGroup.Id,
		}).Warn("未查询到用户信息")
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("当前群组内未查询到该用户,用户Id:%s", chatGroupUserId))
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	}

	if sendGroupUser.Id == groupUser.Id {
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("不可对自己转让积分!"))
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	}

	// 获取发起转让用户对应的互斥锁
	sendUserLockKey := fmt.Sprintf(ChatGroupUserLockKey, chatGroup.TgChatGroupId, sendGroupUser.TgUserId)
	sendUserLock := getUserLock(sendUserLockKey)
	sendUserLock.Lock()
	defer sendUserLock.Unlock()

	tx := db.Begin()

	// 重新查询用户信息
	groupUser, _ = groupUser.QueryById(tx)
	sendGroupUser, _ = sendGroupUser.QueryById(tx)

	// 根据运算符执行特定逻辑
	switch operator {
	case "+":
		if sendGroupUser.Balance < updateBalance {
			sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("积分余额不足,您的积分余额为%.2f。", sendGroupUser.Balance))
		} else {
			groupUser.Balance += updateBalance
			tx.Save(&groupUser)
			sendGroupUser.Balance -= updateBalance
			tx.Save(&sendGroupUser)
			sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("转让成功!【%s】中的用户【@%s】增加%.2f积分,您的积分余额为%.2f。", group.TgChatGroupTitle, groupUser.Username, updateBalance, sendGroupUser.Balance))
			// 提交事务
			if err := tx.Commit().Error; err != nil {
				// 提交事务时出现异常，回滚事务
				tx.Rollback()
				return
			}
		}
	}

	_, err = sendMessage(bot, &sendMsg)
	// 删除bot与当前对话人的cache
	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, fromUser.ID)
	redisDB.Del(redisDB.Context(), redisKey)
	blockedOrKicked(err, chatId)

	// 发送被转让用户的提示消息
	sendNotifyMsg := tgbotapi.NewMessage(groupUser.TgUserId, fmt.Sprintf("【%s】您收到用户【@%s】转让的%.2f积分,您的积分余额为%.2f。", group.TgChatGroupTitle, sendGroupUser.Username, updateBalance, groupUser.Balance))
	_, err = sendMessage(bot, &sendNotifyMsg)
	blockedOrKicked(err, groupUser.TgUserId)
	return
}

func updateQuickThereTripletOdds(bot *tgbotapi.BotAPI, message *tgbotapi.Message, botPrivateChatCache *common.BotPrivateChatCache) {
	text := message.Text
	tgUserId := message.From.ID
	chatId := message.Chat.ID
	messageId := message.MessageID

	// 校验当前对话人是否为该群管理员
	err := checkGroupAdmin(botPrivateChatCache.ChatGroupId, tgUserId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": botPrivateChatCache.ChatGroupId,
			"tgUserId":    tgUserId,
		}).Error("当前对话人非该群管理员")
		return
	}

	// 将字符串转换为float64
	tripletOdds, err := strconv.ParseFloat(text, 64)
	if err != nil {
		fmt.Println(err)
		return
	}

	quickThereConfig := &model.QuickThereConfig{
		ChatGroupId: botPrivateChatCache.ChatGroupId,
		TripletOdds: tripletOdds,
	}

	err = quickThereConfig.UpdateTripletOddsByChatGroupId(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"ChatGroupId": botPrivateChatCache.ChatGroupId,
			"TripletOdds": tripletOdds,
		}).Error("设置快三豹子倍率异常")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf("设置成功!\n【经典快三】豹子倍率已设置为%.2f倍!", tripletOdds))
	sendMsg.ReplyToMessageID = messageId

	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
	// 删除bot与当前对话人的cache
	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, tgUserId)
	redisDB.Del(redisDB.Context(), redisKey)
}

func updateQuickThereSimpleOdds(bot *tgbotapi.BotAPI, message *tgbotapi.Message, botPrivateChatCache *common.BotPrivateChatCache) {
	text := message.Text
	tgUserId := message.From.ID
	chatId := message.Chat.ID
	messageId := message.MessageID

	// 校验当前对话人是否为该群管理员
	err := checkGroupAdmin(botPrivateChatCache.ChatGroupId, tgUserId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": botPrivateChatCache.ChatGroupId,
			"tgUserId":    tgUserId,
		}).Error("当前对话人非该群管理员")
		return
	}

	// 将字符串转换为float64
	simpleOdds, err := strconv.ParseFloat(text, 64)
	if err != nil {
		fmt.Println(err)
		return
	}

	quickThereConfig := &model.QuickThereConfig{
		ChatGroupId: botPrivateChatCache.ChatGroupId,
		SimpleOdds:  simpleOdds,
	}

	err = quickThereConfig.UpdateSimpleOddsByChatGroupId(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"ChatGroupId": botPrivateChatCache.ChatGroupId,
			"SimpleOdds":  simpleOdds,
		}).Error("设置快三简易倍率异常")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf("设置成功!\n【经典快三】简易倍率已设置为%.2f倍!", simpleOdds))
	sendMsg.ReplyToMessageID = messageId

	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
	// 删除bot与当前对话人的cache
	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, tgUserId)
	redisDB.Del(redisDB.Context(), redisKey)
}

func updateUserBalance(bot *tgbotapi.BotAPI, message *tgbotapi.Message, botPrivateChatCache *common.BotPrivateChatCache) {
	tgUserId := message.From.ID
	text := message.Text
	chatId := message.Chat.ID

	// 校验当前对话人是否为该群管理员
	err := checkGroupAdmin(botPrivateChatCache.ChatGroupId, tgUserId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": botPrivateChatCache.ChatGroupId,
			"tgUserId":    tgUserId,
		}).Error("当前对话人非该群管理员")
		return
	}

	var operator string
	var index int

	// 检查字符串中的运算符
	if strings.Contains(text, "+") {
		operator = "+"
		index = strings.Index(text, "+")
	} else if strings.Contains(text, "-") {
		operator = "-"
		index = strings.Index(text, "-")
	} else if strings.Contains(text, "=") {
		operator = "="
		index = strings.Index(text, "=")
	} else {
		logrus.WithFields(logrus.Fields{
			"text": text,
		}).Warn("未知的运算符")
		return
	}

	var sendMsg tgbotapi.MessageConfig

	// 分割字符串
	chatGroupUserId := text[:index]
	updateBalanceStr := text[index+1:]
	updateBalance, err := strconv.ParseFloat(updateBalanceStr, 64)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"updateBalanceStr": updateBalanceStr,
			"err":              err,
		}).Error("updateBalance转int异常")
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("积分存在非法字符:%s", updateBalanceStr))
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	} else if updateBalance <= 0 || updateBalance > 9999999999 {
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("积分不合法,可调整积分范围[0-9999999999]"))
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	}

	// 查询用户信息
	chatGroupUser := &model.ChatGroupUser{
		Id:          chatGroupUserId,
		ChatGroupId: botPrivateChatCache.ChatGroupId,
	}
	groupUser, err := chatGroupUser.QueryByIdAndChatGroupId(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupUserId": chatGroupUserId,
			"ChatGroupId":     botPrivateChatCache.ChatGroupId,
			"err":             err,
		}).Warn("查询用户信息异常")
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("当前群组内未查询到该用户,用户Id:%s", chatGroupUserId))
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	}

	// 查询用户群信息
	chatGroup := &model.ChatGroup{
		Id: groupUser.ChatGroupId,
	}
	group, err := model.QueryChatGroupById(db, chatGroup.Id)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"ChatGroupId": groupUser.ChatGroupId,
			"err":         err,
		}).Error("群配置信息查询异常")
		return
	}

	// 获取用户对应的互斥锁
	userLockKey := fmt.Sprintf(ChatGroupUserLockKey, group.TgChatGroupId, groupUser.TgUserId)
	userLock := getUserLock(userLockKey)
	userLock.Lock()
	defer userLock.Unlock()

	// 重新查询用户信息
	groupUser, _ = chatGroupUser.QueryById(db)
	var sendNotifyMsg tgbotapi.MessageConfig

	// 根据运算符执行特定逻辑
	switch operator {
	case "+":
		groupUser.Balance += updateBalance
		db.Save(&groupUser)
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("已为【%s】中的用户【@%s】增加%.2f积分,积分余额为%.2f。", group.TgChatGroupTitle, groupUser.Username, updateBalance, groupUser.Balance))
		sendNotifyMsg = tgbotapi.NewMessage(groupUser.TgUserId, fmt.Sprintf("【%s】管理员为您增加了%.2f积分,您的积分余额为%.2f。", group.TgChatGroupTitle, updateBalance, groupUser.Balance))
	case "-":
		if groupUser.Balance < updateBalance {
			sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("【%s】中的用户【@%s】积分余额为%.2f,小于您想扣除的积分，请留点积分吧。", group.TgChatGroupTitle, groupUser.Username, groupUser.Balance))
			_, err = sendMessage(bot, &sendMsg)
			blockedOrKicked(err, chatId)
			return
		} else {
			groupUser.Balance -= updateBalance
			db.Save(&groupUser)
			sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("已为【%s】中的用户【@%s】扣除%.2f积分,积分余额为%.2f。", group.TgChatGroupTitle, groupUser.Username, updateBalance, groupUser.Balance))
			sendNotifyMsg = tgbotapi.NewMessage(groupUser.TgUserId, fmt.Sprintf("【%s】管理员扣除了您%.2f积分,您的积分余额为%.2f。", group.TgChatGroupTitle, updateBalance, groupUser.Balance))
		}
	case "=":
		groupUser.Balance = updateBalance
		db.Save(&groupUser)
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("已将【%s】中的用户【@%s】积分修改为%.2f。", group.TgChatGroupTitle, groupUser.Username, groupUser.Balance))
		sendNotifyMsg = tgbotapi.NewMessage(groupUser.TgUserId, fmt.Sprintf("【%s】管理员将您的积分修改为%.2f。", group.TgChatGroupTitle, groupUser.Balance))
	}

	_, err = sendMessage(bot, &sendMsg)
	blockedOrKicked(err, chatId)

	_, err = sendMessage(bot, &sendNotifyMsg)
	blockedOrKicked(err, groupUser.TgUserId)

	// 删除bot与当前对话人的cache
	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, tgUserId)
	redisDB.Del(redisDB.Context(), redisKey)
	return

}

func queryUser(bot *tgbotapi.BotAPI, message *tgbotapi.Message, botPrivateChatCache *common.BotPrivateChatCache) {
	text := message.Text
	tgUserId := message.From.ID
	chatId := message.Chat.ID
	messageId := message.MessageID

	// 校验当前对话人是否为该群管理员
	err := checkGroupAdmin(botPrivateChatCache.ChatGroupId, tgUserId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": botPrivateChatCache.ChatGroupId,
			"tgUserId":    tgUserId,
		}).Error("当前对话人非该群管理员")
		return
	}

	chatGroupUser := &model.ChatGroupUser{
		ChatGroupId: botPrivateChatCache.ChatGroupId,
		Username:    text[1:],
	}

	groupUser, err := chatGroupUser.QueryByUsernameAndChatGroupId(db)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有找到记录
		msgConfig := tgbotapi.NewMessage(chatId, "未查询到该用户，未注册或用户名已更改!")
		msgConfig.ReplyToMessageID = messageId
		_, err := sendMessage(bot, &msgConfig)
		blockedOrKicked(err, chatId)
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"ChatGroupId": chatGroupUser.ChatGroupId,
			"Username":    chatGroupUser.Username,
		}).Error("群id+用户名查找群成员异常")
	} else {
		// 查询到记录
		msgConfig := tgbotapi.NewMessage(chatId, fmt.Sprintf("用户ID:%v\n用户名称:%s\n积分余额:%.2f", groupUser.Id, groupUser.Username, groupUser.Balance))
		msgConfig.ReplyToMessageID = messageId
		_, err := sendMessage(bot, &msgConfig)
		blockedOrKicked(err, chatId)
		// 删除bot与当前对话人的cache
		redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, tgUserId)
		redisDB.Del(redisDB.Context(), redisKey)
		return
	}
}

func updateGameDrawCycle(bot *tgbotapi.BotAPI, message *tgbotapi.Message, botPrivateChatCache *common.BotPrivateChatCache) {
	text := message.Text
	tgUserId := message.From.ID
	chatId := message.Chat.ID
	messageId := message.MessageID

	// 校验当前对话人是否为该群管理员
	err := checkGroupAdmin(botPrivateChatCache.ChatGroupId, tgUserId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": botPrivateChatCache.ChatGroupId,
			"tgUserId":    tgUserId,
		}).Error("当前对话人非该群管理员")
		return
	}

	drawCycle, err := strconv.Atoi(text)
	if err != nil {
		logrus.WithField("err", err).Error("drawCycle转int异常")
		return
	}

	if drawCycle <= 0 || drawCycle > 60 {
		sendMsg := tgbotapi.NewMessage(chatId, "开奖周期必须大于0分钟小于60分钟哦!")
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatId)
		return
	}

	chatGroup := &model.ChatGroup{
		Id:            botPrivateChatCache.ChatGroupId,
		GameDrawCycle: drawCycle,
	}

	err = chatGroup.UpdateGameDrawCycleById(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId":   botPrivateChatCache.ChatGroupId,
			"GameDrawCycle": drawCycle,
		}).Error("设置开奖周期异常")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf("设置成功!当前群组开奖周期为%v分钟,重新开启游戏后生效哦!", drawCycle))
	sendMsg.ReplyToMessageID = messageId

	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
	// 删除bot与当前对话人的cache
	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, tgUserId)
	redisDB.Del(redisDB.Context(), redisKey)
}
