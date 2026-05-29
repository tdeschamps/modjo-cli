package ask

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

func TestLooksLikeUUID(t *testing.T) {
	if !looksLikeUUID("1204e84f-6edd-4782-bbdf-e5e070b400cf") {
		t.Error("valid uuid")
	}
	for _, bad := range []string{
		"short",
		"1204e84f6edd4782bbdfe5e070b400cffff",  // wrong length
		"1204e84fX6edd-4782-bbdf-e5e070b400cf", // wrong separator position
		"zzzze84f-6edd-4782-bbdf-e5e070b400cf", // non-hex
	} {
		if looksLikeUUID(bad) {
			t.Errorf("%q should not look like a uuid", bad)
		}
	}
}

func askFactory(t *testing.T, agentsBody string) *cmdutil.Factory {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"values":[` + agentsBody + `],"pagination":{}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MODJO_BASE_URL", srv.URL)
	io, _, _, _ := iostreams.Test()
	store := auth.NewMemoryStore()
	_ = store.Set("default", auth.Credential{Token: "t"})
	return &cmdutil.Factory{IOStreams: io, Flags: &cmdutil.GlobalFlags{}, ConfigPath: t.TempDir() + "/c.toml", CredStore: store}
}

func TestResolveAgent(t *testing.T) {
	ctx := context.Background()

	// Empty → empty.
	if id, err := resolveAgent(ctx, askFactory(t, ``), ""); err != nil || id != "" {
		t.Errorf("empty agent = %q, %v", id, err)
	}

	// UUID passthrough (no server call needed).
	uuid := "1204e84f-6edd-4782-bbdf-e5e070b400cf"
	if id, _ := resolveAgent(ctx, askFactory(t, ``), uuid); id != uuid {
		t.Errorf("uuid passthrough = %q", id)
	}

	// Native name resolves offline.
	if id, _ := resolveAgent(ctx, askFactory(t, ``), "DealBriefing"); id != uuid {
		t.Errorf("native name = %q", id)
	}

	// Custom name resolved via server lookup.
	f := askFactory(t, `{"uuid":"custom-uuid","name":"ChurnInspector","origin":"user"}`)
	if id, err := resolveAgent(ctx, f, "ChurnInspector"); err != nil || id != "custom-uuid" {
		t.Errorf("custom lookup = %q, %v", id, err)
	}

	// Not found → error.
	f2 := askFactory(t, `{"uuid":"x","name":"Other"}`)
	if _, err := resolveAgent(ctx, f2, "Missing"); err == nil {
		t.Error("expected not-found error")
	}
}
