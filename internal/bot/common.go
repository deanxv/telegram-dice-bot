package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
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

const (
	RedisButtonCallBackDataKey  = "BUTTON_CALLBACK_DATA:%s"
	RedisBotPrivateChatCacheKey = "BOT_PRIVATE_CHAT_CACHE:TG_USER_ID:%v"
)

func sendMessage(bot *tgbotapi.BotAPI, chattable tgbotapi.Chattable) (tgbotapi.Message, error) {
	sentMsg, err := bot.Send(chattable)
	if err != nil {
		logrus.WithField("err", err).Error("发送消息异常")
		return sentMsg, err
	}
	return sentMsg, nil
}

func blockedOrKicked(err error, chatId int64) {
	if err != nil {
		if strings.Contains(err.Error(), "Forbidden: bot was blocked") {
			logrus.WithField("chatId", chatId).Warn("The bot was blocked ChatId")
			// 对话已被用户阻止
		} else if strings.Contains(err.Error(), "Forbidden: bot was kicked") {
			logrus.WithField("chatId", chatId).Warn("The bot was kicked ChatId")
			// 对话已被踢出群聊 修改群配置
			chatGroupUpdate := &model.ChatGroup{
				TgChatGroupId:   chatId,
				ChatGroupStatus: enums.GroupKicked.Value,
				GameplayStatus:  0,
			}
			_, err := chatGroupUpdate.UpdateGameplayStatusAndChatGroupStatusByTgChatId(db)
			if err != nil {
				logrus.WithField("err", err).Error("群配置修改失败")
				return
			}
		} else if strings.Contains(err.Error(), "Forbidden: the group chat was deleted") {
			logrus.WithField("chatId", chatId).Warn("the group chat was deleted")
			// 群组被删除 修改群配置
			chatGroupUpdate := &model.ChatGroup{
				TgChatGroupId:   chatId,
				ChatGroupStatus: enums.GroupDeleted.Value,
				GameplayStatus:  0,
			}
			_, err := chatGroupUpdate.UpdateGameplayStatusAndChatGroupStatusByTgChatId(db)
			if err != nil {
				logrus.WithField("err", err).Error("群配置修改失败")
				return
			}
		}
	}

}

// getChatMember 获取有关聊天成员的信息。
func getChatMember(bot *tgbotapi.BotAPI, chatID int64, userId int64) (tgbotapi.ChatMember, error) {
	chatMemberConfig := tgbotapi.ChatConfigWithUser{
		ChatID: chatID,
		UserID: userId,
	}

	return bot.GetChatMember(tgbotapi.GetChatMemberConfig{ChatConfigWithUser: chatMemberConfig})
}

func buildDefaultInlineKeyboardMarkup(bot *tgbotapi.BotAPI) *tgbotapi.InlineKeyboardMarkup {
	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👨🏻‍💼我加入的群", enums.CallbackJoinedGroup.Value),
			tgbotapi.NewInlineKeyboardButtonData("👮🏻‍♂️我管理的群", enums.CallbackAdminGroup.Value)),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🌟GitHub地址", "https://github.com/deanxv/telegram-dice-bot")),
	)
	return &newInlineKeyboardMarkup
}

func buildGameplayConfigInlineKeyboardButton(chatGroup *model.ChatGroup, callbackDataQueryString string) ([]tgbotapi.InlineKeyboardButton, error) {

	var inlineKeyboardButton []tgbotapi.InlineKeyboardButton
	if chatGroup.GameplayType == enums.QuickThere.Value {
		// 查询该配置
		quickThereConfig, err := model.QueryQuickThereConfigByChatGroupId(db, chatGroup.Id)

		if err != nil {
			return nil, err
		}
		inlineKeyboardButton = tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("⚖️简易倍率: %v 倍", quickThereConfig.SimpleOdds), fmt.Sprintf("%s%s", enums.CallbackUpdateQuickThereSimpleOdds.Value, callbackDataQueryString)),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("⚖️豹子倍率: %v 倍", quickThereConfig.TripletOdds), fmt.Sprintf("%s%s", enums.CallbackUpdateQuickThereTripletOdds.Value, callbackDataQueryString)),
		)
	}

	return inlineKeyboardButton, nil
}

func buildJoinedGroupMsg(query *tgbotapi.CallbackQuery) (*tgbotapi.EditMessageTextConfig, error) {
	fromUser := query.From
	fromChatId := query.Message.Chat.ID
	messageId := query.Message.MessageID

	var sendMsg tgbotapi.EditMessageTextConfig
	var inlineKeyboardRows [][]tgbotapi.InlineKeyboardButton

	// 查询当前人的信息
	chatGroupUserQuery := &model.ChatGroupUser{
		// 查询用户信息
		TgUserId: fromUser.ID,
		IsLeft:   0,
	}

	chatGroupUsers, err := chatGroupUserQuery.ListByTgUserIdAndIsLeft(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"TgUserId": fromUser.ID,
			"IsLeft":   0,
		}).Error("群组查询异常")
		return nil, err
	}
	if len(chatGroupUsers) == 0 {
		// 没有找到记录
		sendMsg = tgbotapi.NewEditMessageText(fromChatId, messageId, "您暂无加入的群!")
	} else {

		// 查询该用户的ChatGroupId
		var chatGroupIds []string
		for _, user := range chatGroupUsers {
			chatGroupIds = append(chatGroupIds, user.ChatGroupId)
		}

		chatGroups, err := model.ListChatGroupByIds(db, chatGroupIds)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"chatIds": chatGroupIds,
			}).Error("群组查询异常")
			return nil, err
		}

		sendMsg = tgbotapi.NewEditMessageText(fromChatId, messageId, fmt.Sprintf("您有%v个加入的群:", len(chatGroups)))

		for _, group := range chatGroups {
			callbackDataKey, err := ButtonCallBackDataAddRedis(map[string]string{
				"chatGroupId": group.Id,
			})
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"chatGroupId": group.Id,
					"err":         err,
				}).Error("内联键盘回调参数存入redis异常")
				return nil, err
			}

			callbackDataQueryString := utils.MapToQueryString(map[string]string{
				"callbackDataKey": callbackDataKey,
			})

			inlineKeyboardRows = append(inlineKeyboardRows,
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("👥 %s", group.TgChatGroupTitle), fmt.Sprintf("%s%s", enums.CallbackChatGroupInfo.Value, callbackDataQueryString)),
				),
			)
		}
	}
	inlineKeyboardRows = append(inlineKeyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️返回", enums.CallbackMainMenu.Value),
		),
	)

	// 组装列表数据
	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		inlineKeyboardRows...,
	)

	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup

	return &sendMsg, nil
}

func buildAdminGroupMsg(query *tgbotapi.CallbackQuery) (*tgbotapi.EditMessageTextConfig, error) {
	chatId := query.Message.Chat.ID
	fromUser := query.From
	messageId := query.Message.MessageID

	var sendMsg tgbotapi.EditMessageTextConfig
	var inlineKeyboardRows [][]tgbotapi.InlineKeyboardButton

	inlineKeyboardRows = append(inlineKeyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕点击添加新的群组", enums.CallbackAddAdminGroup.Value),
		),
	)

	// 查询当前消息来源人关联的群聊
	chatGroupAdmins, err := model.ListChatGroupAdminByAdminTgUserId(db, fromUser.ID)
	if len(chatGroupAdmins) == 0 && err == nil {
		sendMsg = tgbotapi.NewEditMessageText(chatId, messageId, "您暂无管理的群!")
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromUserID": fromUser.ID,
		}).Error("查询管理群列表异常")
		return nil, errors.New("查询管理群列表异常")
	} else {
		sendMsg = tgbotapi.NewEditMessageText(chatId, messageId, fmt.Sprintf("您有%v个管理的群:", len(chatGroupAdmins)))
		for _, chatGroupAdmin := range chatGroupAdmins {
			// 查找该群的信息
			ChatGroup, err := model.QueryChatGroupById(db, chatGroupAdmin.ChatGroupId)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logrus.WithFields(logrus.Fields{
					"ChatGroupId": chatGroupAdmin.ChatGroupId,
				}).Warn("未查询到群配置")
				continue
			} else if err != nil {
				logrus.WithFields(logrus.Fields{
					"ChatGroupId": chatGroupAdmin.ChatGroupId,
					"err":         err,
				}).Error("群配置查询异常")
				continue
			} else {
				callbackDataKey, err := ButtonCallBackDataAddRedis(map[string]string{
					"chatGroupId": ChatGroup.Id,
				})
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"chatGroupId": ChatGroup.Id,
						"err":         err,
					}).Error("内联键盘回调参数存入redis异常")
					return nil, err
				}

				callbackDataQueryString := utils.MapToQueryString(map[string]string{
					"callbackDataKey": callbackDataKey,
				})

				inlineKeyboardRows = append(inlineKeyboardRows,
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("👥 %s", ChatGroup.TgChatGroupTitle), fmt.Sprintf("%s%s", enums.CallbackChatGroupConfig.Value, callbackDataQueryString))),
				)
			}
		}
	}
	inlineKeyboardRows = append(inlineKeyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️返回", enums.CallbackMainMenu.Value),
		),
	)

	// 组装列表数据
	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		inlineKeyboardRows...,
	)

	sendMsg.ReplyMarkup = &newInlineKeyboardMarkup
	return &sendMsg, nil
}

func checkGroupAdmin(chatGroupId string, tgUserId int64) error {
	_, err := model.QueryChatGroupAdminByChatGroupIdAndTgUserId(db, chatGroupId, tgUserId)
	if err != nil {
		return err
	}
	return nil
}

func buildGameplayTypeInlineKeyboardButton(chatGroupId string) ([][]tgbotapi.InlineKeyboardButton, error) {

	ChatGroup, err := model.QueryChatGroupById(db, chatGroupId)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
		}).Warn("未查询到群组信息 [未初始化过配置]")
		return nil, err
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroupId,
			"err":         err,
		}).Error("群组信息查询异常")
		return nil, err
	}

	var inlineKeyboardRows [][]tgbotapi.InlineKeyboardButton

	for key, value := range enums.GameplayTypeMap {

		callBackDataKey, err := ButtonCallBackDataAddRedis(map[string]string{
			"chatGroupId":  chatGroupId,
			"gameplayType": key,
		})

		if err != nil {
			logrus.WithFields(logrus.Fields{
				"chatGroupId":  ChatGroup.Id,
				"gameplayType": key,
				"err":          err,
			}).Error("内联键盘回调参数存入redis异常")
			return nil, err
		}

		buttonDataText := value.Name

		if ChatGroup.GameplayType == key {
			buttonDataText = fmt.Sprintf("%s✅", buttonDataText)
		}

		callBackDataQueryString := utils.MapToQueryString(map[string]string{
			"callbackDataKey": callBackDataKey,
		})

		inlineKeyboardRows = append(inlineKeyboardRows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(buttonDataText, fmt.Sprintf("%s%s", enums.CallbackUpdateGameplayType.Value, callBackDataQueryString)),
			),
		)
	}

	callbackDataKey, err := ButtonCallBackDataAddRedis(map[string]string{
		"chatGroupId": ChatGroup.Id,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": ChatGroup.Id,
			"err":         err,
		}).Error("内联键盘回调参数存入redis异常")
		return nil, err
	}

	callBackDataQueryString := utils.MapToQueryString(map[string]string{
		"callbackDataKey": callbackDataKey,
	})

	inlineKeyboardRows = append(inlineKeyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️返回", fmt.Sprintf("%s%s", enums.CallbackChatGroupConfig.Value, callBackDataQueryString)),
		),
	)
	return inlineKeyboardRows, nil
}

func ButtonCallBackDataAddRedis(queryMap map[string]string) (string, error) {
	jsonBytes, err := json.Marshal(queryMap)
	if err != nil {
		return "", err
	}

	id, err := utils.NextID()
	if err != nil {
		return "", err
	}

	redisKey := fmt.Sprintf(RedisButtonCallBackDataKey, id)

	// 存入redis
	err = redisDB.Set(redisDB.Context(), redisKey, string(jsonBytes), 1*time.Hour).Err()

	return id, nil
}

func ButtonCallBackDataQueryFromRedis(key string) (map[string]string, error) {

	redisKey := fmt.Sprintf(RedisButtonCallBackDataKey, key)
	result := redisDB.Get(redisDB.Context(), redisKey)
	if errors.Is(result.Err(), redis.Nil) || result == nil {
		logrus.WithFields(logrus.Fields{
			"redisKey": redisKey,
		}).Error("redis键不存在")
		return nil, result.Err()
	} else if result.Err() != nil {
		logrus.WithFields(logrus.Fields{
			"redisKey": redisKey,
			"err":      result.Err(),
		}).Error("redis查询键盘回调信息异常")
		return nil, result.Err()
	} else {
		var m map[string]string
		mapString, _ := result.Result()
		err := json.Unmarshal([]byte(mapString), &m)
		if err != nil {
			return nil, err
		}
		return m, nil
	}
}

func PrivateChatCacheAddRedis(tgUserID int64, botPrivateChatCache *common.BotPrivateChatCache) error {

	jsonBytes, err := json.Marshal(botPrivateChatCache)
	if err != nil {
		return err
	}

	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, tgUserID)

	// 存入redis
	return redisDB.Set(redisDB.Context(), redisKey, string(jsonBytes), 24*time.Hour).Err()

}

func buildChatGroupInlineKeyboardMarkup(query *tgbotapi.CallbackQuery, chatGroup *model.ChatGroup) (*tgbotapi.InlineKeyboardMarkup, error) {

	gameplayType, b := enums.GetGameplayType(chatGroup.GameplayType)
	if !b {
		logrus.WithFields(logrus.Fields{
			"GameplayType": chatGroup.GameplayType,
		}).Error("群配置玩法映射查询异常")
		return nil, errors.New("群配置玩法查询异常")
	}
	gameplayStatus, b := enums.GetGameplayStatus(chatGroup.GameplayStatus)
	if !b {
		logrus.WithFields(logrus.Fields{
			"GameplayStatus": chatGroup.GameplayStatus,
		}).Error("群配置游戏状态映射查询异常")
		return nil, errors.New("群配置游戏状态查询异常")
	}

	// 重新生成内联键盘回调key
	callbackDataKey, err := ButtonCallBackDataAddRedis(map[string]string{
		"chatGroupId": chatGroup.Id,
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("内联键盘回调参数存入redis异常")
		return nil, err
	}

	callbackDataQueryString := utils.MapToQueryString(map[string]string{
		"callbackDataKey": callbackDataKey,
	})

	inlineKeyboardButtons, err := buildGameplayConfigInlineKeyboardButton(chatGroup, callbackDataQueryString)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroup.Id,
		}).Warn("未查询到该群的配置信息")
		return nil, err
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("群配置信息查询异常")
		return nil, err
	}

	newInlineKeyboardMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🛠️当前玩法:【%s】", gameplayType.Name), fmt.Sprintf("%s%s", enums.CallbackGameplayType.Value, callbackDataQueryString)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🕹️开启状态: %s", gameplayStatus.Name), fmt.Sprintf("%s%s", enums.CallbackUpdateGameplayStatus.Value, callbackDataQueryString)),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("⏲️开奖周期: %v 分钟", chatGroup.GameDrawCycle), fmt.Sprintf("%s%s", enums.CallbackUpdateGameDrawCycle.Value, callbackDataQueryString)),
		),
		inlineKeyboardButtons,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍查询用户信息", fmt.Sprintf("%s%s", enums.CallbackQueryChatGroupUser.Value, callbackDataQueryString)),
			tgbotapi.NewInlineKeyboardButtonData("🖊️修改用户积分", fmt.Sprintf("%s%s", enums.CallbackUpdateChatGroupUserBalance.Value, callbackDataQueryString)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️返回", enums.CallbackAdminGroup.Value),
			tgbotapi.NewInlineKeyboardButtonData("🚮我已退群", fmt.Sprintf("%s%s", enums.CallbackAdminExitGroup.Value, callbackDataQueryString)),
		),
	)
	return &newInlineKeyboardMarkup, nil
}
