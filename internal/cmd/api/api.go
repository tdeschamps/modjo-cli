// Package api implements `modjo api` — the raw authenticated request escape
// hatch (like `gh api` / `stripe get`). Auth, base URL, and pagination are
// handled; the response is raw JSON by default.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	modjoapi "github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/output"
)

// NewCmdAPI returns the api command.
func NewCmdAPI(f *cmdutil.Factory) *cobra.Command {
	var params []string
	var fields []string
	var input string
	var paginate bool

	cmd := &cobra.Command{
		Use:     "api <method> <path>",
		Short:   "Make an authenticated request to any REST v2 endpoint",
		GroupID: "core",
		Long: `Make an authenticated request to an arbitrary Modjo REST v2 endpoint.

  modjo api GET /calls --param "limit=50" --param "relations=summary"
  modjo api POST /users --field email=new@acme.com
  cat body.json | modjo api POST /webhooks --input -`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := "/"
			switch {
			case len(args) == 2:
				path = args[1]
			case !isHTTPMethod(method):
				// "modjo api /calls" shorthand → GET.
				path, method = args[0], "GET"
			}

			client, err := f.APIClient()
			if err != nil {
				return err
			}

			query := url.Values{}
			for _, p := range params {
				k, v, ok := strings.Cut(p, "=")
				if !ok {
					return cmdutil.NewUsageError(fmt.Errorf("--param must be key=value, got %q", p))
				}
				query.Add(k, v)
			}

			body, err := buildBody(f, fields, input)
			if err != nil {
				return err
			}

			p, err := f.Printer()
			if err != nil {
				return err
			}

			if paginate {
				return paginateAll(cmd, client, method, path, query, body, p)
			}

			raw, err := client.Raw(cmd.Context(), method, path, query, body)
			if err != nil {
				return err
			}
			return printRaw(f, p, raw)
		},
	}
	cmd.Flags().StringArrayVar(&params, "param", nil, "Query parameter key=value (repeatable)")
	cmd.Flags().StringArrayVar(&fields, "field", nil, "JSON body field key=value (repeatable)")
	cmd.Flags().StringVar(&input, "input", "", "Read the request body from a file (- for stdin)")
	cmd.Flags().BoolVar(&paginate, "paginate", false, "Follow pages and concatenate results")
	return cmd
}

func buildBody(f *cmdutil.Factory, fields []string, input string) ([]byte, error) {
	if input != "" {
		if input == "-" {
			return f.IOStreams.ReadAllStdin()
		}
		// The request body path is explicitly supplied by the user via --input.
		return os.ReadFile(input)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	m := map[string]any{}
	for _, fld := range fields {
		k, v, ok := strings.Cut(fld, "=")
		if !ok {
			return nil, cmdutil.NewUsageError(fmt.Errorf("--field must be key=value, got %q", fld))
		}
		m[k] = v
	}
	return json.Marshal(m)
}

// paginateAll follows the {data, pagination:{page,size,total}} envelope and
// prints the concatenated rows as a single JSON array. It walks pages until the
// reported total is covered or a short/empty page arrives.
func paginateAll(cmd *cobra.Command, client *modjoapi.Client, method, path string, query url.Values, body []byte, p *output.Printer) error {
	var all []json.RawMessage
	for page := 1; ; page++ {
		q := maps.Clone(query)
		q.Set("page", strconv.Itoa(page))
		raw, err := client.Raw(cmd.Context(), method, path, q, body)
		if err != nil {
			return err
		}
		var resp struct {
			Data       []json.RawMessage `json:"data"`
			Pagination struct {
				Page  int `json:"page"`
				Size  int `json:"size"`
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return fmt.Errorf("response is not a paginated list: %w", err)
		}
		all = append(all, resp.Data...)
		if len(resp.Data) == 0 {
			break
		}
		if resp.Pagination.Total > 0 && len(all) >= resp.Pagination.Total {
			break
		}
		// Total unknown/zero: keep going until a short page (fewer rows than the
		// server-reported page size) signals the end, rather than assuming the
		// first page is the only one. Fall back to stopping if Size is also
		// unreported, since we have no signal to continue safely.
		if resp.Pagination.Total == 0 {
			if resp.Pagination.Size <= 0 || len(resp.Data) < resp.Pagination.Size {
				break
			}
		}
	}
	return p.PrintJSON(all)
}

func printRaw(f *cmdutil.Factory, p *output.Printer, raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		_, err := f.IOStreams.Out.Write(raw)
		return err
	}
	return p.PrintJSON(v)
}

func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
		return true
	}
	return false
}
