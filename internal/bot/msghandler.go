package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleMessage 处理传入的消息。
func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {

	if message.IsCommand() {
		// cmd
		if message.Chat.IsSuperGroup() || message.Chat.IsGroup() {
			handleGroupCommand(bot, message)
		} else if message.Chat.IsPrivate() {
			handlePrivateCommand(bot, message)
		}
	} else {
		// 非cmd
		if message.Chat.IsSuperGroup() || message.Chat.IsGroup() {
			go handleGroupMigrateFromChatID(bot, message)
			go handleGroupNewChatTitle(bot, message)
			go handleGroupNewMembers(bot, message)
			go handleGroupLeftChatMember(bot, message)
			go handleGroupText(bot, message)
		} else if message.Chat.IsPrivate() {
			handlePrivateText(bot, message)
		}
	}
}
