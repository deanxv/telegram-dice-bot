package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/gorm"
	"log"
	"strconv"
	"strings"
	"telegram-dice-bot/internal/common"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
)

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
	member, err := getChatMember(bot, chatId, fromUser.ID)

	if err != nil {
		log.Println("获取聊天成员异常", err)
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf("你好,%s!", member.User.FirstName))
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
		//log.Printf("键 %s 不存在 [当前机器人暂无对话状态]", redisKey)
		return
	} else if result.Err() != nil {
		log.Println("获取值时发生错误: [当前机器人对话状态查询异常]", result.Err())
		return
	} else {
		var botPrivateChatCache common.BotPrivateChatCache
		botPrivateChatCacheString, _ := result.Result()
		err := json.Unmarshal([]byte(botPrivateChatCacheString), &botPrivateChatCache)
		if err != nil {
			log.Printf("BotPrivateChatCache 解析异常 botPrivateChatCacheString %s err %s", botPrivateChatCacheString, result.Err())
			return
		}
		if enums.WaitGameDrawCycle.Value == botPrivateChatCache.ChatStatus {
			// 开奖周期设置
			updateGameDrawCycle(bot, message, &botPrivateChatCache)
		} else if enums.WaitQueryUser.Value == botPrivateChatCache.ChatStatus {
			// 查询用户信息
			queryUser(bot, message, &botPrivateChatCache)
		} else if enums.WaitUpdateUserBalance.Value == botPrivateChatCache.ChatStatus {
			// 查询用户信息
			updateUserBalance(bot, message, &botPrivateChatCache)
		}

	}
}

func updateUserBalance(bot *tgbotapi.BotAPI, message *tgbotapi.Message, botPrivateChatCache *common.BotPrivateChatCache) {
	tgUserId := message.From.ID
	text := message.Text
	chatId := message.Chat.ID

	// 校验当前对话人是否为该群管理员
	err := checkGroupAdmin(botPrivateChatCache.ChatGroupId, tgUserId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("chatGroupId %v userId %v 当前对话人非该群管理员 ", botPrivateChatCache.ChatGroupId, tgUserId)
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
		log.Println("未知的运算符")
		return
	}

	var sendMsg tgbotapi.MessageConfig

	// 分割字符串
	chatGroupUserId := text[:index]
	updateBalanceStr := text[index+1:]
	updateBalance, err := strconv.ParseFloat(updateBalanceStr, 64)
	if err != nil {
		log.Printf("updateBalance转int异常 err %s", err.Error())
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("积分存在非法字符:%s", updateBalanceStr))
		return
	}

	// 查询用户信息
	chatGroupUser := &model.ChatGroupUser{
		Id: chatGroupUserId,
	}
	groupUser, err := chatGroupUser.QueryById(db)
	if err != nil {
		log.Println("查询用户信息异常:", err)
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("未查询到该用户,用户Id:%s", chatGroupUserId))
		return
	}

	// 查询用户群信息
	chatGroup := &model.ChatGroup{
		Id: groupUser.ChatGroupId,
	}
	group, err := model.QueryChatGroupById(db, chatGroup.Id)
	if err != nil {
		log.Printf("群TgChatId %v 查找异常 %s", chatGroup.Id, err.Error())
		return
	}

	// 获取用户对应的互斥锁
	userLockKey := fmt.Sprintf(ChatGroupUserLockKey, group.TgChatGroupId, groupUser.TgUserId)
	userLock := getUserLock(userLockKey)
	userLock.Lock()
	defer userLock.Unlock()

	// 重新查询用户信息
	groupUser, _ = chatGroupUser.QueryById(db)

	// 根据运算符执行特定逻辑
	switch operator {
	case "+":
		groupUser.Balance += updateBalance
		db.Save(&groupUser)
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("已为用户【%s】增加%.2f积分,积分余额为%.2f。", groupUser.Username, updateBalance, groupUser.Balance))
	case "-":
		if groupUser.Balance < updateBalance {
			sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("用户【%s】积分余额为%.2f,小于你想扣除的积分，请留点积分吧。", groupUser.Username, groupUser.Balance))
			_, err = sendMessage(bot, &sendMsg)
			blockedOrKicked(err, chatId)
			return
		} else {
			groupUser.Balance -= updateBalance
			db.Save(&groupUser)
			sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("已为用户【%s】减少%.2f积分,积分余额为%.2f。", groupUser.Username, updateBalance, groupUser.Balance))
		}
	case "=":
		groupUser.Balance = updateBalance
		db.Save(&groupUser)
		sendMsg = tgbotapi.NewMessage(chatId, fmt.Sprintf("已将用户【%s】积分修改为%.2f。", groupUser.Username, groupUser.Balance))
	}

	_, err = sendMessage(bot, &sendMsg)
	// 删除bot与当前对话人的cache
	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, groupUser.TgUserId)
	redisDB.Del(redisDB.Context(), redisKey)
	blockedOrKicked(err, chatId)
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
		log.Printf("chatGroupId %v userId %v 当前对话人非该群管理员 ", botPrivateChatCache.ChatGroupId, tgUserId)
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
		log.Println("查询异常:", err)
	} else {
		// 查询到记录
		msgConfig := tgbotapi.NewMessage(chatId, fmt.Sprintf("用户ID:%v\n用户名:%s\n余额:%.2f", groupUser.Id, groupUser.Username, groupUser.Balance))
		msgConfig.ReplyToMessageID = messageId
		_, err := sendMessage(bot, &msgConfig)
		blockedOrKicked(err, chatId)
		// 删除bot与当前对话人的cache
		redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, groupUser.TgUserId)
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
		log.Printf("chatGroupId %v userId %v 当前对话人非该群管理员 ", botPrivateChatCache.ChatGroupId, tgUserId)
		return
	}

	drawCycle, err := strconv.Atoi(text)
	if err != nil {
		log.Printf("drawCycle转int异常 err %s", err.Error())
		return
	}

	if drawCycle <= 0 || drawCycle > 60 {
		return
	}

	chatGroup := &model.ChatGroup{
		Id:            botPrivateChatCache.ChatGroupId,
		GameDrawCycle: drawCycle,
	}

	err = chatGroup.UpdateGameDrawCycleById(db)
	if err != nil {
		log.Printf("设置开奖周期异常 %s", err.Error())
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
