package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
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

// Render 根据 Format 渲染：JSON 或调用 prettyFn 输出表格
// prettyFn 接收一个 tabwriter，由调用方写入行
func (w *Writer) Render(v any, prettyFn func(tw *tabwriter.Writer)) error {
	if w.Format == FormatJSON {
		return w.JSON(v)
	}
	tw := tabwriter.NewWriter(w.Out, 0, 0, 2, ' ', 0)
	prettyFn(tw)
	return tw.Flush()
}

// PrettyLine 辅助：以 TAB 分隔字段写入 tabwriter
func PrettyLine(tw *tabwriter.Writer, cols ...string) {
	fmt.Fprintln(tw, strings.Join(cols, "\t"))
}
