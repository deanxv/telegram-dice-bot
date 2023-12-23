package enums

// BetResultType 代表枚举的自定义类型
type BetResultType struct {
	Value int
	Name  string
}

// 枚举映射
var BetResultTypeMap = make(map[int]BetResultType)

// 构造函数
func newBetResultType(value int, name string) BetResultType {
	enum := BetResultType{Value: value, Name: name}
	BetResultTypeMap[value] = enum
	return enum
}

// 使用构造函数定义枚举值
var (
	Loss = newBetResultType(0, "输")
	Win  = newBetResultType(1, "赢")
)

// GetBetResultType 通过 value 获取枚举项
func GetBetResultType(value int) (BetResultType, bool) {
	enum, ok := BetResultTypeMap[value]
	return enum, ok

}
