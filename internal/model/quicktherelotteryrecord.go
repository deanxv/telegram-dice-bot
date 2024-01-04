package model

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"telegram-dice-bot/internal/utils"
)

type QuickThereLotteryRecord struct {
	Id           string `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	ChatGroupId  string `json:"chat_group_id" gorm:"type:varchar(64);not null"`
	IssueNumber  string `json:"issue_number" gorm:"type:varchar(64);not null"`
	ValueA       int    `json:"value_a" gorm:"type:int(11);not null"`
	ValueB       int    `json:"value_b" gorm:"type:int(11);not null"`
	ValueC       int    `json:"value_c" gorm:"type:int(11);not null"`
	Total        int    `json:"total" gorm:"type:int(11);not null"`
	SingleDouble string `json:"single_double" gorm:"type:varchar(255);not null"`
	BigSmall     string `json:"big_small" gorm:"type:varchar(255);not null"`
	Triplet      int    `json:"triplet" gorm:"type:int(11);not null"`
	CreateTime   string `json:"create_time" gorm:"type:varchar(255);not null"`
}

func (c *QuickThereLotteryRecord) Create(db *gorm.DB) error {
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

func (c *QuickThereLotteryRecord) QueryByIssueNumberAndChatGroupId(db *gorm.DB) (*QuickThereLotteryRecord, error) {
	var quickThereLotteryRecord *QuickThereLotteryRecord

	result := db.Where("issue_number = ? and chat_group_id = ?", c.IssueNumber, c.ChatGroupId).First(&quickThereLotteryRecord)
	if result.Error != nil {
		return nil, result.Error
	}

	return quickThereLotteryRecord, nil
}

func (c *QuickThereLotteryRecord) QueryById(db *gorm.DB) (*QuickThereLotteryRecord, error) {
	var quickThereLotteryRecord *QuickThereLotteryRecord
	result := db.First(&quickThereLotteryRecord, c.Id)
	if result.Error != nil {
		return nil, result.Error
	}
	return quickThereLotteryRecord, nil
}
