package model

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"telegram-dice-bot/internal/utils"
)

type ChatGroup struct {
	Id               string `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	TgChatGroupTitle string `json:"tg_chat_group_title" gorm:"type:varchar(900);not null"`
	TgChatGroupId    int64  `json:"tg_chat_group_id" gorm:"type:bigint(20);not null"`
	GameplayType     string `json:"gameplay_type" gorm:"type:varchar(255);not null"`
	GameDrawCycle    int    `json:"game_draw_cycle" gorm:"type:int(11);not null"`
	GameplayStatus   int    `json:"gameplay_status" gorm:"type:int(11);not null"`
	ChatGroupStatus  string `json:"chat_group_status" gorm:"type:varchar(255);not null"`
	CreateTime       string `json:"create_time" gorm:"type:varchar(255);not null"`
}

func (c *ChatGroup) Create(db *gorm.DB) error {
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

func UpdateChatGroupStatusByTgChatId(db *gorm.DB, chatGroup *ChatGroup) (*ChatGroup, error) {
	result := db.Model(&ChatGroup{}).Where("tg_chat_group_id = ?", chatGroup.TgChatGroupId).Update("chat_group_status", chatGroup.ChatGroupStatus)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroup, nil
}

func (c *ChatGroup) UpdateGameplayStatusAndChatGroupStatusByTgChatId(db *gorm.DB) (*ChatGroup, error) {

	result := db.Model(&ChatGroup{}).Where("tg_chat_group_id = ?", c.TgChatGroupId).
		Updates(map[string]interface{}{
			"ChatGroupStatus": c.ChatGroupStatus,
			"GameplayStatus":  c.GameplayStatus,
		})

	if result.Error != nil {
		return nil, result.Error
	}
	return c, nil
}

func QueryChatGroupByTgChatId(db *gorm.DB, tgChatId int64) (*ChatGroup, error) {
	var chatGroup *ChatGroup
	result := db.Where("tg_chat_group_id = ?", tgChatId).First(&chatGroup)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroup, nil
}

func QueryChatGroupById(db *gorm.DB, id string) (*ChatGroup, error) {
	var chatGroup *ChatGroup
	result := db.First(&chatGroup, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroup, nil
}

func UpdateChatGroupGameplayTypeById(db *gorm.DB, chatGroup *ChatGroup) error {
	result := db.Model(&chatGroup).Select("gameplay_type").Updates(chatGroup)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (c *ChatGroup) UpdateGameDrawCycleById(db *gorm.DB) error {
	result := db.Model(&c).Select("game_draw_cycle").Updates(c)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (c *ChatGroup) UpdateChatGroupStatusById(db *gorm.DB) error {
	result := db.Model(&c).Select("gameplay_status").Updates(c)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (c *ChatGroup) ListByGameplayStatus(db *gorm.DB) ([]*ChatGroup, error) {
	var chatGroups []*ChatGroup

	result := db.Where("gameplay_status = ? and chat_group_status = 'NORMAL'", c.GameplayStatus).Find(&chatGroups)
	if result.Error != nil {
		return nil, result.Error
	}

	return chatGroups, nil
}

func ListChatGroupByIds(db *gorm.DB, chatGroupIds []string) ([]*ChatGroup, error) {
	var chatGroups []*ChatGroup

	result := db.Where("id in ? ", chatGroupIds).Find(&chatGroups)
	if result.Error != nil {
		return nil, result.Error
	}

	return chatGroups, nil
}
