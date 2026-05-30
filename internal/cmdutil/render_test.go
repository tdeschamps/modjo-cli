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

func TestRenderSlice(t *testing.T) {
	f, out := renderFactory(t)
	if err := RenderSlice(f, []row{{"x"}}, fields()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"name": "x"`) {
		t.Errorf("RenderSlice: %s", out.String())
	}
}

func TestGetAndRender(t *testing.T) {
	f, out := renderFactory(t)
	get := func(_ context.Context, id string) (row, error) { return row{Name: id}, nil }
	if err := GetAndRender(context.Background(), f, []string{"a", "b"}, get, fields()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"name": "a"`) || !strings.Contains(out.String(), `"name": "b"`) {
		t.Errorf("GetAndRender: %s", out.String())
	}
}

func TestGetAndRenderError(t *testing.T) {
	f, _ := renderFactory(t)
	get := func(_ context.Context, id string) (row, error) { return row{}, errors.New("boom") }
	if err := GetAndRender(context.Background(), f, []string{"a"}, get, fields()); err == nil || err.Error() != "boom" {
		t.Errorf("want boom, got %v", err)
	}
}

func TestRenderBadFormatPropagates(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := &Factory{IOStreams: io, Flags: &GlobalFlags{Output: "bogus"}, ConfigPath: t.TempDir() + "/c.toml"}
	if err := RenderSlice(f, []row{{"x"}}, fields()); err == nil {
		t.Error("expected error from bad format")
	}
}
