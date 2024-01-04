package model

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"telegram-dice-bot/internal/utils"
)

type ChatGroupAdmin struct {
	Id            string `json:"id" gorm:"type:varchar(64);not null;primaryKey"`
	ChatGroupId   string `json:"chat_group_id" gorm:"type:varchar(64);not null"`
	AdminTgUserId int64  `json:"admin_tg_user_id" gorm:"type:bigint(20);not null"` // Telegram 用户ID
	CreateTime    string `json:"create_time" gorm:"type:varchar(255);not null"`
}

func CreateChatGroupAdmin(db *gorm.DB, chatGroupAdmin *ChatGroupAdmin) error {
	id, err := utils.NextID()
	if err != nil {
		logrus.Error("SnowFlakeId create error")
		return err
	}
	chatGroupAdmin.Id = id
	result := db.Create(chatGroupAdmin)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func QueryChatGroupAdminByChatGroupIdAndTgUserId(db *gorm.DB, chatGroupId string, adminTgUserId int64) (*ChatGroupAdmin, error) {
	var chatGroupAdmin *ChatGroupAdmin
	result := db.Where("chat_group_id = ? and admin_tg_user_id = ?", chatGroupId, adminTgUserId).First(&chatGroupAdmin)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatGroupAdmin, nil
}

func ListChatGroupAdminByAdminTgUserId(db *gorm.DB, adminTgUserId int64) ([]*ChatGroupAdmin, error) {
	var chatGroupAdmins []*ChatGroupAdmin

	result := db.Model(&ChatGroupAdmin{}).
		Select("chat_group_admins.*").
		Joins("left join chat_groups on chat_groups.id = chat_group_admins.chat_group_id").
		Where("chat_group_admins.admin_tg_user_id = ? and chat_groups.chat_group_status = 'NORMAL'", adminTgUserId).
		Limit(100).
		Find(&chatGroupAdmins)

	if result.Error != nil {
		return nil, result.Error
	}

	return chatGroupAdmins, nil
}

func DeleteChatGroupAdminByChatGroupId(db *gorm.DB, chatGroupId string) {
	db.Where("chat_group_id = ?", chatGroupId).Delete(&ChatGroupAdmin{})
}

func (c *ChatGroupAdmin) DeleteByChatGroupIdAndAdminTgUserId(db *gorm.DB) {
	db.Where("chat_group_id = ? and admin_tg_user_id = ?", c.ChatGroupId, c.AdminTgUserId).Delete(&ChatGroupAdmin{})
}
