// Package output renders command results in the format the context calls for:
// aligned tables for a TTY, machine-readable JSON/CSV/TSV/YAML when piped. It
// also implements the built-in --jq filter (via gojq) so users never need an
// external jq binary. JSON output is treated as an API: golden tests lock the
// shapes down.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"
)

// Format is an output format selector.
type Format string

// Supported formats.
const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
	FormatTSV   Format = "tsv"
	FormatYAML  Format = "yaml"
)

// ParseFormat validates and parses a format string.
func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case FormatTable:
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatCSV:
		return FormatCSV, nil
	case FormatTSV:
		return FormatTSV, nil
	case FormatYAML:
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("unknown output format %q (use table|json|csv|tsv|yaml)", s)
	}
}

// Field describes one column of tabular output: a header and an extractor that
// pulls the cell value from a single item.
type Field struct {
	Name    string
	Extract func(any) string
}

// Printer renders values to Out in the configured Format.
type Printer struct {
	Out          io.Writer
	Format       Format
	Columns      []string // optional case-insensitive column filter/order
	JQ           string   // optional jq expression (applies to JSON/YAML path)
	ColorEnabled bool
}

// Output renders results. raw is the structured value used for JSON/YAML (and
// the jq filter); items+fields drive tabular formats (table/csv/tsv). Passing
// both lets each format use its most natural representation.
func (p *Printer) Output(raw any, items []any, fields []Field) error {
	switch p.Format {
	case FormatJSON:
		return p.renderJSON(raw)
	case FormatYAML:
		return p.renderYAML(raw)
	case FormatCSV:
		return p.renderSeparated(items, fields, ',')
	case FormatTSV:
		return p.renderSeparated(items, fields, '\t')
	case FormatTable, "":
		return p.renderTable(items, p.selectFields(fields))
	default:
		return fmt.Errorf("unsupported format %q", p.Format)
	}
}

// PrintJSON renders an arbitrary value as JSON (with optional jq), used by the
// raw `api` and `mcp call` commands.
func (p *Printer) PrintJSON(raw any) error { return p.renderJSON(raw) }

func (p *Printer) selectFields(fields []Field) []Field {
	if len(p.Columns) == 0 {
		return fields
	}
	byName := map[string]Field{}
	for _, f := range fields {
		byName[strings.ToLower(f.Name)] = f
	}
	out := make([]Field, 0, len(p.Columns))
	for _, c := range p.Columns {
		if f, ok := byName[strings.ToLower(strings.TrimSpace(c))]; ok {
			out = append(out, f)
		}
	}
	return out
}

func (p *Printer) renderJSON(raw any) error {
	if p.JQ != "" {
		return p.renderJQ(raw)
	}
	enc := json.NewEncoder(p.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(raw)
}

func (p *Printer) renderYAML(raw any) error {
	if p.JQ != "" {
		// Apply jq, then emit YAML of the results.
		results, err := p.evalJQ(raw)
		if err != nil {
			return err
		}
		enc := yaml.NewEncoder(p.Out)
		enc.SetIndent(2)
		defer enc.Close()
		for _, r := range results {
			if err := enc.Encode(r); err != nil {
				return err
			}
		}
		return nil
	}
	enc := yaml.NewEncoder(p.Out)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(toGeneric(raw))
}

func (p *Printer) renderSeparated(items []any, fields []Field, sep rune) error {
	fields = p.selectFields(fields)
	w := csv.NewWriter(p.Out)
	w.Comma = sep
	header := make([]string, len(fields))
	for i, f := range fields {
		header[i] = f.Name
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, it := range items {
		row := make([]string, len(fields))
		for i, f := range fields {
			row[i] = f.Extract(it)
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func (p *Printer) renderTable(items []any, fields []Field) error {
	if len(fields) == 0 {
		return nil
	}
	widths := make([]int, len(fields))
	for i, f := range fields {
		widths[i] = displayWidth(f.Name)
	}
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		row := make([]string, len(fields))
		for i, f := range fields {
			row[i] = f.Extract(it)
			if w := displayWidth(row[i]); w > widths[i] {
				widths[i] = w
			}
		}
		rows = append(rows, row)
	}

	writeRow := func(cells []string) error {
		var b strings.Builder
		for i, c := range cells {
			b.WriteString(c)
			if i < len(cells)-1 {
				pad := widths[i] - displayWidth(c) + 2
				b.WriteString(strings.Repeat(" ", pad))
			}
		}
		_, err := fmt.Fprintln(p.Out, strings.TrimRight(b.String(), " "))
		return err
	}

	header := make([]string, len(fields))
	for i, f := range fields {
		header[i] = f.Name
	}
	if err := writeRow(header); err != nil {
		return err
	}
	for _, r := range rows {
		if err := writeRow(r); err != nil {
			return err
		}
	}
	return nil
}

// displayWidth counts runes (good enough for our column alignment without a
// full east-asian-width table).
func displayWidth(s string) int { return len([]rune(s)) }

func (p *Printer) renderJQ(raw any) error {
	results, err := p.evalJQ(raw)
	if err != nil {
		return err
	}
	for _, r := range results {
		switch v := r.(type) {
		case string:
			fmt.Fprintln(p.Out, v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return err
			}
			fmt.Fprintln(p.Out, string(b))
		}
	}
	return nil
}

func (p *Printer) evalJQ(raw any) ([]any, error) {
	query, err := gojq.Parse(p.JQ)
	if err != nil {
		return nil, fmt.Errorf("invalid --jq expression: %w", err)
	}
	input := toGeneric(raw)
	iter := query.Run(input)
	var out []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, fmt.Errorf("jq: %w", err)
		}
		out = append(out, v)
	}
	return out, nil
}

// toGeneric round-trips a value through JSON so gojq/yaml see plain
// maps/slices/scalars (and json struct tags are honored).
func toGeneric(raw any) any {
	b, err := json.Marshal(raw)
	if err != nil {
		return raw
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return raw
	}
	return v
}
