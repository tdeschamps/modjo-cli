package cmdutil

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

type row struct {
	Name string `json:"name"`
}

func fields() []output.Field {
	return []output.Field{{Name: "NAME", Extract: func(v any) string { return v.(row).Name }}}
}

func seqOf(items []row, err error) iter.Seq2[row, error] {
	return func(yield func(row, error) bool) {
		for _, it := range items {
			if !yield(it, nil) {
				return
			}
		}
		if err != nil {
			yield(row{}, err)
		}
	}
}

func renderFactory(t *testing.T) (*Factory, *strings.Builder) {
	t.Helper()
	io, _, out, _ := iostreams.Test()
	_ = out
	b := &strings.Builder{}
	io.Out = b
	return &Factory{IOStreams: io, Flags: &GlobalFlags{JSON: true}, ConfigPath: t.TempDir() + "/c.toml"}, b
}

func TestCollectAndRender(t *testing.T) {
	f, out := renderFactory(t)
	err := CollectAndRender(context.Background(), f, seqOf([]row{{"a"}, {"b"}}, nil), fields())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"name": "a"`) {
		t.Errorf("output: %s", out.String())
	}
}

func TestCollectAndRenderError(t *testing.T) {
	f, _ := renderFactory(t)
	err := CollectAndRender(context.Background(), f, seqOf([]row{{"a"}}, errors.New("boom")), fields())
	if err == nil || err.Error() != "boom" {
		t.Errorf("want boom, got %v", err)
	}
}

func TestRenderSliceAndOne(t *testing.T) {
	f, out := renderFactory(t)
	if err := RenderSlice(f, []row{{"x"}}, fields()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"name": "x"`) {
		t.Errorf("RenderSlice: %s", out.String())
	}

	f2, out2 := renderFactory(t)
	if err := RenderOne(f2, row{"y"}, fields()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2.String(), `"name": "y"`) {
		t.Errorf("RenderOne: %s", out2.String())
	}
}

func TestRenderBadFormatPropagates(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{Output: "bogus"}, ConfigPath: t.TempDir() + "/c.toml"}
	if err := RenderSlice(f, []row{{"x"}}, fields()); err == nil {
		t.Error("expected error from bad format")
	}
}
