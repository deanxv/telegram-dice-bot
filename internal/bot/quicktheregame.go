package bot

import (
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"telegram-dice-bot/internal/enums"
	"telegram-dice-bot/internal/model"
	"time"
)

func quickThereTask(bot *tgbotapi.BotAPI, group *model.ChatGroup, issueNumber string) (nextIssueNumber string, err error) {

	redisKey := fmt.Sprintf(RedisCurrentIssueNumberKey, group.Id)
	// 删除当前期号和对话ID
	err = redisDB.Del(redisDB.Context(), redisKey).Err()
	if err != nil {
		log.Println("删除当前期号和对话ID异常:", err)
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
		log.Printf("issueNumber %s 开奖结果消息格式化异常", issueNumber)
	}

	lotteryRecord := &model.QuickThereLotteryRecord{
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
	err = lotteryRecord.Create(db)
	if err != nil {
		log.Printf("开奖记录插入异常 group.Id %v issueNumber %v", group.Id, issueNumber)
		return "", err
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("开奖历史", "betting_history"),
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
		log.Println("存储新期号和对话ID异常:", err)
	}

	// 遍历下注记录，计算竞猜结果
	go func() {
		// 获取所有参与竞猜的用户下注记录
		//betRecords, err := model.GetBetRecordsByChatIDAndIssue(db, chatID, issueNumber)
		//if err != nil {
		//	log.Println("获取用户下注记录异常:", err)
		//	return
		//}
		//// 获取当前期数开奖结果
		//var lotteryRecord model.LotteryRecord
		//db.Where("issue_number = ? AND chat_id = ?", issueNumber, chatID).First(&lotteryRecord)
		//
		//for _, betRecord := range betRecords {
		//	// 更新用户余额
		//	updateBalance(betRecord, &lotteryRecord)
		//}
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
			log.Println("发送骰子消息异常:", err)
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
		bigOrSmall = enums.SMALL.Value
	} else {
		bigOrSmall = enums.BIG.Value
	}

	if count%2 == 1 {
		singleOrDouble = enums.SINGLE.Value
	} else {
		singleOrDouble = enums.DOUBLE.Value
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
		log.Printf("singleOrDouble %s 开奖结果映射异常", singleOrDouble)
		return "", errors.New("开奖结果映射异常")
	}
	bigOrSmallType, b := enums.GetGameLotteryType(bigOrSmall)
	if !b {
		log.Printf("bigOrSmall %s 开奖结果映射异常", bigOrSmall)
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
