package model

import (
	"gorm.io/gorm"
	"log"
	"telegram-dice-bot/internal/utils"
)

type ChatGroupUser struct {
	Id          string  `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	TgUserId    int64   `json:"tg_user_id" gorm:"type:bigint(20);not null"` // Telegram 用户ID
	ChatGroupId string  `json:"chat_group_id" gorm:"type:varchar(64);not null;index"`
	Username    string  `json:"username" gorm:"type:varchar(500);not null"` // Telegram 用户名
	Balance     float64 `json:"balance" gorm:"type:decimal(20, 2);not null"`
	SignInTime  string  `json:"sign_in_time" gorm:"type:varchar(500)"` // 签到时间
	CreateTime  string  `json:"create_time" gorm:"type:varchar(255);not null"`
}

func (c *ChatGroupUser) Create(db *gorm.DB) error {
	if c.Id == "" {
		id, err := utils.NextID()
		if err != nil {
			log.Println("SnowFlakeId create error")
			return err
		}
		c.Id = id
	}
	result := db.Create(c)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (c *ChatGroupUser) QueryByTgUserIdAndChatGroupId(db *gorm.DB) (*ChatGroupUser, error) {
	var chatGroupUser *ChatGroupUser
	result := db.Where("tg_user_id = ? AND chat_group_id = ?", c.TgUserId, c.ChatGroupId).First(&chatGroupUser)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroupUser, nil
}

func (c *ChatGroupUser) QueryByUsernameAndChatGroupId(db *gorm.DB) (*ChatGroupUser, error) {
	var chatGroupUser *ChatGroupUser
	result := db.Where("username = ? AND chat_group_id = ?", c.Username, c.ChatGroupId).First(&chatGroupUser)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroupUser, nil
}

func (c *ChatGroupUser) QueryById(db *gorm.DB) (*ChatGroupUser, error) {
	var chatGroupUser *ChatGroupUser
	result := db.First(&chatGroupUser, c.Id)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroupUser, nil
}
