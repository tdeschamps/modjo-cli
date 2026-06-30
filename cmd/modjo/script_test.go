package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// writeJSON encodes v as the response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// listOf wraps rows in the API's paginated envelope.
func listOf(rows ...map[string]any) map[string]any {
	return map[string]any{
		"data":       rows,
		"pagination": map[string]any{"page": 1, "size": 50, "total": len(rows)},
	}
}

// stubHandler emulates the slice of the Modjo API the e2e scripts exercise.
func stubHandler() http.Handler {
	mux := http.NewServeMux()

	// --- users: list (also the credential-validation read) + membership writes ---
	// The "/v2/users/" subtree (trailing slash) covers /users/{id}, /users/{id}/teams
	// and /users/{id}/teams/{teamId}; the exact "/v2/users" serves the list.
	mux.HandleFunc("/v2/users", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, listOf(map[string]any{"id": 1, "email": "me@acme.com"}))
	})
	mux.HandleFunc("/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch:
			writeJSON(w, map[string]any{"id": 7, "email": "u@acme.com", "jobTitle": "VP"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/teams"):
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, map[string]any{"userId": 7, "teamId": 5})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			writeJSON(w, map[string]any{"id": 7, "email": "u@acme.com"})
		}
	})

	// --- deals: list + summary ---
	mux.HandleFunc("/v2/deals", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, listOf(map[string]any{"crmId": "D1", "name": "Contoso", "status": "Open", "amount": 42000}))
	})
	mux.HandleFunc("/v2/deals/", func(w http.ResponseWriter, r *http.Request) {
		// /v2/deals/{id}/summary
		writeJSON(w, map[string]any{
			"data":     []map[string]any{{"type": "overview", "value": "Strong intent to buy."}},
			"language": "en",
		})
	})

	// --- teams: list + writes + members ---
	mux.HandleFunc("/v2/teams", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, map[string]any{"id": 5, "name": "Sales"})
			return
		}
		writeJSON(w, listOf(map[string]any{"id": 5, "name": "Sales"}))
	})
	mux.HandleFunc("/v2/teams/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/members"):
			writeJSON(w, listOf(map[string]any{"id": 1, "email": "rep@acme.com", "firstName": "Rep", "lastName": "One"}))
		case r.Method == http.MethodPatch:
			writeJSON(w, map[string]any{"id": 5, "name": "Renamed"})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			writeJSON(w, map[string]any{"id": 5, "name": "Sales"})
		}
	})

	// --- calls: upload (POST /calls -> 202) ---
	mux.HandleFunc("/v2/calls", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]any{"callId": "call-1", "status": "processing"})
	})

	// --- calls: sub-resources (notes, next-steps, tags, crm-answers) ---
	mux.HandleFunc("/v2/calls/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/notes"):
			writeJSON(w, map[string]any{"data": []map[string]any{{"id": 9, "title": "Recap", "status": "PUBLISHED", "type": "AI"}}})
		case strings.HasSuffix(path, "/next-steps"):
			writeJSON(w, map[string]any{"data": []map[string]any{{"title": "Send quote", "description": "by Friday"}}})
		case strings.HasSuffix(path, "/crm-filling-answers"):
			writeJSON(w, listOf(map[string]any{"uuid": "ans-1", "callId": 42, "crmFillingFieldUuid": "fld-1", "crmId": "C1"}))
		case strings.HasSuffix(path, "/tags") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, map[string]any{"callId": 42, "tagId": 3})
		case strings.HasSuffix(path, "/tags"):
			writeJSON(w, map[string]any{"data": []map[string]any{{"id": 3, "name": "Pricing", "color": "blue"}}})
		case strings.Contains(path, "/tags/") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			writeJSON(w, map[string]any{"id": 42, "name": "Discovery"})
		}
	})

	// --- webhooks: update (PATCH /webhooks/{uuid}) ---
	mux.HandleFunc("/v2/webhooks/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"uuid": "wh-1", "name": "Renamed", "url": "https://x"})
	})

	// --- CRM filling templates: list + get + fields ---
	mux.HandleFunc("/v2/crm-filling-templates", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, listOf(map[string]any{"uuid": "tpl-1", "title": "Discovery", "status": "published", "language": "en"}))
	})
	mux.HandleFunc("/v2/crm-filling-templates/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/fields") {
			writeJSON(w, listOf(map[string]any{"uuid": "fld-1", "order": 1, "crm": "hubspot", "entityType": "deal", "fieldKey": "amount", "fieldType": "number", "prompt": "Deal amount?"}))
			return
		}
		writeJSON(w, map[string]any{"uuid": "tpl-1", "title": "Discovery", "status": "published", "language": "en"})
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
