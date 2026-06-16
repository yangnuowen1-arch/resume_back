package parser

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const defaultMaxResumeBytes int64 = 10 << 20

type Result struct {
	RawText    string
	ParsedData *string
	Language   *string
}

type Parser interface {
	Parse(reader io.Reader) (*Result, error)
}

type PlainTextParser struct {
	MaxBytes int64
}

func NewPlainTextParser() PlainTextParser {
	return PlainTextParser{
		MaxBytes: defaultMaxResumeBytes,
	}
}

func (p PlainTextParser) Parse(reader io.Reader) (*Result, error) {
	maxBytes := p.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxResumeBytes
	}

	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("简历文件过大，无法解析")
	}
	if len(data) == 0 {
		return nil, errors.New("简历文件为空")
	}
	if !utf8.Valid(data) {
		return nil, errors.New("当前解析器只支持 UTF-8 文本简历")
	}

	rawText := string(data)
	if looksBinaryText(rawText) {
		return nil, errors.New("当前解析器只支持文本简历")
	}

	text := strings.TrimSpace(sanitizeText(rawText))
	if text == "" {
		return nil, errors.New("简历文本为空")
	}

	parsedData := map[string]interface{}{
		"source":     "plain_text",
		"textLength": utf8.RuneCountInString(text),
	}
	encoded, err := json.Marshal(parsedData)
	if err != nil {
		return nil, err
	}
	encodedString := string(encoded)

	return &Result{
		RawText:    text,
		ParsedData: &encodedString,
	}, nil
}

func sanitizeText(text string) string {
	text = strings.TrimPrefix(text, "\ufeff")
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
}

func looksBinaryText(text string) bool {
	total := 0
	controlLike := 0
	for _, r := range text {
		total++
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if unicode.IsControl(r) {
			controlLike++
		}
	}

	return total > 0 && controlLike*10 > total
}
