package logger

import (
	"bytes"
	"encoding/json"
	"strings"
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

func TestRegisterWriter(t *testing.T) {
	var buf bytes.Buffer
	unregister := RegisterWriter(&buf)

	Info("hello from test", "key", "value")

	unregister()

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output in subscriber, got empty string")
	}

	// The subscriber should receive JSON.
	var fields map[string]interface{}
	// There may be a trailing newline; trim it.
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &fields); err != nil {
		t.Fatalf("expected JSON output, got: %s (error: %v)", output, err)
	}

	if msg, ok := fields["msg"].(string); !ok || msg != "hello from test" {
		t.Fatalf("expected msg='hello from test', got: %v", fields["msg"])
	}
}

func TestRegisterWriterUnregister(t *testing.T) {
	var buf bytes.Buffer
	unregister := RegisterWriter(&buf)
	unregister()

	Info("should not appear")

	if buf.Len() != 0 {
		t.Fatalf("expected no output after unregister, got: %s", buf.String())
	}
}
