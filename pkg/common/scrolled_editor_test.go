package common

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestParsingLine(t *testing.T) {

	line := CreateLineWithExtra("This is a test line", "Extra info")

	l, extra := ParseLineContent(&line)
	fmt.Printf("Line: %s, Extra: %s\n", *l, *extra)

	if *l != "This is a test line" {
		t.Errorf("Expected line 'This is a test line', got '%s'", *l)
	}
	if *extra != "Extra info" {
		t.Errorf("Expected extra 'Extra info', got '%s'", *extra)
	}

	line = CreateLineWithExtraFullFormat("test line", "Extra")
	l, extra = ParseLineContent(&line)
	fmt.Printf("Line: %s, Extra: %s\n", *l, *extra)

	if *l != "test line" {
		t.Errorf("Expected line 'test line', got '%s'", *l)
	}
	if *extra != "Extra" {
		t.Errorf("Expected extra 'Extra', got '%s'", *extra)
	}

	line = "this is test line without extra"
	l, extra = ParseLineContent(&line)

	if *l != line {
		t.Errorf("Expected line to be the same as original")
	}
	if extra != nil {
		t.Errorf("Expected extra to be nil")
	}
}

func CreateLineWithExtraFullFormat(line, extra string) string {
	extra64 := MakeExtraFragment(extra)
	return fmt.Sprintf("%s%s", line, extra64)
}

func CreateLineWithExtra(line, extra string) string {
	extra64 := base64.StdEncoding.EncodeToString([]byte(extra))
	return fmt.Sprintf("%s%s%s", line, begin_token, extra64)
}
