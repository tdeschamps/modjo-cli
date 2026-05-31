package iostreams

import (
	"strings"
	"testing"
)

func TestPrompt(t *testing.T) {
	s, in, _, errOut := Test()
	in.WriteString("  Contoso  \n")
	got, err := s.Prompt("Name: ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Contoso" {
		t.Errorf("Prompt = %q", got)
	}
	if !strings.Contains(errOut.String(), "Name:") {
		t.Errorf("label should go to stderr: %q", errOut.String())
	}
}

func TestSelect(t *testing.T) {
	s, in, _, _ := Test()
	in.WriteString("2\n")
	idx, err := s.Select("Pick:", []string{"call", "deal", "account"})
	if err != nil || idx != 1 {
		t.Fatalf("Select = %d, %v", idx, err)
	}
}

func TestSelectInvalid(t *testing.T) {
	for _, input := range []string{"0\n", "9\n", "x\n"} {
		s, in, _, _ := Test()
		in.WriteString(input)
		if _, err := s.Select("Pick:", []string{"a", "b"}); err == nil {
			t.Errorf("input %q should be an invalid choice", input)
		}
	}
}
