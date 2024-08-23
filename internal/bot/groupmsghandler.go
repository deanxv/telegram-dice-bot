package bot

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
	"telegram-dice-bot/internal/utils"
	"time"
)

func handleGroupCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {

	chatId := message.Chat.ID
	user := message.From

	chatMember, err := getChatMember(bot, chatId, user.ID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatId":     chatId,
			"fromUserId": user.ID,
			"err":        err,
		}).Error("获取聊天成员异常")
		return
	}

	switch message.Command() {
	case "reload":
		if chatMember.IsAdministrator() || chatMember.IsCreator() {
			handleGroupReloadCommand(bot, message)
		}
	case "register":
		handleRegisterCommand(bot, message)
	case "sign":
		handleSignCommand(bot, message)
	case "my":
		handleMyCommand(bot, message)
	case "myhistory":
		handleMyHistoryCommand(bot, message)
	case "help":
		handleHelpCommand(bot, message)
	}
}

func handleHelpCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	fromChatId := message.Chat.ID
	messageID := message.MessageID

	// 查询当前群组的配置
	chatGroup, err := model.QueryChatGroupByTgChatId(db, fromChatId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 群未初始化则不处理
		logrus.WithFields(logrus.Fields{
			"fromChatId": fromChatId,
		}).Warn("未查询到群配置信息 [未初始化]")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"fromChatId": fromChatId,
			"err":        err,
		}).Error("群配置信息查询异常")
		return
	}

	var gameHelp string

	if chatGroup.GameplayType == enums.QuickThere.Value {
		quickThereConfig, err := model.QueryQuickThereConfigByChatGroupId(db, chatGroup.Id)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"chatGroupId": chatGroup.Id,
				"err":         err,
			}).Error("群的快三配置异常")
			return
		}
		gameHelp = fmt.Sprintf("当前倍率:\n简易%v倍丨豹子%v倍\n\n支持竞猜类型: 单、双、大、小、豹子\n竞猜示例(竞猜类型-单,下注积分-20):\n #单 20", quickThereConfig.SimpleOdds, quickThereConfig.TripletOdds)
	}

	gameplayType, b := enums.GetGameplayType(chatGroup.GameplayType)
	if !b {
		logrus.WithFields(logrus.Fields{
			"GameplayType": chatGroup.GameplayType,
			"err":          err,
		}).Error("群配置玩法映射查询异常")
		return
	}

	// help命令
	msgConfig := tgbotapi.NewMessage(fromChatId,
		fmt.Sprintf("/help 帮助\n"+
			"/register 用户注册\n"+
			"/sign 用户签到\n"+
			"/my 查询积分\n"+
			"/myhistory 查询历史下注记录\n\n"+
			"当前游戏类型【%s】\n"+
			"开奖周期 %v 分钟\n"+
			"%s",
			gameplayType.Name,
			chatGroup.GameDrawCycle,
			gameHelp))
	msgConfig.ReplyToMessageID = messageID
	sentMsg, err := sendMessage(bot, &msgConfig)
	if err != nil {
		blockedOrKicked(err, fromChatId)
		return
	}
	go func(messageID int) {
		time.Sleep(1 * time.Minute)
		deleteMsg := tgbotapi.NewDeleteMessage(fromChatId, messageID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("删除消息异常")
		}
	}(sentMsg.MessageID)
}

func handleMyHistoryCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	fromUser := message.From
	messageId := message.MessageID
	tgChatId := message.Chat.ID

	chatGroup, err := model.QueryChatGroupByTgChatId(db, tgChatId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"tgChatId": tgChatId,
		}).Warn("未查询到群配置 [未初始化]")
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"tgChatId": tgChatId,
			"err":      err,
		}).Error("群配置查询异常")
		return
	}

	chatGroupUserQuery := &model.ChatGroupUser{
		// 查询用户信息
		TgUserId:    fromUser.ID,
		ChatGroupId: chatGroup.Id,
	}

	chatGroupUser, err := chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(db)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有找到记录
		msgConfig := tgbotapi.NewMessage(tgChatId, "您还未注册，使用 /register 进行注册。")
		msgConfig.ReplyToMessageID = messageId
		_, err := sendMessage(bot, &msgConfig)
		blockedOrKicked(err, tgChatId)
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"tgChatId": tgChatId,
			"err":      err,
		}).Error("群配置查询异常")
		return
	}
	// 查询下注记录

	betRecord := &model.BetRecord{ChatGroupUserId: chatGroupUser.Id}
	betRecords, err := betRecord.ListByChatGroupUserId(db)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatGroupUserId": chatGroupUser.Id,
			"err":             err,
		}).Error("查询下注记录异常")
		return
	}
	sendMsg := tgbotapi.NewMessage(tgChatId, "")
	sendMsg.ReplyToMessageID = messageId

	if len(betRecords) == 0 {
		// 下注记录为空
		sendMsg.Text = "您还没有下注记录哦!"
	} else {
		sendMsg.Text = "您的近10期下注记录如下:\n"

		for _, record := range betRecords {
			// 开奖类型查询开奖信息
			switch record.GameplayType {
			case enums.QuickThere.Value:

				quickThereBetRecord := &model.QuickThereBetRecord{
					Id: record.Id,
				}
				quickThereBetRecord, err := quickThereBetRecord.QueryById(db)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"recordId": record.Id,
						"err":      err,
					}).Error("查询快三下注记录异常")
					return
				}

				betType, _ := enums.GetGameLotteryType(quickThereBetRecord.BetType)

				betResultTypeName := "「未开奖」"

				if quickThereBetRecord.BetResultType != nil {
					betType, _ := enums.GetBetResultType(*quickThereBetRecord.BetResultType)
					betResultTypeName = betType.Name
				}

				sendMsg.Text += fmt.Sprintf("%s期 %s %s %v %s %v \n",
					record.IssueNumber,
					"快三",
					betType.Name,
					quickThereBetRecord.BetAmount,
					betResultTypeName,
					quickThereBetRecord.BetResultAmount,
				)
			}
		}

		sentMsg, err := sendMessage(bot, &sendMsg)
		if err != nil {
			blockedOrKicked(err, tgChatId)
			return
		}
		go func(messageID int) {
			time.Sleep(1 * time.Minute)
			deleteMsg := tgbotapi.NewDeleteMessage(tgChatId, messageID)
			_, err := bot.Request(deleteMsg)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"err": err,
				}).Error("删除消息异常")
			}
		}(sentMsg.MessageID)

		return
	}
}

func handleGroupNewMembers(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// 检查是否有新成员加入
	if message != nil && message.NewChatMembers != nil {
		chatId := message.Chat.ID
		chatTitle := message.Chat.Title
		for _, newMember := range message.NewChatMembers {
			// 检查是否是机器人被邀请加入
			if newMember.UserName == bot.Self.UserName {

				tx := db.Begin()
				logrus.WithFields(logrus.Fields{
					"chatTitle": chatTitle,
				}).Info("Bot was added to a group")
				// 查找是否原来关联过该群
				chatGroup, err := model.QueryChatGroupByTgChatId(tx, chatId)
				if errors.Is(err, gorm.ErrRecordNotFound) {
					logrus.WithFields(logrus.Fields{
						"chatId": chatId,
					}).Warn("未查询到群配置信息 [未初始化]")

					// 初始化群配置
					chatGroupId, err := utils.NextID()
					if err != nil {
						logrus.Error("SnowFlakeId create error")
						return
					}

					chatGroup := &model.ChatGroup{
						Id:               chatGroupId,
						TgChatGroupTitle: chatTitle,
						TgChatGroupId:    chatId,
						GameplayType:     enums.QuickThere.Value,
						GameDrawCycle:    1,
						GameplayStatus:   0,
						ChatGroupStatus:  enums.GroupNormal.Value,
						CreateTime:       time.Now().Format("2006-01-02 15:04:05"),
					}
					err = chatGroup.Create(tx)
					if err != nil {
						logrus.WithFields(logrus.Fields{
							"err": err,
						}).Error("初始化群配置异常")
						tx.Rollback()
						return
					}

					// 初始化快三配置
					quickThereConfig := &model.QuickThereConfig{
						ChatGroupId: chatGroupId,
						SimpleOdds:  2,
						TripletOdds: 10,
						CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
					}

					err = quickThereConfig.Create(tx)
					if err != nil {
						logrus.WithFields(logrus.Fields{
							"err": err,
						}).Error("初始化快三配置异常")
						tx.Rollback()
						return
					}

					// 提交事务
					if err := tx.Commit().Error; err != nil {
						// 提交事务时出现异常，回滚事务
						tx.Rollback()
					}

					logrus.WithFields(logrus.Fields{
						"chatId": chatId,
					}).Info("该群初始化配置成功")
					return
				} else if err != nil {
					logrus.WithFields(logrus.Fields{
						"chatId": chatId,
						"err":    err,
					}).Error("群配置查询异常")
					return
				} else {
					// 更新原有配置状态为正常
					chatGroup.ChatGroupStatus = enums.GroupNormal.Value
					result := db.Save(&chatGroup)
					if result.Error != nil {
						logrus.WithFields(logrus.Fields{
							"err": err,
						}).Error("更新配置异常")
						return
					}
					logrus.WithFields(logrus.Fields{
						"chatId": chatId,
					}).Warn("该群已被初始化过配置")
					return
				}

			} else {
				// 非机器人入群 则检查是否已注册
				chatGroup, err := model.QueryChatGroupByTgChatId(db, chatId)
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// 群未初始化则不处理
					logrus.WithFields(logrus.Fields{
						"chatId": chatId,
					}).Warn("未查询到群信息 [未初始化]")
				} else if err != nil {
					logrus.WithFields(logrus.Fields{
						"chatId": chatId,
						"err":    err,
					}).Error("群信息查询异常")
					return
				} else {
					// 群存在则判断该用户是否已注册
					chatGroupUserQuery := &model.ChatGroupUser{
						TgUserId:    newMember.ID,
						ChatGroupId: chatGroup.Id,
					}
					chatGroupUser, err := chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(db)

					if errors.Is(err, gorm.ErrRecordNotFound) {
						logrus.WithFields(logrus.Fields{
							"TgUserId":    newMember.ID,
							"ChatGroupId": chatGroup.Id,
						}).Warn("未查询到用户信息")
						return
					} else if err != nil {
						logrus.WithFields(logrus.Fields{
							"TgUserId":    newMember.ID,
							"ChatGroupId": chatGroup.Id,
							"err":         err,
						}).Error("查询用户信息异常")
						return
					} else {
						// 已注册则更新状态为未离开
						chatGroupUser.IsLeft = 0
						db.Save(&chatGroupUser)
					}
				}
			}
		}
	}
}

func handleGroupMigrateFromChatID(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {

	if message.MigrateFromChatID != 0 {
		oldGroupID := message.MigrateFromChatID
		newSuperGroupID := message.Chat.ID
		// 普通群组升级为超级群组
		chatGroup, err := model.QueryChatGroupByTgChatId(db, oldGroupID)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"oldGroupID": oldGroupID,
				"err":        err,
			}).Error("群配置查询异常")
			return
		}
		chatGroup.TgChatGroupId = newSuperGroupID
		db.Save(&chatGroup)
		logrus.WithFields(logrus.Fields{
			"oldGroupID":      oldGroupID,
			"newSuperGroupID": newSuperGroupID,
		}).Info("群组 ID 已更新 [升级为超级群组]")
	}
}

func handleGroupNewChatTitle(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {

	if message.NewChatTitle != "" {
		newChatTitle := message.NewChatTitle
		tgChatGroupId := message.Chat.ID
		chatGroup, err := model.QueryChatGroupByTgChatId(db, tgChatGroupId)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"tgChatGroupId": tgChatGroupId,
				"err":           err,
			}).Error("群配置查询异常")
			return
		}
		chatGroup.TgChatGroupTitle = newChatTitle
		db.Save(&chatGroup)
		logrus.WithFields(logrus.Fields{
			"chatGroupId":  chatGroup.Id,
			"newChatTitle": newChatTitle,
		}).Info("群组Title已更新")
	}
}

func handleGroupLeftChatMember(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// 检查是否有人离开群组
	if message.LeftChatMember != nil {
		tgChatGroupId := message.Chat.ID
		leftUser := message.LeftChatMember

		tx := db.Begin()

		// 查询该群的信息
		chatGroup, err := model.QueryChatGroupByTgChatId(tx, tgChatGroupId)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"tgChatGroupId": tgChatGroupId,
				"err":           err,
			}).Error("群配置查询异常")
			tx.Rollback()
			return
		}

		// 查询该用户
		chatGroupUserQuery := &model.ChatGroupUser{
			TgUserId:    leftUser.ID,
			ChatGroupId: chatGroup.Id,
		}

		chatGroupUser, err := chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(tx)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logrus.WithFields(logrus.Fields{
				"TgUserId":    chatGroupUserQuery.TgUserId,
				"ChatGroupId": chatGroupUserQuery.ChatGroupId,
			}).Warn("该用户未注册")
			return
		} else if err != nil {
			logrus.WithFields(logrus.Fields{
				"TgUserId":    chatGroupUserQuery.TgUserId,
				"ChatGroupId": chatGroupUserQuery.ChatGroupId,
				"err":         err,
			}).Error("查询用户信息异常")
			return
		} else {
			// 更新该用户状态为离开
			chatGroupUser.IsLeft = 1
			result := tx.Save(&chatGroupUser)
			if result.Error != nil {
				logrus.WithFields(logrus.Fields{
					"err": err,
				}).Error("更新用户状态异常")
				tx.Rollback()
				return
			}
		}

		chatGroupAdmin, err := model.QueryChatGroupAdminByChatGroupIdAndTgUserId(tx, chatGroup.Id, leftUser.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logrus.WithFields(logrus.Fields{
				"tgChatGroupId": chatGroup.Id,
				"tgUserId":      leftUser.ID,
			}).Warn("该用户非管理员")
			// 提交事务
			if err := tx.Commit().Error; err != nil {
				// 提交事务时出现异常，回滚事务
				tx.Rollback()
			}
			return
		} else if err != nil {
			logrus.WithFields(logrus.Fields{
				"tgChatGroupId": chatGroup.Id,
				"tgUserId":      leftUser.ID,
				"err":           err,
			}).Error("管理员用户查询异常")
			tx.Rollback()
		} else {
			// 删除该用户的管理员权限
			tx.Delete(&chatGroupAdmin)
			// 提交事务
			if err := tx.Commit().Error; err != nil {
				// 提交事务时出现异常，回滚事务
				tx.Rollback()
			}
		}
	}
}

func handleGroupText(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	go handleBettingText(bot, message)
}

func handleBettingText(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	tgChatGroupId := message.Chat.ID
	messageId := message.MessageID

	// 查询该群的信息
	chatGroup, err := model.QueryChatGroupByTgChatId(db, tgChatGroupId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"tgChatGroupId": tgChatGroupId,
		}).Warn("未查询到该群配置 [未初始化]")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"tgChatGroupId": tgChatGroupId,
			"err":           err,
		}).Error("群配置查询异常")
		return
	}

	if chatGroup.GameplayType == enums.QuickThere.Value {
		b, err := handleQuickThereBettingText(bot, chatGroup, message)
		if b {
			// 回复下注成功信息
			replyMsg := tgbotapi.NewMessage(tgChatGroupId, "下注成功!")
			replyMsg.ReplyToMessageID = messageId
			_, err = bot.Send(replyMsg)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"err": err,
				}).Error("发送消息异常")
				blockedOrKicked(err, tgChatGroupId)
			}
		} else if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("处理下注信息异常")
		}
	}
}

func handleQuickThereBettingText(bot *tgbotapi.BotAPI, chatGroup *model.ChatGroup, message *tgbotapi.Message) (bool, error) {
	text := message.Text
	tgChatGroupId := message.Chat.ID
	messageId := message.MessageID

	// 解析下注命令，示例命令格式：#单 20
	parts := strings.Fields(text)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "#") {
		return false, nil
	}

	// 获取下注类型和下注积分
	betType := parts[0][1:]
	if betType != "单" && betType != "双" && betType != "大" && betType != "小" && betType != "豹子" {
		return false, nil
	}

	betAmount, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || betAmount <= 0 {
		return false, errors.New("下注积分异常")
	}

	if chatGroup.GameplayStatus == enums.GameplayStatusOFF.Value {
		registrationMsg := tgbotapi.NewMessage(tgChatGroupId, "功能未开启！")
		registrationMsg.ReplyToMessageID = messageId
		_, err := bot.Send(registrationMsg)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("功能未开启提示消息异常")
			blockedOrKicked(err, tgChatGroupId)
			return false, err
		}
		return false, nil
	}

	// 获取当前进行的期号
	redisKey := fmt.Sprintf(RedisCurrentIssueNumberKey, chatGroup.Id)
	issueNumberResult := redisDB.Get(redisDB.Context(), redisKey)
	if errors.Is(issueNumberResult.Err(), redis.Nil) || issueNumberResult == nil {
		logrus.WithFields(logrus.Fields{
			"redisKey": redisKey,
			"err":      issueNumberResult.Err(),
		}).Warn("redis键不存在")
		replyMsg := tgbotapi.NewMessage(tgChatGroupId, "当前暂无开奖活动!")
		replyMsg.ReplyToMessageID = messageId
		_, sendErr := bot.Send(replyMsg)
		blockedOrKicked(sendErr, tgChatGroupId)
		return false, nil
	} else if issueNumberResult.Err() != nil {
		logrus.WithFields(logrus.Fields{
			"redisKey": redisKey,
			"err":      issueNumberResult.Err(),
		}).Error("redis获取当前期号异常")
		return false, nil
	}

	issueNumber, _ := issueNumberResult.Result()

	// 存储下注记录到数据库，并扣除用户余额
	b, err := storeQuickThereBetRecord(bot, chatGroup, message, &model.QuickThereBetRecord{
		IssueNumber: issueNumber,
		BetType:     betType,
		BetAmount:   betAmount,
	})

	if !b && err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Error("保存下注记录异常")
		return false, err
	}
	return b, nil
}

func storeQuickThereBetRecord(bot *tgbotapi.BotAPI, chatGroup *model.ChatGroup, message *tgbotapi.Message, quickThereBetRecord *model.QuickThereBetRecord) (bool, error) {
	user := message.From
	messageId := message.MessageID
	chatId := message.Chat.ID

	// 获取用户对应的互斥锁
	userLockKey := fmt.Sprintf(ChatGroupUserLockKey, message.Chat.ID, user.ID)
	userLock := getUserLock(userLockKey)
	userLock.Lock()
	defer userLock.Unlock()

	tx := db.Begin()

	// 查询该群用户信息
	chatGroupUserQuery := &model.ChatGroupUser{
		TgUserId:    user.ID,
		ChatGroupId: chatGroup.Id,
	}

	chatGroupUser, err := chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(tx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 用户不存在，发送注册提示
		registrationMsg := tgbotapi.NewMessage(chatId, "您还未注册，使用 /register 进行注册。")
		registrationMsg.ReplyToMessageID = messageId
		_, sendErr := bot.Send(registrationMsg)
		if sendErr != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("发送注册提示消息异常")
			blockedOrKicked(sendErr, chatId)
			return false, sendErr
		}
		return false, nil
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    chatGroupUserQuery.TgUserId,
			"ChatGroupId": chatGroupUserQuery.ChatGroupId,
			"err":         err,
		}).Error("查询用户信息异常")
		return false, err
	} else {
		// 检查用户余额是否足够
		if chatGroupUser.Balance < quickThereBetRecord.BetAmount {
			// 用户不存在，发送注册提示
			balanceInsufficientMsg := tgbotapi.NewMessage(chatId, "您的余额不足!")
			balanceInsufficientMsg.ReplyToMessageID = messageId
			_, err := bot.Send(balanceInsufficientMsg)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"err": err,
				}).Error("您的余额不足提示异常")
				blockedOrKicked(err, chatId)
				return false, err
			} else {
				return false, nil
			}
		}

		// 扣除用户余额
		chatGroupUser.Balance -= quickThereBetRecord.BetAmount
		// 同步更新用户信息
		chatGroupUser.Username = user.UserName

		result := tx.Save(&chatGroupUser)
		if result.Error != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("扣除用户余额异常")
			tx.Rollback()
			return false, result.Error
		}
		currentTime := time.Now().Format("2006-01-02 15:04:05")

		// 映射下注类型
		betType, b := enums.GetGameLotteryTypeForName(quickThereBetRecord.BetType)
		if !b {
			logrus.WithFields(logrus.Fields{
				"betType": quickThereBetRecord.BetType,
				"err":     err,
			}).Error("下注类型映射异常")
			return false, errors.New("该下注类型映射异常")
		}

		id, err := utils.NextID()
		if err != nil {
			logrus.Error("SnowFlakeId create error")
			return false, err
		}

		// 保存下注记录
		betRecord := &model.BetRecord{
			Id:              id,
			ChatGroupUserId: chatGroupUser.Id,
			ChatGroupId:     chatGroup.Id,
			GameplayType:    chatGroup.GameplayType,
			IssueNumber:     quickThereBetRecord.IssueNumber,
			UpdateTime:      currentTime,
			CreateTime:      currentTime,
		}

		err = betRecord.Create(tx)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("保存下注记录异常")
			// 如果保存下注记录失败，需要返还用户余额
			//chatGroupUser.Balance += quickThereBetRecord.BetAmount
			//tx.Save(&user)
			tx.Rollback()
			return false, err
		}

		// 保存快三下注记录
		quickThereBetRecordCreate := &model.QuickThereBetRecord{
			Id:              id,
			ChatGroupUserId: chatGroupUser.Id,
			ChatGroupId:     chatGroup.Id,
			IssueNumber:     quickThereBetRecord.IssueNumber,
			BetType:         betType.Value,
			BetAmount:       quickThereBetRecord.BetAmount,
			SettleStatus:    enums.Unsettled.Value,
			UpdateTime:      currentTime,
			CreateTime:      currentTime,
		}

		err = quickThereBetRecordCreate.Create(tx)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("保存快三下注记录异常")
			// 如果保存下注记录失败，需要返还用户余额
			//chatGroupUser.Balance += quickThereBetRecord.BetAmount
			//tx.Save(&user)
			tx.Rollback()
			return false, result.Error
		}

		// 提交事务
		if err := tx.Commit().Error; err != nil {
			// 提交事务时出现异常，回滚事务
			tx.Rollback()
		}

		return true, nil
	}
}

func handleMyCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	tgChatGroupId := message.Chat.ID
	fromUser := message.From
	messageId := message.MessageID

	// 查询该群的信息
	chatGroup, err := model.QueryChatGroupByTgChatId(db, tgChatGroupId)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"tgChatGroupId": tgChatGroupId,
			"err":           err,
		}).Error("群配置查询异常")
		return
	}

	// 查询该群用户信息
	chatGroupUserQuery := &model.ChatGroupUser{
		TgUserId:    fromUser.ID,
		ChatGroupId: chatGroup.Id,
	}

	chatGroupUser, err := chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(db)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有找到记录
		msgConfig := tgbotapi.NewMessage(tgChatGroupId, "请发送 /register 注册用户！")
		msgConfig.ReplyToMessageID = messageId
		_, err := sendMessage(bot, &msgConfig)
		if err != nil {
			blockedOrKicked(err, tgChatGroupId)
			return
		}
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    fromUser.ID,
			"ChatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("群用户查询异常")
	} else {
		msgConfig := tgbotapi.NewMessage(tgChatGroupId, fmt.Sprintf("%s 您的积分余额为%.2f", fromUser.FirstName, chatGroupUser.Balance))
		msgConfig.ReplyToMessageID = messageId
		sentMsg, err := sendMessage(bot, &msgConfig)
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
}

func handleSignCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {

	tgChatGroupId := message.Chat.ID
	fromUser := message.From
	messageId := message.MessageID

	// 查询该群的信息
	chatGroup, err := model.QueryChatGroupByTgChatId(db, tgChatGroupId)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"tgChatGroupId": tgChatGroupId,
			"err":           err,
		}).Error("群配置查询异常")
		return
	}

	// 查询该群用户信息
	chatGroupUserQuery := &model.ChatGroupUser{
		TgUserId:    fromUser.ID,
		ChatGroupId: chatGroup.Id,
	}

	chatGroupUser, err := chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(db)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有找到记录
		msgConfig := tgbotapi.NewMessage(tgChatGroupId, "请发送 /register 注册用户！")
		msgConfig.ReplyToMessageID = messageId
		_, err := sendMessage(bot, &msgConfig)
		blockedOrKicked(err, tgChatGroupId)
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    fromUser.ID,
			"ChatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("群用户查询异常")
	} else {
		// 获取用户对应的互斥锁
		userLockKey := fmt.Sprintf(ChatGroupUserLockKey, message.Chat.ID, message.From.ID)
		userLock := getUserLock(userLockKey)
		userLock.Lock()
		defer userLock.Unlock()

		if chatGroupUser.SignInTime != "" {
			signInTime, err := time.Parse("2006-01-02 15:04:05", chatGroupUser.SignInTime)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"SignInTime": chatGroupUser.SignInTime,
					"err":        err,
				}).Error("时间解析异常")
				return
			}
			// 获取当前时间
			currentTime := time.Now()
			currentMidnight := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
			if !signInTime.Before(currentMidnight) {
				msgConfig := tgbotapi.NewMessage(tgChatGroupId, "今天已签到过了哦！")
				msgConfig.ReplyToMessageID = messageId
				_, err := sendMessage(bot, &msgConfig)
				blockedOrKicked(err, tgChatGroupId)
				return
			}
		}
		chatGroupUser.SignInTime = time.Now().Format("2006-01-02 15:04:05")
		chatGroupUser.Balance += 1000
		result := db.Save(&chatGroupUser)
		if result.Error != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("保存用户信息异常")
			return
		}
		msgConfig := tgbotapi.NewMessage(tgChatGroupId, "签到成功！奖励1000积分！")
		msgConfig.ReplyToMessageID = messageId
		_, err := sendMessage(bot, &msgConfig)
		blockedOrKicked(err, tgChatGroupId)
	}
}

func handleRegisterCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	tgChatGroupId := message.Chat.ID
	fromUser := message.From
	messageId := message.MessageID

	// 查询该群的信息
	chatGroup, err := model.QueryChatGroupByTgChatId(db, tgChatGroupId)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Error("群配置查询异常")
		return
	}

	// 查询该群用户信息
	chatGroupUserQuery := &model.ChatGroupUser{
		TgUserId:    fromUser.ID,
		ChatGroupId: chatGroup.Id,
	}

	_, err = chatGroupUserQuery.QueryByTgUserIdAndChatGroupId(db)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有找到记录 则注册
		chatGroupUser := &model.ChatGroupUser{
			TgUserId:    fromUser.ID,
			ChatGroupId: chatGroup.Id,
			Username:    fromUser.UserName,
			IsLeft:      0,
			Balance:     1000,
			CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
		}
		err := chatGroupUser.Create(db)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("创建用户信息异常")
		} else {
			msgConfig := tgbotapi.NewMessage(tgChatGroupId, "注册成功！奖励1000积分！")
			msgConfig.ReplyToMessageID = messageId
			_, err := sendMessage(bot, &msgConfig)
			blockedOrKicked(err, tgChatGroupId)
		}
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"TgUserId":    fromUser.ID,
			"ChatGroupId": chatGroup.Id,
			"err":         err,
		}).Error("查询群用户信息异常")
	} else {
		msgConfig := tgbotapi.NewMessage(tgChatGroupId, "请勿重复注册！")
		msgConfig.ReplyToMessageID = messageId
		_, err := sendMessage(bot, &msgConfig)
		blockedOrKicked(err, tgChatGroupId)
	}
}

func handleGroupReloadCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatId := message.Chat.ID
	messageId := message.MessageID

	// 重载群管理信息
	chatConfig := tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatId,
		}}

	administrators, err := bot.GetChatAdministrators(chatConfig)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatId": chatId,
			"err":    err,
		}).Error("无法获取管理员列表")
		return
	}

	ChatGroup, err := model.QueryChatGroupByTgChatId(db, chatId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.WithFields(logrus.Fields{
			"chatId": chatId,
		}).Error("未查询到群配置信息")
		return
	} else if err != nil {
		logrus.WithFields(logrus.Fields{
			"chatId": chatId,
			"err":    err,
		}).Error("群配置信息查询异常")
		return
	}

	botIsAdmin := false

	tx := db.Begin()

	// 删除该群关联的管理员记录
	model.DeleteChatGroupAdminByChatGroupId(tx, ChatGroup.Id)

	// 载入新的管理员信息
	for _, administrator := range administrators {
		user := administrator.User
		logrus.WithFields(logrus.Fields{
			"userName": user.UserName,
		}).Info("管理员信息")
		if user.UserName == bot.Self.UserName {
			botIsAdmin = true
		}
		err := model.CreateChatGroupAdmin(tx, &model.ChatGroupAdmin{
			ChatGroupId:   ChatGroup.Id,
			AdminTgUserId: user.ID,
			CreateTime:    time.Now().Format("2006-01-02 15:04:05"),
		})
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ChatGroupId":   ChatGroup.Id,
				"AdminTgUserId": user.ID,
				"err":           err,
			}).Error("初始化管理员信息异常")
			tx.Rollback()
			return
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		// 提交事务时出现异常，回滚事务
		tx.Rollback()
	}

	var sendMsg tgbotapi.MessageConfig
	if !botIsAdmin {
		sendMsg = tgbotapi.NewMessage(chatId, "❌请将我设置为管理员!")
	} else {
		sendMsg = tgbotapi.NewMessage(chatId, "✅重新载入成功!")
	}

	sendMsg.ReplyToMessageID = messageId
	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
}
