package enums

// CallbackPrefix 代表枚举的自定义类型
type CallbackPrefix struct {
	Value string
	Name  string
}

// 枚举映射
var CallbackPrefixMap = make(map[string]CallbackPrefix)

// 构造函数
func newCallbackPrefix(value string, name string) CallbackPrefix {
	enum := CallbackPrefix{Value: value, Name: name}
	CallbackPrefixMap[value] = enum
	return enum
}

// 使用构造函数定义枚举值
var (
	CallbackMainMenu                    = newCallbackPrefix("main_menu", "主菜单")
	CallbackJoinedGroup                 = newCallbackPrefix("joined_group", "加入的群")
	CallbackAdminGroup                  = newCallbackPrefix("admin_group", "管理的群")
	CallbackAddAdminGroup               = newCallbackPrefix("add_admin_group", "添加管理的群")
	CallbackAlreadyInvited              = newCallbackPrefix("already_invited", "已经邀请入群")
	CallbackAlreadyReload               = newCallbackPrefix("already_reload", "群已经重新载入")
	CallbackChatGroupConfig             = newCallbackPrefix("chat_group_config?", "群配置")
	CallbackGameplayType                = newCallbackPrefix("gameplay_type?", "游戏类型")
	CallbackUpdateGameplayType          = newCallbackPrefix("update_gameplay_type?", "更新游戏类型")
	CallbackUpdateQuickThereSimpleOdds  = newCallbackPrefix("update_q_t_simple_odds?", "更新快三简易倍率")
	CallbackUpdateQuickThereTripletOdds = newCallbackPrefix("update_q_t_triplet_odds?", "更新快三豹子倍率")
	CallbackUpdateGameplayStatus        = newCallbackPrefix("update_gameplay_status?", "更新游戏类型状态")
	CallbackUpdateGameDrawCycle         = newCallbackPrefix("update_game_draw_cycle?", "更新游戏开奖周期")
	CallbackQueryChatGroupUser          = newCallbackPrefix("query_chat_group_user?", "查询群用户信息")
	CallbackUpdateChatGroupUserBalance  = newCallbackPrefix("update_chat_group_user_balance?", "更新用户积分")
	CallbackLotteryHistory              = newCallbackPrefix("lottery_history", "开奖历史")
	CallbackChatGroupInfo               = newCallbackPrefix("chat_group_info?", "群详情信息")
	CallbackTransferBalance             = newCallbackPrefix("transfer_balance?", "转让积分(用户)")
	CallbackExitGroup                   = newCallbackPrefix("exit_group?", "退出群聊")
	CallbackAdminExitGroup              = newCallbackPrefix("admin_exit_group?", "退出群聊")
)

// GetCallbackPrefix 通过 value 获取枚举项
func GetCallbackPrefix(value string) (CallbackPrefix, bool) {
	enum, ok := CallbackPrefixMap[value]
	return enum, ok

}
