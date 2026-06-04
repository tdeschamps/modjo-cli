package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestYAMLRender(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatYAML}
	if err := p.Output(sample, items(), fields()); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "name: Contoso") && !strings.Contains(got, "amount: 42000") {
		t.Errorf("yaml output:\n%s", got)
	}
}

func TestYAMLWithJQ(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatYAML, JQ: ".[].name"}
	if err := p.Output(sample, items(), fields()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Contoso") {
		t.Errorf("yaml+jq:\n%s", buf.String())
	}
}

func TestJQInvalidExpression(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatJSON, JQ: "this is not jq ]["}
	if err := p.Output(sample, items(), fields()); err == nil {
		t.Error("expected jq parse error")
	}
}

func TestJQRuntimeError(t *testing.T) {
	var buf bytes.Buffer
	// .foo on an array errors at runtime in jq.
	p := &Printer{Out: &buf, Format: FormatJSON, JQ: ".foo"}
	if err := p.Output(sample, items(), fields()); err == nil {
		t.Error("expected jq runtime error")
	}
}

func TestJQStringResult(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatJSON, JQ: ".[0].name"}
	if err := p.Output(sample, items(), fields()); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "Contoso – Platform" {
		t.Errorf("jq string result = %q", buf.String())
	}
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatJSON}
	if err := p.PrintJSON(map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"a": 1`) {
		t.Errorf("PrintJSON: %s", buf.String())
	}
}

func TestUnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: Format("xml")}
	if err := p.Output(sample, items(), fields()); err == nil {
		t.Error("expected unsupported format error")
	}
}

func TestEmptyTable(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Format: FormatTable}
	if err := p.Output([]deal{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "" {
		t.Errorf("empty table should render nothing, got %q", buf.String())
	}
}
