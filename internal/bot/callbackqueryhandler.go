package bot

import (
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/gorm"
	"log"
	"strings"
	"telegram-dice-bot/internal/common"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
	"telegram-dice-bot/internal/utils"
)

// handleCallbackQuery 处理回调查询。
func handleCallbackQuery(bot *tgbotapi.BotAPI, callbackQuery *tgbotapi.CallbackQuery) {
	if callbackQuery.Message.Chat.IsPrivate() {
		if callbackQuery.Data == "main_menu" {
			mainMenuCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == "joined_group" {
			joinedGroupCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == "admin_group" {
			adminGroupCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == "add_admin_group" {
			addAdminGroupCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == "already_invited" {
			alreadyInvitedCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == "already_reload" {
			alreadyReloadCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, "chat_group_config?") {
			// 群配置
			chatGroupCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, "gameplay_type?") {
			// 群配置-游戏类型
			GameplayTypeCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, "update_gameplay_type?") {
			// 群配置-更新游戏类型
			updateGameplayTypeCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, "update_gameplay_status?") {
			// 群配置-更新游戏状态
			updateGameplayStatusCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, "update_game_draw_cycle?") {
			// 群配置-更新游戏开奖周期
			updateGameDrawCycleCallBack(bot, callbackQuery)
		}
	}
}

func updateGameDrawCycleCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	user := query.From

	// 查询当前群配置的游戏类型
	queryString := query.Data[strings.Index(query.Data, "update_game_draw_cycle?")+len("update_game_draw_cycle?"):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		log.Printf("queryData %v 内联键盘解析异常 ", query.Data)
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		log.Printf("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	sendMsg := tgbotapi.NewMessage(chatId, "请输入️要设置的开奖周期(1-60的整数)(单位:分钟)")

	// 设置当前机器人状态
	err = PrivateChatCacheAddRedis(user.ID, &common.BotPrivateChatCache{
		ChatStatus:  enums.WaitGameDrawCycle.Value,
		ChatGroupId: chatGroupId,
	})

	if err != nil {
		log.Printf("BotChatStatus 设置异常 TgUserID %v ChatStatus %s", user.ID, enums.WaitGameDrawCycle.Value)
		return
	}

	_, err = sendMessage(bot, &sendMsg)

	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func updateGameplayStatusCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID
	user := query.From

	// 查询使用的chatGroupId为内联键盘中的Data
	queryString := query.Data[strings.Index(query.Data, "update_gameplay_status?")+len("update_gameplay_status?"):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		log.Printf("queryData %v 内联键盘解析异常 ", query.Data)
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		log.Printf("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, user.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("chatGroupId %v userId %v 当前对话人非该群管理员 ", chatGroupId, user.ID)
		return
	}

	chatGroup, err := model.QueryChatGroupById(db, chatGroupId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("群TgChatId %v 该群未初始化过配置 ", chatGroupId)
		return
	} else if err != nil {
		log.Printf("群TgChatId %v 查找异常 %s", chatGroupId, err.Error())
		return
	}

	// 更新群配置-游戏状态
	chatGroupUpdate := &model.ChatGroup{
		Id: chatGroupId,
	}
	if chatGroup.GameplayStatus == enums.GameplayStatusON.Value {
		chatGroupUpdate.GameplayStatus = enums.GameplayStatusOFF.Value
		chatGroup.GameplayStatus = enums.GameplayStatusOFF.Value
		gameStop(chatGroup)
	} else {
		chatGroupUpdate.GameplayStatus = enums.GameplayStatusON.Value
		chatGroup.GameplayStatus = enums.GameplayStatusON.Value
		// 开启
		gameStart(bot, chatGroup)
	}
	err = chatGroupUpdate.UpdateChatGroupStatusById(db)
	if err != nil {
		log.Printf("更新群配置-游戏状态异常 err %s", err.Error())
		return
	}

	inlineKeyboardMarkup, err := buildChatGroupInlineKeyboardMarkup(query, chatGroup)

	if err != nil {
		log.Printf("组装群组配置内联键盘异常 err %s", err.Error())
		return
	}

	sendMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("点击修改【%s】相关配置:", chatGroup.TgChatGroupTitle))

	sendMsg.ReplyMarkup = inlineKeyboardMarkup
	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatID)
		return
	}
}

func updateGameplayTypeCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	user := query.From
	messageId := query.Message.MessageID

	// 查询当前群配置的游戏类型
	queryString := query.Data[strings.Index(query.Data, "update_gameplay_type?")+len("update_gameplay_type?"):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		log.Printf("queryData %v 内联键盘解析异常 ", query.Data)
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		log.Printf("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]
	gameplayType := callBackData["gameplayType"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, user.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("chatGroupId %v userId %v 当前对话人非该群管理员 ", chatGroupId, user.ID)
		return
	}

	// 更改配置
	err = model.UpdateChatGroupGameplayTypeById(db, &model.ChatGroup{
		Id:           chatGroupId,
		GameplayType: gameplayType,
	})

	if err != nil {
		log.Println("更新群配置异常", err)
		return
	}

	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, "请选择游戏类型:")

	inlineKeyboardRows, err := buildGameplayTypeInlineKeyboardButton(chatGroupId)

	// 组装列表数据
	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		inlineKeyboardRows...,
	)

	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup

	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}

}

func GameplayTypeCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	user := query.From
	messageId := query.Message.MessageID

	// 查询当前群配置的游戏类型
	queryString := query.Data[strings.Index(query.Data, "gameplay_type?")+len("gameplay_type?"):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		log.Printf("queryData %v 内联键盘解析异常 ", query.Data)
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		log.Printf("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, user.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("chatGroupId %v userId %v 当前对话人非该群管理员 ", chatGroupId, user.ID)
		return
	}

	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, "请选择游戏类型:")

	inlineKeyboardRows, err := buildGameplayTypeInlineKeyboardButton(chatGroupId)

	if err != nil {
		log.Printf("群ChatGroupId %v 组装游戏类型内联键盘异常 %s ", chatGroupId, err.Error())
		return
	}

	// 组装列表数据
	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		inlineKeyboardRows...,
	)

	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup

	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func chatGroupCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID
	user := query.From

	// 查询使用的chatGroupId为内联键盘中的Data
	queryString := query.Data[strings.Index(query.Data, "chat_group_config?")+len("chat_group_config?"):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		log.Printf("queryData %v 内联键盘解析异常 ", query.Data)
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		log.Printf("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, user.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("chatGroupId %v userId %v 当前对话人非该群管理员 ", chatGroupId, user.ID)
		return
	}

	chatGroup, err := model.QueryChatGroupById(db, chatGroupId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("群TgChatId %v 该群未初始化过配置 ", chatGroupId)
		return
	} else if err != nil {
		log.Printf("群TgChatId %v 查找异常 %s", chatGroupId, err.Error())
		return
	}

	inlineKeyboardMarkup, err := buildChatGroupInlineKeyboardMarkup(query, chatGroup)

	if err != nil {
		log.Printf("组装群组配置内联键盘异常 err %s", err.Error())
		return
	}

	sendMsg := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("点击修改【%s】相关配置:", chatGroup.TgChatGroupTitle))

	sendMsg.ReplyMarkup = inlineKeyboardMarkup
	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatID)
		return
	}
}

func mainMenuCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	userId := query.From.ID
	messageId := query.Message.MessageID

	member, err := getChatMember(bot, chatId, userId)

	if err != nil {
		log.Println("获取聊天成员异常", err)
		return
	}

	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, fmt.Sprintf("你好,%s!", member.User.FirstName))
	sendMsg.ReplyMarkup = buildDefaultInlineKeyboardMarkup(bot)

	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func addAdminGroupCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	messageId := query.Message.MessageID
	// 邀请bot进群链接
	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, "点击下方按钮将机器人添加至群组并设置为管理员!")
	inviteBotLink := fmt.Sprintf("https://t.me/%s?startgroup=true", bot.Self.UserName)

	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("➕点击添加➕", inviteBotLink),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅好了✅", "already_invited"),
		),
	)
	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup
	_, err := sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func adminGroupCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	user := query.From
	chatId := query.Message.Chat.ID

	sendMsg, err := buildAddAdminGroupMsg(query)
	if err != nil {
		log.Printf("TgUserId %v 查询管理群列表异常 %s ", user.ID, err.Error())
		return
	}

	_, err = sendMessage(bot, sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func joinedGroupCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {

}

func alreadyReloadCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	messageId := query.Message.MessageID

	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, "接下来就可以使用啦!")
	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅好了✅", "admin_group"),
		),
	)

	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup
	_, err := sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func alreadyInvitedCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	messageId := query.Message.MessageID
	// 邀请bot进群链接
	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, "请在【群组】中发送 /reload 重新加载!")
	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅好了✅", "already_reload"),
		),
	)
	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup
	_, err := sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}
