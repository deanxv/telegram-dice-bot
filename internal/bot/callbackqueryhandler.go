package bot

import (
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"strings"
	"telegram-dice-bot/internal/common"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
	"telegram-dice-bot/internal/utils"
	"time"
)

// handleCallbackQuery 处理回调查询。
func handleCallbackQuery(bot *tgbotapi.BotAPI, callbackQuery *tgbotapi.CallbackQuery) {
	if callbackQuery.Message.Chat.IsPrivate() {
		if callbackQuery.Data == enums.CallbackMainMenu.Value {
			mainMenuCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == enums.CallbackJoinedGroup.Value {
			joinedGroupCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == enums.CallbackAdminGroup.Value {
			adminGroupCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == enums.CallbackAddAdminGroup.Value {
			addAdminGroupCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == enums.CallbackAlreadyInvited.Value {
			alreadyInvitedCallBack(bot, callbackQuery)
		} else if callbackQuery.Data == enums.CallbackAlreadyReload.Value {
			alreadyReloadCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackChatGroupInfo.Value) {
			// 群详情信息
			chatGroupInfoCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackTransferBalance.Value) {
			// 转让积分
			transferBalanceCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackExitGroup.Value) {
			// 退群删除
			exitGroupCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackChatGroupConfig.Value) {
			// 群配置
			chatGroupConfigCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackGameplayType.Value) {
			// 群配置-游戏类型
			gameplayTypeCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackUpdateGameplayType.Value) {
			// 群配置-更新游戏类型
			updateGameplayTypeCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackUpdateQuickThereSimpleOdds.Value) {
			// 群配置-更新快三-简易赔率
			updateQuickThereSimpleOddsCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackUpdateQuickThereTripletOdds.Value) {
			// 群配置-更新快三-豹子赔率
			updateQuickThereTripletOddsCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackUpdateGameplayStatus.Value) {
			// 群配置-更新游戏状态
			updateGameplayStatusCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackUpdateGameDrawCycle.Value) {
			// 群配置-更新游戏开奖周期
			updateGameDrawCycleCallBack(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackQueryChatGroupUser.Value) {
			// 查询用户信息
			queryChatGroupUser(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackUpdateChatGroupUserBalance.Value) {
			// 修改用户积分
			updateChatGroupUserBalance(bot, callbackQuery)
		} else if strings.HasPrefix(callbackQuery.Data, enums.CallbackAdminExitGroup.Value) {
			// 管理员退群
			exitAdminGroupCallBack(bot, callbackQuery)
		}
	} else if callbackQuery.Message.Chat.IsGroup() || callbackQuery.Message.Chat.IsSuperGroup() {
		if callbackQuery.Data == enums.CallbackLotteryHistory.Value {
			// 群内联键盘 查看开奖历史
			lotteryHistoryCallBack(bot, callbackQuery)
		}
	}
}

func exitAdminGroupCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	fromUser := query.From
	fromChatId := query.Message.Chat.ID

	// 查询使用的chatGroupId为内联键盘中的Data
	queryString := query.Data[strings.Index(query.Data, enums.CallbackExitGroup.Value)+len(enums.CallbackExitGroup.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithField("queryData", query.Data).Error("内联键盘解析异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 查询该群信息
	chatGroup, err := model.QueryChatGroupById(db, chatGroupId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithField("chatGroupId", chatGroupId).Error("群未初始化过配置")
		return
	} else if err != nil {
		logrus.WithField("chatGroupId", chatGroupId).Error("群配置查找异常")
		return
	}

	// 删除该用户
	chatGroupUserQuery := &model.ChatGroupAdmin{
		AdminTgUserId: fromUser.ID,
		ChatGroupId:   chatGroup.Id,
	}

	chatGroupUserQuery.DeleteByChatGroupIdAndAdminTgUserId(db)

	// 更新上条消息
	sendMsg, err := buildAdminGroupMsg(query)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId": fromUser.ID,
			"err":        err,
		}).Info("查询管理的群列表异常")
		return
	}

	_, err = sendMessage(bot, sendMsg)
	blockedOrKicked(err, fromChatId)

	// 发送提示消息
	msgConfig := tgbotapi.NewMessage(fromChatId, fmt.Sprintf("删除【%s】信息成功!", chatGroup.TgChatGroupTitle))
	_, err = sendMessage(bot, &msgConfig)
	blockedOrKicked(err, fromChatId)
}

func transferBalanceCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	fromChatId := query.Message.Chat.ID
	fromUser := query.From

	queryString := query.Data[strings.Index(query.Data, enums.CallbackTransferBalance.Value)+len(enums.CallbackTransferBalance.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	sendMsg := tgbotapi.NewMessage(fromChatId, "请按照以下格式转让用户积分:\n"+
		"[用户Id]+[积分] 例子: 10086+100")

	// 设置当前机器人状态
	err = PrivateChatCacheAddRedis(fromUser.ID, &common.BotPrivateChatCache{
		ChatStatus:  enums.WaitTransferBalance.Value,
		ChatGroupId: chatGroupId,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId":  fromUser.ID,
			"ChatStatus":  enums.WaitTransferBalance.Value,
			"ChatGroupId": chatGroupId,
		}).Error("BotChatStatus 设置异常")
		return
	}

	_, err = sendMessage(bot, &sendMsg)

	if err != nil {
		blockedOrKicked(err, fromChatId)
		return
	}
}

func exitGroupCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	fromUser := query.From
	fromChatId := query.Message.Chat.ID

	// 查询使用的chatGroupId为内联键盘中的Data
	queryString := query.Data[strings.Index(query.Data, enums.CallbackExitGroup.Value)+len(enums.CallbackExitGroup.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 查询该群信息
	chatGroup, err := model.QueryChatGroupById(db, chatGroupId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
		}).Error("群未初始化过配置")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"err":         err,
		}).Error("群配置信息查询异常")
		return
	}

	// 查询该用户
	chatGroupUserQuery := &model.ChatGroupUser{
		TgUserId:    fromUser.ID,
		ChatGroupId: chatGroup.Id,
	}

	chatGroupUser, err := chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(db)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    fromUser.ID,
			"ChatGroupId": chatGroup.Id,
		}).Warn("该用户未注册")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    fromUser.ID,
			"ChatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("用户查询异常")
		return
	}

	// 更新该用户状态为离开
	chatGroupUser.IsLeft = 1
	result := db.Save(&chatGroupUser)

	if result.Error != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Error("保存用户信息异常")
		return
	}

	// 更新上条消息
	sendMsg, err := buildJoinedGroupMsg(query)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Error("组装加入的群列表异常")
		return
	}

	_, err = sendMessage(bot, sendMsg)
	blockedOrKicked(err, fromChatId)

	// 发送提示消息
	msgConfig := tgbotapi.NewMessage(fromChatId, fmt.Sprintf("删除【%s】信息成功!", chatGroup.TgChatGroupTitle))
	_, err = sendMessage(bot, &msgConfig)
	blockedOrKicked(err, fromChatId)
}

func chatGroupInfoCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	fromUser := query.From
	fromChatId := query.Message.Chat.ID
	messageId := query.Message.MessageID

	// 查询使用的chatGroupId为内联键盘中的Data
	queryString := query.Data[strings.Index(query.Data, enums.CallbackChatGroupInfo.Value)+len(enums.CallbackChatGroupInfo.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 查询该群信息
	chatGroup, err := model.QueryChatGroupById(db, chatGroupId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
		}).Error("未查询到群配置")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"err":         err,
		}).Error("群配置查询异常")
		return
	}

	// 查询用户在该群的信息
	chatGroupUserQuery := &model.ChatGroupUser{
		TgUserId:    fromUser.ID,
		ChatGroupId: chatGroup.Id,
	}

	chatGroupUser, err := chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(db)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    fromUser.ID,
			"ChatGroupId": chatGroup.Id,
		}).Warn("群组中不存在该用户")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    fromUser.ID,
			"ChatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("用户查询异常")
		return
	}

	sendMsg := tgbotapi.NewEditMessageText(fromChatId, messageId, fmt.Sprintf("您在【%s】中的信息:\n用户ID:%s\n积分余额:%.2f\n", chatGroup.TgChatGroupTitle, chatGroupUser.Id, chatGroupUser.Balance))

	// 重新生成内联键盘回调key
	callbackDataKey, err := ButtonCallBackDataAddRedis(map[string]string{
		"chatGroupId": chatGroup.Id,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("内联键盘回调参数存入redis异常")
		return
	}

	callbackDataQueryString := utils.MapToQueryString(map[string]string{
		"callbackDataKey": callbackDataKey,
	})

	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💸转让积分", fmt.Sprintf("%s%s", enums.CallbackTransferBalance.Value, callbackDataQueryString)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️返回", enums.CallbackJoinedGroup.Value),
			tgbotapi.NewInlineKeyboardButtonData("🚮我已退群", fmt.Sprintf("%s%s", enums.CallbackExitGroup.Value, callbackDataQueryString)),
		),
	)
	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup
	_, err = sendMessage(bot, &sendMsg)
	blockedOrKicked(err, fromChatId)
	return

}

func updateQuickThereTripletOddsCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	fromUser := query.From

	// 查询当前群配置的游戏类型
	queryString := query.Data[strings.Index(query.Data, enums.CallbackUpdateQuickThereTripletOdds.Value)+len(enums.CallbackUpdateQuickThereTripletOdds.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"fromUserID":  fromUser.ID,
		}).Error("当前对话人非该群管理员")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, "请输入️要设置的【经典快三】豹子倍率:")

	// 设置当前机器人状态
	err = PrivateChatCacheAddRedis(fromUser.ID, &common.BotPrivateChatCache{
		ChatStatus:  enums.WaitQuickThereTripletOdds.Value,
		ChatGroupId: chatGroupId,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId":  fromUser.ID,
			"ChatStatus":  enums.WaitQuickThereTripletOdds.Value,
			"ChatGroupId": chatGroupId,
			"err":         err,
		}).Error("BotChatStatus 设置异常")
		return
	}

	_, err = sendMessage(bot, &sendMsg)

	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func updateQuickThereSimpleOddsCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	fromUser := query.From

	// 查询当前群配置的游戏类型
	queryString := query.Data[strings.Index(query.Data, enums.CallbackUpdateQuickThereSimpleOdds.Value)+len(enums.CallbackUpdateQuickThereSimpleOdds.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"fromUserID":  fromUser.ID,
		}).Error("当前对话人非该群管理员")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, "请输入️要设置的【经典快三】简易倍率(大/小/单/双):")

	// 设置当前机器人状态
	err = PrivateChatCacheAddRedis(fromUser.ID, &common.BotPrivateChatCache{
		ChatStatus:  enums.WaitQuickThereSimpleOdds.Value,
		ChatGroupId: chatGroupId,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId":  fromUser.ID,
			"ChatStatus":  enums.WaitQuickThereSimpleOdds.Value,
			"ChatGroupId": chatGroupId,
			"err":         err,
		}).Error("BotChatStatus 设置异常")
		return
	}

	_, err = sendMessage(bot, &sendMsg)

	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func lotteryHistoryCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	tgChatGroupId := query.Message.Chat.ID

	// 查询该群历史开奖信息
	chatGroup, err := model.QueryChatGroupByTgChatId(db, tgChatGroupId)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"tgChatGroupId": tgChatGroupId,
			"err":           err,
		}).Error("群配置查询异常")
		return
	}

	sendMsg := tgbotapi.NewMessage(tgChatGroupId, "")

	lotteryRecord := &model.LotteryRecord{ChatGroupId: chatGroup.Id}
	lotteryRecords, err := lotteryRecord.ListByChatGroupId(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"tgChatGroupId": tgChatGroupId,
			"err":           err,
		}).Error("开奖记录查询异常")
		return
	}
	if len(lotteryRecords) == 0 {
		sendMsg.Text = "暂无开奖记录"
	} else {
		sendMsg.Text = "近10期开奖记录:\n"
		for _, record := range lotteryRecords {
			// 开奖类型查询开奖信息
			switch record.GameplayType {
			case enums.QuickThere.Value:
				quickThereLotteryRecord := &model.QuickThereLotteryRecord{
					Id: record.Id,
				}
				quickThereLotteryRecord, err := quickThereLotteryRecord.QueryById(db)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"IssueNumber": record.IssueNumber,
					}).Error("快三开奖记录查询异常")
					return
				}

				bigSmall, _ := enums.GetGameLotteryType(quickThereLotteryRecord.BigSmall)
				singleDouble, _ := enums.GetGameLotteryType(quickThereLotteryRecord.SingleDouble)

				triplet := ""
				if quickThereLotteryRecord.Triplet == 1 {
					triplet = "【豹子】"
				}

				sendMsg.Text += fmt.Sprintf("%s期 %s %d+%d+%d=%d %s %s %s\n",
					quickThereLotteryRecord.IssueNumber,
					"快三",
					quickThereLotteryRecord.ValueA,
					quickThereLotteryRecord.ValueB,
					quickThereLotteryRecord.ValueC,
					quickThereLotteryRecord.ValueA+quickThereLotteryRecord.ValueB+quickThereLotteryRecord.ValueC,
					bigSmall.Name,
					singleDouble.Name,
					triplet,
				)
			}
		}
	}
	sentMsg, err := sendMessage(bot, &sendMsg)

	if err != nil {
		blockedOrKicked(err, tgChatGroupId)
		return
	}

	go func(messageID int) {
		time.Sleep(1 * time.Minute)
		deleteMsg := tgbotapi.NewDeleteMessage(tgChatGroupId, messageID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("删除消息异常")
		}
	}(sentMsg.MessageID)
}

func updateChatGroupUserBalance(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	fromUser := query.From

	queryString := query.Data[strings.Index(query.Data, enums.CallbackUpdateChatGroupUserBalance.Value)+len(enums.CallbackUpdateChatGroupUserBalance.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"fromUserID":  fromUser.ID,
		}).Error("当前对话人非该群管理员")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, "请按照以下格式修改用户积分:\n"+
		"增加用户积分:[用户Id]+[积分] 例子: 10086+100\n"+
		"减少用户积分:[用户Id]-[积分] 例子: 10086-100\n"+
		"设置用户积分:[用户Id]=[积分] 例子: 10086=1000")

	// 设置当前机器人状态
	err = PrivateChatCacheAddRedis(fromUser.ID, &common.BotPrivateChatCache{
		ChatStatus:  enums.WaitUpdateUserBalance.Value,
		ChatGroupId: chatGroupId,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId":  fromUser.ID,
			"ChatStatus":  enums.WaitUpdateUserBalance.Value,
			"ChatGroupId": chatGroupId,
			"err":         err,
		}).Error("BotChatStatus 设置异常")
		return
	}

	_, err = sendMessage(bot, &sendMsg)

	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func queryChatGroupUser(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {

	chatId := query.Message.Chat.ID
	fromUser := query.From

	queryString := query.Data[strings.Index(query.Data, enums.CallbackQueryChatGroupUser.Value)+len(enums.CallbackQueryChatGroupUser.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"fromUserID":  fromUser.ID,
		}).Error("当前对话人非该群管理员")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, "请输入当前群聊中的用户名称，如:@username")

	// 设置当前机器人状态
	err = PrivateChatCacheAddRedis(chatId, &common.BotPrivateChatCache{
		ChatStatus:  enums.WaitQueryUser.Value,
		ChatGroupId: chatGroupId,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId":  fromUser.ID,
			"ChatStatus":  enums.WaitQueryUser.Value,
			"ChatGroupId": chatGroupId,
			"err":         err,
		}).Error("BotChatStatus 设置异常")
		return
	}

	_, err = sendMessage(bot, &sendMsg)

	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func updateGameDrawCycleCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	fromUser := query.From

	// 查询当前群配置的游戏类型
	queryString := query.Data[strings.Index(query.Data, enums.CallbackUpdateGameDrawCycle.Value)+len(enums.CallbackUpdateGameDrawCycle.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"fromUserID":  fromUser.ID,
		}).Error("当前对话人非该群管理员")
		return
	}

	sendMsg := tgbotapi.NewMessage(chatId, "请输入️要设置的开奖周期(1-60的整数)(单位:分钟)")

	// 设置当前机器人状态
	err = PrivateChatCacheAddRedis(fromUser.ID, &common.BotPrivateChatCache{
		ChatStatus:  enums.WaitGameDrawCycle.Value,
		ChatGroupId: chatGroupId,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId":  fromUser.ID,
			"ChatStatus":  enums.WaitGameDrawCycle.Value,
			"ChatGroupId": chatGroupId,
			"err":         err,
		}).Error("BotChatStatus 设置异常")
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
	fromUser := query.From

	// 查询使用的chatGroupId为内联键盘中的Data
	queryString := query.Data[strings.Index(query.Data, enums.CallbackUpdateGameplayStatus.Value)+len(enums.CallbackUpdateGameplayStatus.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"fromUserID":  fromUser.ID,
		}).Error("当前对话人非该群管理员")
		return
	}

	chatGroup, err := model.QueryChatGroupById(db, chatGroupId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
		}).Error("未查询到群配置信息 [未初始化]")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"err":         err,
		}).Error("群配置信息查询异常")
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
		// 发送提示消息
		sendMsg := tgbotapi.NewMessage(chatID, "关闭成功!")
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatID)
	} else {
		chatGroupUpdate.GameplayStatus = enums.GameplayStatusON.Value
		chatGroup.GameplayStatus = enums.GameplayStatusON.Value
		// 开启
		gameStart(bot, chatGroup)
		// 发送提示消息
		sendMsg := tgbotapi.NewMessage(chatID, "开启成功!")
		_, err = sendMessage(bot, &sendMsg)
		blockedOrKicked(err, chatID)
	}
	err = chatGroupUpdate.UpdateChatGroupStatusById(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Error("更新群配置-游戏状态异常")
		return
	}

	inlineKeyboardMarkup, err := buildChatGroupInlineKeyboardMarkup(query, chatGroup)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Error("组装群组配置内联键盘异常")
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
	fromUser := query.From
	messageId := query.Message.MessageID

	// 查询当前群配置的游戏类型
	queryString := query.Data[strings.Index(query.Data, enums.CallbackUpdateGameplayType.Value)+len(enums.CallbackUpdateGameplayType.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]
	gameplayType := callBackData["gameplayType"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"fromUserID":  fromUser.ID,
		}).Error("当前对话人非该群管理员")
		return
	}

	// 更改配置
	err = model.UpdateChatGroupGameplayTypeById(db, &model.ChatGroup{
		Id:           chatGroupId,
		GameplayType: gameplayType,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId":  chatGroupId,
			"GameplayType": gameplayType,
			"err":          err,
		}).Error("更新群配置异常")
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

func gameplayTypeCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	fromUser := query.From
	messageId := query.Message.MessageID

	// 查询当前群配置的游戏类型
	queryString := query.Data[strings.Index(query.Data, enums.CallbackGameplayType.Value)+len(enums.CallbackGameplayType.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"fromUserID":  fromUser.ID,
		}).Error("当前对话人非该群管理员")
		return
	}

	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, "请选择游戏类型:")

	inlineKeyboardRows, err := buildGameplayTypeInlineKeyboardButton(chatGroupId)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"err":         err,
		}).Error("组装游戏类型内联键盘异常")
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

func chatGroupConfigCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID
	fromUser := query.From

	// 查询使用的chatGroupId为内联键盘中的Data
	queryString := query.Data[strings.Index(query.Data, enums.CallbackChatGroupConfig.Value)+len(enums.CallbackChatGroupConfig.Value):]

	queryStringToMap, err := utils.QueryStringToMap(queryString)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"queryData": query.Data,
			"err":       err,
		}).Error("群配置信息查询异常")
		return
	}
	callBackDataKey := queryStringToMap["callbackDataKey"]

	callBackData, err := ButtonCallBackDataQueryFromRedis(callBackDataKey)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"callBackDataKey": callBackDataKey,
			"err":             err,
		}).Error("内联键盘回调参数redis查询异常")
		return
	}

	chatGroupId := callBackData["chatGroupId"]

	// 校验当前对话人是否为该群管理员
	err = checkGroupAdmin(chatGroupId, fromUser.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"fromUserId":  fromUser.ID,
			"chatGroupId": chatGroupId,
		}).Warn("当前对话人非该群管理员")
		return
	}

	chatGroup, err := model.QueryChatGroupById(db, chatGroupId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
		}).Error("未查询到群配置信息")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"err":         err,
		}).Error("群配置信息查询异常")
		return
	}

	inlineKeyboardMarkup, err := buildChatGroupInlineKeyboardMarkup(query, chatGroup)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Error("组装群组配置内联键盘异常")
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
		logrus.WithFields(logrus.Fields{
			"chatId":     chatId,
			"fromUserId": userId,
			"err":        err,
		}).Error("获取聊天成员异常")
		return
	}

	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, fmt.Sprintf("您好,%s!", member.User.FirstName))
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
	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, "点击下方按钮将机器人添加至【超级群组】并设置为管理员!")
	inviteBotLink := fmt.Sprintf("https://t.me/%s?startgroup=true", bot.Self.UserName)

	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("➕点击添加➕", inviteBotLink),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅好了✅", enums.CallbackAlreadyInvited.Value),
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
	chatId := query.Message.Chat.ID

	sendMsg, err := buildAdminGroupMsg(query)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId": query.From.ID,
			"err":        err,
		}).Error("查询管理群列表异常")
		return
	}

	_, err = sendMessage(bot, sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}

func joinedGroupCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	fromChatId := query.Message.Chat.ID

	sendMsg, err := buildJoinedGroupMsg(query)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserId": query.From.ID,
			"err":        err,
		}).Error("查询加入的群列表异常")
		return
	}

	_, err = sendMessage(bot, sendMsg)
	if err != nil {
		blockedOrKicked(err, fromChatId)
		return
	}
}

func alreadyReloadCallBack(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	chatId := query.Message.Chat.ID
	messageId := query.Message.MessageID

	sendMsg := tgbotapi.NewEditMessageText(chatId, messageId, "接下来就可以使用啦!")
	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅好了✅", enums.CallbackAdminGroup.Value),
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
			tgbotapi.NewInlineKeyboardButtonData("✅好了✅", enums.CallbackAlreadyReload.Value),
		),
	)
	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup
	_, err := sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}
