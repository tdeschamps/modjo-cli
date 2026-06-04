package output

import (
	"errors"
	"testing"
)

// failWriter fails after allowing n successful writes (so we can fail mid-render).
type failWriter struct{ remaining int }

func (w *failWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errors.New("write failed")
	}
	w.remaining--
	return len(p), nil
}

func TestRenderWriteErrors(t *testing.T) {
	formats := []Format{FormatTable, FormatCSV, FormatTSV, FormatJSON, FormatYAML}
	for _, f := range formats {
		p := &Printer{Out: &failWriter{remaining: 0}, Format: f}
		if err := p.Output(sample, items(), fields()); err == nil {
			t.Errorf("%s: expected write error", f)
		}
	}
}

func TestRenderTableHeaderWriteError(t *testing.T) {
	// Allow the header row, then fail on the first data row.
	p := &Printer{Out: &failWriter{remaining: 1}, Format: FormatTable}
	if err := p.Output(sample, items(), fields()); err == nil {
		t.Error("expected write error on data row")
	}
}

func TestRenderSeparatedRowError(t *testing.T) {
	// csv.Writer buffers; a tiny allowance still surfaces the flush error.
	p := &Printer{Out: &failWriter{remaining: 0}, Format: FormatCSV}
	if err := p.Output(sample, items(), fields()); err == nil {
		t.Error("expected csv write error")
	}
}

func TestToGenericFallback(t *testing.T) {
	// A value that can't be JSON-marshalled falls back to itself.
	ch := make(chan int)
	if got := toGeneric(ch); got == nil {
		t.Error("toGeneric should fall back to the original value")
	}
}

func TestRenderYAMLJQError(t *testing.T) {
	p := &Printer{Out: &failWriter{remaining: 100}, Format: FormatYAML, JQ: "this is bad jq ]["}
	if err := p.Output(sample, items(), fields()); err == nil {
		t.Error("expected jq parse error in yaml path")
	}
}

func TestColumnsUnknownIgnored(t *testing.T) {
	var w failWriter
	w.remaining = 100
	p := &Printer{Out: &w, Format: FormatCSV, Columns: []string{"name", "nonexistent"}}
	// Should not error; unknown columns are skipped.
	if err := p.Output(sample, items(), fields()); err != nil {
		t.Fatal(err)
	}
}
