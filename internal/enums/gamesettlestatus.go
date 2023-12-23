package enums

// GameSettleStatus 代表枚举的自定义类型
type GameSettleStatus struct {
	Value int
	Name  string
}

// 枚举映射
var GameSettleStatusMap = make(map[int]GameSettleStatus)

// 构造函数
func newGameSettleStatus(value int, name string) GameSettleStatus {
	enum := GameSettleStatus{Value: value, Name: name}
	GameSettleStatusMap[value] = enum
	return enum
}

// 使用构造函数定义枚举值
var (
	Unsettled = newGameSettleStatus(0, "未结算")
	Settled   = newGameSettleStatus(1, "已开奖")
)

// GetGameSettleStatus 通过 value 获取枚举项
func GetGameSettleStatus(value int) (GameSettleStatus, bool) {
	enum, ok := GameSettleStatusMap[value]
	return enum, ok

}
