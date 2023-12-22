package enums

// GameplayType 代表枚举的自定义类型
type GameplayType struct {
	Value string
	Name  string
}

// 枚举映射
var GameplayTypeMap = make(map[string]GameplayType)

// 构造函数
func newGameplayType(value string, name string) GameplayType {
	enum := GameplayType{Value: value, Name: name}
	GameplayTypeMap[value] = enum
	return enum
}

// 使用构造函数定义枚举值
var (
	QuickThere = newGameplayType("QUICK_THERE", "经典快三")
	//_          = newGameplayType("UNDEFINED1", "未定义玩法1")
	//_          = newGameplayType("UNDEFINED2", "未定义玩法2")
)

// GetGameplayType 通过 value 获取枚举项
func GetGameplayType(value string) (GameplayType, bool) {
	enum, ok := GameplayTypeMap[value]
	return enum, ok

}
