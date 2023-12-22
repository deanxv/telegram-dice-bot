package enums

// GameplayStatus 代表枚举的自定义类型
type GameplayStatus struct {
	Value int
	Name  string
}

// 枚举映射
var GameplayStatusMap = make(map[int]GameplayStatus)

// 构造函数
func newGameplayStatus(value int, name string) GameplayStatus {
	enum := GameplayStatus{Value: value, Name: name}
	GameplayStatusMap[value] = enum
	return enum
}

// 使用构造函数定义枚举值
var (
	GameplayStatusOFF = newGameplayStatus(0, "⏹️已关闭")
	GameplayStatusON  = newGameplayStatus(1, "▶️已开启")
)

// GetGameplayStatus 通过 value 获取枚举项
func GetGameplayStatus(value int) (GameplayStatus, bool) {
	enum, ok := GameplayStatusMap[value]
	return enum, ok

}
