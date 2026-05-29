package update

import (
	"strings"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/cmd/version"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func runUpdate(t *testing.T) string {
	t.Helper()
	io, _, out, errOut := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}}
	cmd := NewCmdUpdate(f)
	cmd.SetArgs(nil)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String() + errOut.String()
}

func TestUpdateDevBuild(t *testing.T) {
	orig := version.Version
	version.Version = "dev"
	defer func() { version.Version = orig }()
	if !strings.Contains(runUpdate(t), "development build") {
		t.Error("dev build should mention development build")
	}
}

func TestUpdateReleaseBuild(t *testing.T) {
	orig := version.Version
	version.Version = "1.2.3"
	defer func() { version.Version = orig }()
	if !strings.Contains(runUpdate(t), "package manager") {
		t.Error("release build should mention package manager")
	}
}
