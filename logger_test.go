package logger

import (
	"testing"
)

type TestError struct {
}

func (t TestError) Error() string {
	return "This is a Test Error"
}

func TestLogger(t *testing.T) {
	e := TestError{}
	err := e.Error()
	Error(err,
		"ip", "1.2.3.4",
		"test", "abcdef1234")
}
