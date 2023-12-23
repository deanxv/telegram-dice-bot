package common

import "fmt"

type HandleBetTextError struct {
	Msg  string
	Code int
}

func (e *HandleBetTextError) Error() string {
	return fmt.Sprintf("Error: %s, Code: %d", e.Msg, e.Code)
}
