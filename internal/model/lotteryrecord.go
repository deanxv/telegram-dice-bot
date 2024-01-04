package model

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"telegram-dice-bot/internal/utils"
)

type LotteryRecord struct {
	Id           string `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	ChatGroupId  string `json:"chat_group_id" gorm:"type:varchar(64);not null"`
	IssueNumber  string `json:"issue_number" gorm:"type:varchar(64);not null"`
	GameplayType string `json:"gameplay_type" gorm:"type:varchar(255);not null"`
	CreateTime   string `json:"create_time" gorm:"type:varchar(255);not null"`
}

func (c *LotteryRecord) Create(db *gorm.DB) error {
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

func (c *LotteryRecord) ListByChatGroupId(db *gorm.DB) ([]*LotteryRecord, error) {
	var lotteryRecords []*LotteryRecord

	result := db.Where("chat_group_id = ?", c.ChatGroupId).Order("issue_number desc").Limit(10).Find(&lotteryRecords)
	if result.Error != nil {
		return nil, result.Error
	}

	return lotteryRecords, nil
}
