package enums

// GroupStatus 代表枚举的自定义类型
type GroupStatus struct {
	Value string
	Name  string
}

// 枚举映射
var GroupStatusMap = make(map[string]GroupStatus)

// 构造函数
func newGroupStatus(value string, name string) GroupStatus {
	enum := GroupStatus{Value: value, Name: name}
	GroupStatusMap[value] = enum
	return enum
}

// 使用构造函数定义枚举值
var (
	Normal = newGroupStatus("NORMAL", "正常")
	Kicked = newGroupStatus("KICKED", "踢出")
)

// GetGroupStatus 通过 value 获取枚举项
func GetGroupStatus(value string) (GroupStatus, bool) {
	enum, ok := GroupStatusMap[value]
	return enum, ok

}
