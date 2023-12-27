package model

import (
	"gorm.io/gorm"
	"log"
	"telegram-dice-bot/internal/utils"
)

type BetRecord struct {
	Id              string `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	ChatGroupUserId string `json:"chat_group_user_id" gorm:"type:varchar(64);not null"` // 用户ID
	ChatGroupId     string `json:"chat_group_id" gorm:"type:varchar(64);not null;"`
	GameplayType    string `json:"gameplay_type" gorm:"type:varchar(255);not null"`
	IssueNumber     string `json:"issue_number" gorm:"type:varchar(64);not null"`
	UpdateTime      string `json:"update_time" gorm:"type:varchar(255);not null"`
	CreateTime      string `json:"create_time" gorm:"type:varchar(255);not null"`
}

func (c *BetRecord) Create(db *gorm.DB) error {
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
