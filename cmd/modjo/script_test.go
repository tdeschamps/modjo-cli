package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain registers the real modjo entrypoint as a testscript command so the
// .txtar scripts in test/script drive the actual binary behavior (the gh/go
// approach to CLI e2e testing).
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"modjo": func() { os.Exit(run()) },
	})
}

// stubHandler emulates the slice of the Modjo API the e2e scripts exercise.
func stubHandler() http.Handler {
	mux := http.NewServeMux()
	// Credential validation reads /users?page=1 (the API has no /me route).
	mux.HandleFunc("/v2/users", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       []map[string]any{{"id": 1, "email": "me@acme.com"}},
			"pagination": map[string]any{"page": 1, "size": 50, "total": 1},
		})
	})
	mux.HandleFunc("/v2/deals", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"crmId": "D1", "name": "Contoso", "status": "Open", "amount": 42000},
			},
			"pagination": map[string]any{"page": 1, "size": 50, "total": 1},
		})
	})
	return mux
}

func TestScripts(t *testing.T) {
	srv := httptest.NewServer(stubHandler())
	t.Cleanup(srv.Close)

	testscript.Run(t, testscript.Params{
		Dir: "../../test/script",
		Setup: func(e *testscript.Env) error {
			e.Setenv("MODJO_BASE_URL", srv.URL+"/v2")
			e.Setenv("MODJO_API_KEY", "test-key")
			e.Setenv("HOME", e.WorkDir)
			e.Setenv("XDG_CONFIG_HOME", e.WorkDir+"/.config")
			// Force the file-backed credential store so the auth scripts stay
			// hermetic: without this, `auth login` writes to the real OS keychain
			// (which the redirected HOME can't isolate), and on macOS it can pop
			// an interactive keychain prompt.
			e.Setenv("MODJO_NO_KEYRING", "1")
			return nil
		},
	})
}
