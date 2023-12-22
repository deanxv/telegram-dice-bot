package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
	"telegram-dice-bot/internal/common"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
)

// 处理私有Command消息
func handlePrivateCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	switch message.Command() {
	case "start":
		handlePrivateStartCommand(bot, message)
	}
}

func handlePrivateStartCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatId := message.Chat.ID
	userId := message.From.ID
	member, err := getChatMember(bot, chatId, userId)

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
		log.Printf("键 %s 不存在 [当前机器人暂无对话状态]", redisKey)
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
		}

	}
}

func updateGameDrawCycle(bot *tgbotapi.BotAPI, message *tgbotapi.Message, botPrivateChatCache *common.BotPrivateChatCache) {
	text := message.Text
	tgUserId := message.From.ID
	chatId := message.Chat.ID

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

	sendMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf("设置成功!当前群组开奖周期为%v分钟。", drawCycle))

	_, err = sendMessage(bot, &sendMsg)
	if err != nil {
		blockedOrKicked(err, chatId)
		return
	}
	// 删除bot与当前对话人的cache
	redisKey := fmt.Sprintf(RedisBotPrivateChatCacheKey, tgUserId)
	redisDB.Del(redisDB.Context(), redisKey)
}
