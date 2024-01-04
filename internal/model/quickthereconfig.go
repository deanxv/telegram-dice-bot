package model

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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

func (c *QuickThereConfig) UpdateSimpleOddsByChatGroupId(db *gorm.DB) error {
	result := db.Model(&QuickThereConfig{}).Where("chat_group_id = ?", c.ChatGroupId).Update("simple_odds", c.SimpleOdds)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (c *QuickThereConfig) UpdateTripletOddsByChatGroupId(db *gorm.DB) error {
	result := db.Model(&QuickThereConfig{}).Where("chat_group_id = ?", c.ChatGroupId).Update("triplet_odds", c.TripletOdds)
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
