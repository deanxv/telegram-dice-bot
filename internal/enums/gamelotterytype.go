package enums

// GameLotteryType 代表枚举的自定义类型
type GameLotteryType struct {
	Value string
	Name  string
}

// 枚举映射
var GameLotteryTypeMap = make(map[string]GameLotteryType)

var GameLotteryTypeMapForName = make(map[string]GameLotteryType)

// 构造函数
func newGameLotteryType(value string, name string) GameLotteryType {
	enum := GameLotteryType{Value: value, Name: name}
	GameLotteryTypeMap[value] = enum

	GameLotteryTypeMapForName[name] = enum

	return enum
}

// 使用构造函数定义枚举值等
var (
	Big     = newGameLotteryType("BIG", "大")
	Small   = newGameLotteryType("SMALL", "小")
	Single  = newGameLotteryType("SINGLE", "单")
	Double  = newGameLotteryType("DOUBLE", "双")
	Triplet = newGameLotteryType("TRIPLET", "豹子")
)

// GetGameLotteryType 通过 value 获取枚举项
func GetGameLotteryType(value string) (GameLotteryType, bool) {
	enum, ok := GameLotteryTypeMap[value]
	return enum, ok
}

// GetGameLotteryType 通过 name 获取枚举项
func GetGameLotteryTypeForName(name string) (GameLotteryType, bool) {
	enum, ok := GameLotteryTypeMapForName[name]
	return enum, ok
}
