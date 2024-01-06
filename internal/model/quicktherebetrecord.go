package model

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"telegram-dice-bot/internal/utils"
)

type QuickThereBetRecord struct {
	Id              string  `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	ChatGroupUserId string  `json:"chat_group_user_id" gorm:"type:varchar(64);not null"` // 用户ID
	ChatGroupId     string  `json:"chat_group_id" gorm:"type:varchar(64);not null;"`
	IssueNumber     string  `json:"issue_number" gorm:"type:varchar(64);not null"`
	BetType         string  `json:"bet_type" gorm:"type:varchar(64);not null"`               // 下注类型
	BetAmount       float64 `json:"bet_amount" gorm:"type:decimal(20, 2);not null"`          // 下注金额
	SettleStatus    int     `json:"settle_status" gorm:"type:int(11);not null"`              // 结算状态
	BetResultType   *int    `json:"bet_result_type" gorm:"type:int(11);default:null"`        // 下注结果输赢
	BetResultAmount string  `json:"bet_result_amount" gorm:"type:varchar(255);default:null"` // 下注结果
	UpdateTime      string  `json:"update_time" gorm:"type:varchar(255);not null"`
	CreateTime      string  `json:"create_time" gorm:"type:varchar(255);not null"`
}

func (c *QuickThereBetRecord) Create(db *gorm.DB) error {
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

func (c *QuickThereBetRecord) ListByChatGroupIdAndIssueNumber(db *gorm.DB) ([]*QuickThereBetRecord, error) {
	var quickThereBetRecord []*QuickThereBetRecord

	result := db.Where("chat_group_id = ? and issue_number = ?", c.ChatGroupId, c.IssueNumber).Find(&quickThereBetRecord)
	if result.Error != nil {
		return nil, result.Error
	}

	return quickThereBetRecord, nil
}

func (c *QuickThereBetRecord) QueryById(db *gorm.DB) (*QuickThereBetRecord, error) {
	var quickThereBetRecord *QuickThereBetRecord
	result := db.First(&quickThereBetRecord, c.Id)
	if result.Error != nil {
		return nil, result.Error
	}
	return quickThereBetRecord, nil
}
