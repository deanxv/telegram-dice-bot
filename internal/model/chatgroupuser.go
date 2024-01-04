package model

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"telegram-dice-bot/internal/utils"
)

type ChatGroupUser struct {
	Id          string  `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	TgUserId    int64   `json:"tg_user_id" gorm:"type:bigint(20);not null"` // Telegram 用户ID
	ChatGroupId string  `json:"chat_group_id" gorm:"type:varchar(64);not null;"`
	IsLeft      int     `json:"is_left" gorm:"type:int(64);not null;"`      // 是否离开群组
	Username    string  `json:"username" gorm:"type:varchar(500);not null"` // Telegram 用户名
	Balance     float64 `json:"balance" gorm:"type:decimal(20, 2);not null"`
	SignInTime  string  `json:"sign_in_time" gorm:"type:varchar(500)"` // 签到时间
	CreateTime  string  `json:"create_time" gorm:"type:varchar(255);not null"`
}

func (c *ChatGroupUser) Create(db *gorm.DB) error {
	if c.Id == "" {
		id, err := utils.NextID()
		if err != nil {
			logrus.Error("SnowFlakeId create error")
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

func (c *ChatGroupUser) ListByTgUserId(db *gorm.DB) ([]*ChatGroupUser, error) {
	var chatGroupUsers []*ChatGroupUser
	result := db.Where("tg_user_id = ?", c.TgUserId).Order("create_time desc").Limit(100).Find(&chatGroupUsers)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroupUsers, nil
}

func (c *ChatGroupUser) ListByTgUserIdAndIsLeft(db *gorm.DB) ([]*ChatGroupUser, error) {
	var chatGroupUsers []*ChatGroupUser
	result := db.Where("tg_user_id = ? and is_left = ?", c.TgUserId, c.IsLeft).Order("create_time desc").Limit(100).Find(&chatGroupUsers)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroupUsers, nil
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

func (c *ChatGroupUser) QueryByIdAndChatGroupId(db *gorm.DB) (*ChatGroupUser, error) {
	var chatGroupUser *ChatGroupUser
	result := db.Where("id = ? AND chat_group_id = ?", c.Id, c.ChatGroupId).First(&chatGroupUser)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroupUser, nil
}
