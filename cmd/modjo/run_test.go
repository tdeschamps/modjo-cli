package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func TestRunVersion(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"modjo", "version"}
	if code := run(); code != 0 {
		t.Errorf("run version exit = %d", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"modjo", "no-such-command"}
	if code := run(); code == 0 {
		t.Error("unknown command should be non-zero")
	}
}

func TestRunHelp(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"modjo", "--help"}
	if code := run(); code != 0 {
		t.Errorf("run --help exit = %d", code)
	}
}

func TestPrintError(t *testing.T) {
	io, _, _, errBuf := iostreams.Test()

	// Plain error.
	printError(io, errors.New("boom"))
	if !strings.Contains(errBuf.String(), "boom") {
		t.Errorf("plain: %s", errBuf.String())
	}

	// Silent error prints just the message.
	errBuf.Reset()
	printError(io, cmdutil.NewSilentError(3, errors.New("hushed")))
	if !strings.Contains(errBuf.String(), "hushed") {
		t.Errorf("silent: %s", errBuf.String())
	}

	// API error adds a hint + request id.
	errBuf.Reset()
	printError(io, &api.Error{StatusCode: 401, Message: "unauthorized", RequestID: "req_9"})
	out := errBuf.String()
	if !strings.Contains(out, "Hint") || !strings.Contains(out, "req_9") {
		t.Errorf("api error: %s", out)
	}
}

func TestRemediation(t *testing.T) {
	for _, code := range []int{401, 403, 404, 422, 429} {
		if remediation(code) == "" {
			t.Errorf("remediation(%d) should be non-empty", code)
		}
	}
	if remediation(200) != "" {
		t.Error("remediation(200) should be empty")
	}
}

func TestPrintErrorAPIWithoutRequestID(t *testing.T) {
	var buf bytes.Buffer
	io, _, _, _ := iostreams.Test()
	io.ErrOut = &buf
	printError(io, &api.Error{StatusCode: 500, Message: "server"})
	if !strings.Contains(buf.String(), "server") {
		t.Errorf("got %s", buf.String())
	}
}
