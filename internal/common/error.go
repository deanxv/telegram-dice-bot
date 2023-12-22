package common

import "fmt"

type HandleBetTextError struct {
	Msg  string
	Code int
}

// Error 方法使MyCustomError实现了error接口
func (e *HandleBetTextError) Error() string {
	return fmt.Sprintf("Error: %s, Code: %d", e.Msg, e.Code)
}
