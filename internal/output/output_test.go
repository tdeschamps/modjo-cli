package output

import (
	"bytes"
	"strings"
	"testing"
)

type deal struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
	Status string `json:"status"`
}

var sample = []deal{
	{"Contoso – Platform", 42000, "Open"},
	{"Globex – Renewal", 18500, "Open"},
}

func fields() []Field {
	return []Field{
		{Name: "NAME", Extract: func(v any) string { return v.(deal).Name }},
		{Name: "AMOUNT", Extract: func(v any) string { return itoa(v.(deal).Amount) }},
		{Name: "STATUS", Extract: func(v any) string { return v.(deal).Status }},
	}
}

func items() []any { return []any{sample[0], sample[1]} }

func itoa(n int) string {
	b := []byte{}
	if n == 0 {
		return "0"
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestTableRender(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatTable}
	if err := p.Output(sample, items(), fields()); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "Contoso – Platform") {
		t.Errorf("table missing content:\n%s", got)
	}
	// Columns should be aligned: header line then rows.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("want 3 lines (header+2 rows), got %d:\n%s", len(lines), got)
	}
}

func TestCSVRender(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatCSV}
	if err := p.Output(sample, items(), fields()); err != nil {
		t.Fatal(err)
	}
	want := "NAME,AMOUNT,STATUS\nContoso – Platform,42000,Open\nGlobex – Renewal,18500,Open\n"
	if buf.String() != want {
		t.Errorf("csv:\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestTSVRender(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatTSV}
	_ = p.Output(sample, items(), fields())
	if !strings.Contains(buf.String(), "NAME\tAMOUNT\tSTATUS") {
		t.Errorf("tsv header wrong:\n%s", buf.String())
	}
}

func TestJSONRender(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatJSON}
	_ = p.Output(sample, items(), fields())
	got := buf.String()
	if !strings.Contains(got, `"amount": 42000`) {
		t.Errorf("json missing field:\n%s", got)
	}
}

func TestColumnsFilter(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatCSV, Columns: []string{"name", "status"}}
	_ = p.Output(sample, items(), fields())
	want := "NAME,STATUS\nContoso – Platform,Open\nGlobex – Renewal,Open\n"
	if buf.String() != want {
		t.Errorf("columns filter:\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestJQFilter(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatJSON, JQ: ".[].amount"}
	if err := p.Output(sample, items(), fields()); err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(buf.String())
	if len(got) != 2 || got[0] != "42000" || got[1] != "18500" {
		t.Errorf("jq output = %v", got)
	}
}

func TestParseFormat(t *testing.T) {
	cases := map[string]Format{"json": FormatJSON, "csv": FormatCSV, "tsv": FormatTSV, "yaml": FormatYAML, "table": FormatTable}
	for in, want := range cases {
		got, err := ParseFormat(in)
		if err != nil || got != want {
			t.Errorf("ParseFormat(%q)=%v,%v", in, got, err)
		}
	}
	if _, err := ParseFormat("bogus"); err == nil {
		t.Error("expected error for bogus format")
	}
}
