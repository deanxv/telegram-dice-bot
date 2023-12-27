package bot

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/gorm"
	"log"
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
		log.Println("获取聊天成员异常:", err)
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
		//case "help":
		//	handleHelpCommand(bot, message)
	}
}

func handleMyHistoryCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	fromUser := message.From
	messageId := message.MessageID
	chatId := message.Chat.ID

	chatGroupUserQuery := &model.ChatGroupUser{
		// 查询用户信息
		TgUserId: fromUser.ID,
	}

	chatGroupUser, err := chatGroupUserQuery.QueryByTgUserId(db)
	if err != nil {
		log.Printf("查询异常 err %s", err.Error())
		return
	}
	// 查询下注记录

	betRecord := &model.BetRecord{ChatGroupUserId: chatGroupUser.Id}
	betRecords, err := betRecord.ListByChatGroupUserId(db)
	if err != nil {
		log.Printf("查询下注记录 err %s", err.Error())
		return
	}
	sendMsg := tgbotapi.NewMessage(chatId, "")
	sendMsg.ReplyToMessageID = messageId

	if len(betRecords) == 0 {
		// 下注记录为空
		sendMsg.Text = "您还没有下注记录哦!"
	} else if err != nil {
		log.Println("查询下注记录异常", err)
		return
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
					log.Printf("RecordId %v 快三下注记录查询异常", record.Id)
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
			blockedOrKicked(err, chatId)
			return
		}
		go func(messageID int) {
			time.Sleep(1 * time.Minute)
			deleteMsg := tgbotapi.NewDeleteMessage(chatId, messageID)
			_, err := bot.Request(deleteMsg)
			if err != nil {
				log.Println("删除消息异常:", err)
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

				log.Printf("Bot was added to a group: %s", message.Chat.Title)
				// 查找是否原来关联过该群
				_, err := model.QueryChatGroupByTgChatId(tx, chatId)
				if errors.Is(err, gorm.ErrRecordNotFound) {
					log.Printf("群TgChatId %v 该群未初始化过配置 ", chatId)

					// 初始化群配置
					chatGroupId, err := utils.NextID()
					if err != nil {
						log.Println("SnowFlakeId create error")
						return
					}

					chatGroup := &model.ChatGroup{
						Id:               chatGroupId,
						TgChatGroupTitle: chatTitle,
						TgChatGroupId:    chatId,
						GameplayType:     enums.QuickThere.Value,
						GameDrawCycle:    1,
						GameplayStatus:   0,
						ChatGroupStatus:  enums.Normal.Value,
						CreateTime:       time.Now().Format("2006-01-02 15:04:05"),
					}
					err = chatGroup.Create(tx)

					// 初始化快三配置
					quickThereConfig := &model.QuickThereConfig{
						ChatGroupId: chatGroupId,
						SimpleOdds:  2,
						TripletOdds: 10,
						CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
					}

					err = quickThereConfig.Create(tx)

					// 提交事务
					if err := tx.Commit().Error; err != nil {
						// 提交事务时出现异常，回滚事务
						tx.Rollback()
					}

					if err != nil {
						log.Printf("群TgChatId %v 初始化群配置异常 %s", chatId, err.Error())
						return
					}
					log.Printf("群TgChatId %v 该群初始化配置成功 ", chatId)
					return
				} else if err != nil {
					log.Printf("群TgChatId %v 查找异常 %s", chatId, err.Error())
					return
				} else {
					log.Printf("群TgChatId %v 该群已被初始化过配置", chatId)
					return
				}

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
	if err != nil {
		log.Printf("TgChatGroupId %v 群配置查询异常", tgChatGroupId)
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
				log.Println("发送消息异常:", err)
				blockedOrKicked(err, tgChatGroupId)
			}
		} else if err != nil {
			log.Println("处理下注信息异常:", err)
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
			log.Println("功能未开启提示消息异常:", err)
			blockedOrKicked(err, tgChatGroupId)
			return false, err
		}
		return false, nil
	}

	// 获取当前进行的期号
	redisKey := fmt.Sprintf(RedisCurrentIssueNumberKey, chatGroup.Id)
	issueNumberResult := redisDB.Get(redisDB.Context(), redisKey)
	if errors.Is(issueNumberResult.Err(), redis.Nil) || issueNumberResult == nil {
		log.Printf("键 %s 不存在", redisKey)
		replyMsg := tgbotapi.NewMessage(tgChatGroupId, "当前暂无开奖活动!")
		replyMsg.ReplyToMessageID = messageId
		_, sendErr := bot.Send(replyMsg)
		blockedOrKicked(sendErr, tgChatGroupId)
		return false, nil
	} else if issueNumberResult.Err() != nil {
		log.Println("获取值时发生异常:", issueNumberResult.Err())
		return false, nil
	}

	issueNumber, _ := issueNumberResult.Result()

	// 存储下注记录到数据库，并扣除用户余额
	b, err := storeQuickThereBetRecord(bot, chatGroup, message, &model.QuickThereBetRecord{
		IssueNumber: issueNumber,
		BetType:     betType,
		BetAmount:   float64(betAmount),
	})

	if !b && err != nil {
		log.Println("存储下注记录异常:", err)
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
			log.Println("发送注册提示消息异常:", sendErr)
			blockedOrKicked(sendErr, chatId)
			return false, sendErr
		}
		return false, nil
	} else if err != nil {
		log.Printf("查询异常 err %s", err.Error())
		return false, err
	} else {
		// 检查用户余额是否足够
		if chatGroupUser.Balance < quickThereBetRecord.BetAmount {
			// 用户不存在，发送注册提示
			balanceInsufficientMsg := tgbotapi.NewMessage(chatId, "您的余额不足!")
			balanceInsufficientMsg.ReplyToMessageID = messageId
			_, err := bot.Send(balanceInsufficientMsg)
			if err != nil {
				log.Println("您的余额不足提示异常:", err)
				blockedOrKicked(err, chatId)
				return false, err
			} else {
				return false, nil
			}
		}

		// 扣除用户余额
		chatGroupUser.Balance -= quickThereBetRecord.BetAmount
		result := tx.Save(&chatGroupUser)
		if result.Error != nil {
			log.Println("扣除用户余额异常:", result.Error)
			tx.Rollback()
			return false, result.Error
		}
		currentTime := time.Now().Format("2006-01-02 15:04:05")

		// 映射下注类型
		betType, b := enums.GetGameLotteryTypeForName(quickThereBetRecord.BetType)
		if !b {
			log.Printf("该下注类型映射异常 betType %s", quickThereBetRecord.BetType)
			return false, errors.New("该下注类型映射异常")
		}

		id, err := utils.NextID()
		if err != nil {
			log.Println("SnowFlakeId create error")
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
			log.Println("保存下注记录异常:", err)
			// 如果保存下注记录失败，需要返还用户余额
			chatGroupUser.Balance += quickThereBetRecord.BetAmount
			tx.Save(&user)
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
			log.Println("保存快三下注记录异常:", result.Error)
			// 如果保存下注记录失败，需要返还用户余额
			chatGroupUser.Balance += quickThereBetRecord.BetAmount
			tx.Save(&user)
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
		log.Printf("TgChatGroupId %v 群配置查询异常", tgChatGroupId)
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
		log.Printf("查询异常 err %s", err.Error())
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
				log.Println("删除消息异常:", err)
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
		log.Printf("TgChatGroupId %v 群配置查询异常", tgChatGroupId)
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
		log.Println("查询异常:", err)
	} else {
		// 获取用户对应的互斥锁
		userLockKey := fmt.Sprintf(ChatGroupUserLockKey, message.Chat.ID, message.From.ID)
		userLock := getUserLock(userLockKey)
		userLock.Lock()
		defer userLock.Unlock()

		if chatGroupUser.SignInTime != "" {
			signInTime, err := time.Parse("2006-01-02 15:04:05", chatGroupUser.SignInTime)
			if err != nil {
				log.Println("时间解析异常:", err)
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
			log.Println("保存用户信息异常:", result.Error)
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
		log.Printf("TgChatGroupId %s 群配置查询异常", tgChatGroupId)
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
			Balance:     1000,
			CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
		}
		err := chatGroupUser.Create(db)
		if err != nil {
			log.Println("用户注册异常:", err)
		} else {
			msgConfig := tgbotapi.NewMessage(tgChatGroupId, "注册成功！奖励1000积分！")
			msgConfig.ReplyToMessageID = messageId
			_, err := sendMessage(bot, &msgConfig)
			blockedOrKicked(err, tgChatGroupId)
		}
	} else if err != nil {
		log.Printf("查询异常 err %s", err.Error())
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
		log.Println("无法获取管理员列表:", err)
		return
	}

	ChatGroup, err := model.QueryChatGroupByTgChatId(db, chatId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("群TgChatId %v 该群未初始化过配置 ", chatId)
		return
	} else if err != nil {
		log.Printf("群TgChatId %v 查找异常 %s", chatId, err.Error())
		return
	}

	botIsAdmin := false

	tx := db.Begin()

	// 删除该群关联的管理员记录
	model.DeleteChatGroupAdminByChatGroupId(tx, ChatGroup.Id)

	// 载入新的管理员信息
	for _, administrator := range administrators {
		user := administrator.User
		log.Println(user.UserName)
		if user.UserName == bot.Self.UserName {
			botIsAdmin = true
		}
		err := model.CreateChatGroupAdmin(tx, &model.ChatGroupAdmin{
			ChatGroupId:   ChatGroup.Id,
			AdminTgUserId: user.ID,
		})
		if err != nil {
			log.Printf("群TgChatId %v 初始化管理员信息异常", chatId)
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
