package parser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlainTextParserParsesUTF8Text(t *testing.T) {
	parser := NewPlainTextParser()

	result, err := parser.Parse(strings.NewReader("  张三\nGo 后端工程师  "))
	if err != nil {
		t.Fatalf("parse text resume: %v", err)
	}
	if result.RawText != "张三\nGo 后端工程师" {
		t.Fatalf("unexpected raw text %q", result.RawText)
	}
	if result.ParsedData == nil {
		t.Fatal("expected parsed data")
	}

	var parsedData map[string]interface{}
	if err := json.Unmarshal([]byte(*result.ParsedData), &parsedData); err != nil {
		t.Fatalf("decode parsed data: %v", err)
	}
	if parsedData["source"] != "plain_text" {
		t.Fatalf("unexpected parsed data source %v", parsedData["source"])
	}
}

func TestPlainTextParserRejectsInvalidUTF8(t *testing.T) {
	parser := NewPlainTextParser()

	if _, err := parser.Parse(strings.NewReader(string([]byte{0xff, 0xfe, 0xfd}))); err == nil {
		t.Fatal("expected invalid UTF-8 to be rejected")
	}
}
