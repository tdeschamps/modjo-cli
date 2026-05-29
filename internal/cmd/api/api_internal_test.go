package api

import (
	"net/url"
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

func TestIsHTTPMethod(t *testing.T) {
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"} {
		if !isHTTPMethod(m) {
			t.Errorf("%s should be a method", m)
		}
	}
	if isHTTPMethod("/calls") {
		t.Error("/calls is not a method")
	}
}

func TestCloneValues(t *testing.T) {
	v := url.Values{"a": {"1", "2"}, "b": {"3"}}
	c := cloneValues(v)
	c.Add("a", "x")
	if len(v["a"]) != 2 {
		t.Error("clone should not mutate original")
	}
}

func TestBuildBody(t *testing.T) {
	io, in, _, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}}

	// No fields, no input → nil.
	if b, err := buildBody(f, nil, ""); err != nil || b != nil {
		t.Errorf("empty body = %v, %v", b, err)
	}

	// Fields → JSON object.
	b, err := buildBody(f, []string{"email=a@b.com", "role=rep"}, "")
	if err != nil || !strings.Contains(string(b), `"email":"a@b.com"`) {
		t.Errorf("field body = %s, %v", b, err)
	}

	// Bad field.
	if _, err := buildBody(f, []string{"novalue"}, ""); err == nil {
		t.Error("bad field should error")
	}

	// Stdin input.
	in.WriteString(`{"x":1}`)
	if b, err := buildBody(f, nil, "-"); err != nil || string(b) != `{"x":1}` {
		t.Errorf("stdin body = %s, %v", b, err)
	}

	// Missing file.
	if _, err := buildBody(f, nil, "/no/such/file.json"); err == nil {
		t.Error("missing file should error")
	}
}

func TestPrintRaw(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{JSON: true}}
	p := &output.Printer{Out: out, Format: output.FormatJSON}

	// Empty body → nothing.
	if err := printRaw(f, p, []byte("  ")); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Errorf("empty should print nothing, got %q", out.String())
	}

	// Valid JSON → pretty.
	out.Reset()
	if err := printRaw(f, p, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"a": 1`) {
		t.Errorf("json: %s", out.String())
	}

	// Non-JSON → passthrough.
	out.Reset()
	if err := printRaw(f, p, []byte("plain text")); err != nil {
		t.Fatal(err)
	}
	if out.String() != "plain text" {
		t.Errorf("passthrough: %q", out.String())
	}
}
