package enums

// BotPrivateChatStatus 代表枚举的自定义类型
type BotPrivateChatStatus struct {
	Value string
	Name  string
}

// 枚举映射
var BotPrivateChatStatusMap = make(map[string]BotPrivateChatStatus)

// 构造函数
func newBotPrivateChatStatus(value string, name string) BotPrivateChatStatus {
	enum := BotPrivateChatStatus{Value: value, Name: name}
	BotPrivateChatStatusMap[value] = enum
	return enum
}

// 使用构造函数定义枚举值等
var (
	WaitGameDrawCycle         = newBotPrivateChatStatus("WAIT_GAME_DRAW_CYCLE", "开奖周期设置")
	WaitQueryUser             = newBotPrivateChatStatus("WAIT_QUERY_USER", "查询用户信息")
	WaitUpdateUserBalance     = newBotPrivateChatStatus("WAIT_UPDATE_USER_BALANCE", "修改用户积分")
	WaitQuickThereSimpleOdds  = newBotPrivateChatStatus("WAIT_QUICK_THERE_SIMPLE_ODDS", "快三简易倍率")
	WaitQuickThereTripletOdds = newBotPrivateChatStatus("WAIT_QUICK_THERE_TRIPLET_ODDS", "快三豹子倍率")
	WaitTransferBalance       = newBotPrivateChatStatus("WAIT_TRANSFER_BALANCE", "转让用户积分")
)

// GetBotPrivateChatStatus 通过 value 获取枚举项
func GetBotPrivateChatStatus(value string) (BotPrivateChatStatus, bool) {
	enum, ok := BotPrivateChatStatusMap[value]
	return enum, ok

}
