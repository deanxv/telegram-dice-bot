package model

import (
	"gorm.io/gorm"
	"log"
	"telegram-dice-bot/internal/utils"
)

type QuickThereConfig struct {
	Id          string  `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	ChatGroupId string  `json:"chat_group_id" gorm:"type:varchar(64);not null"`
	SimpleOdds  float64 `json:"simple_odds" gorm:"decimal(5, 2);not null"`
	TripletOdds float64 `json:"triplet_odds" gorm:"decimal(5, 2);not null"`
	CreateTime  string  `json:"create_time" gorm:"type:varchar(255);not null"`
}

func (c *QuickThereConfig) Create(db *gorm.DB) error {
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

func QueryQuickThereConfigByChatGroupId(db *gorm.DB, chatGroupId string) (*QuickThereConfig, error) {
	var QuickThereConfig *QuickThereConfig
	result := db.Where("chat_group_id = ?", chatGroupId).First(&QuickThereConfig)
	if result.Error != nil {
		return nil, result.Error
	}
	return QuickThereConfig, nil
}
