package output

import (
	"encoding/json"
	"io"
	"os"
)

// Format 输出格式
type Format string

const (
	FormatJSON   Format = "json"
	FormatPretty Format = "pretty"
)

// Writer 输出写入器
type Writer struct {
	Out    io.Writer
	Format Format
}

// New 默认写到 stdout，format=json
func New(format Format) *Writer {
	if format == "" {
		format = FormatJSON
	}
	return &Writer{Out: os.Stdout, Format: format}
}

// JSON 以紧凑 JSON 输出（保持结构稳定，Agent 可直接解析）
func (w *Writer) JSON(v any) error {
	enc := json.NewEncoder(w.Out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
