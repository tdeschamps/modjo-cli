// Package auth implements the `modjo auth` command group: login (API key today,
// OAuth forward-looking), logout, status, refresh, switch, and token.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tdeschamps/modjo-cli/internal/api"
	"github.com/tdeschamps/modjo-cli/internal/auth"
	"github.com/tdeschamps/modjo-cli/internal/cmdutil"
	"github.com/tdeschamps/modjo-cli/internal/config"
	"github.com/tdeschamps/modjo-cli/internal/httpclient"
	"github.com/tdeschamps/modjo-cli/internal/iostreams"
)

// NewCmdAuth returns the auth command group.
func NewCmdAuth(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth <command>",
		Short:   "Authenticate modjo and manage credentials",
		GroupID: "config",
	}
	cmd.AddCommand(
		newLoginCmd(f),
		newLogoutCmd(f),
		newStatusCmd(f),
		newRefreshCmd(f),
		newSwitchCmd(f),
		newTokenCmd(f),
	)
	return cmd
}

func newLoginCmd(f *cmdutil.Factory) *cobra.Command {
	var withToken, useOAuth, useAPIKey, web bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Modjo",
		Long: `Authenticate with Modjo using an API key (available today) or OAuth
(forward-looking). With --with-token the key is read from stdin, which is
CI-friendly:

  echo $MODJO_KEY | modjo auth login --with-token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if useOAuth {
				return fmt.Errorf("OAuth is not yet available for your workspace; use --api-key for now")
			}
			io := f.IOStreams
			var key string

			switch {
			case withToken:
				b, err := io.ReadAllStdin()
				if err != nil {
					return err
				}
				key = strings.TrimSpace(string(b))
			default:
				if web {
					io.Errf("Opening %s\n", config.IntegrationsURL)
					_ = cmdutil.OpenBrowser(config.IntegrationsURL)
				} else {
					io.Errf("Create or copy an API key at %s\n", config.IntegrationsURL)
				}
				if !io.CanPrompt() {
					return fmt.Errorf("cannot prompt for a key in a non-interactive session; use --with-token")
				}
				k, err := cmdutil.PromptSecret(io, "Paste your Modjo API key: ")
				if err != nil {
					return err
				}
				key = strings.TrimSpace(k)
			}
			_ = useAPIKey // both non-oauth paths use the API key flow

			if key == "" {
				return cmdutil.NewUsageError(fmt.Errorf("no API key provided"))
			}

			// Validate the key against a lightweight authed endpoint.
			profileName, err := f.ActiveProfile()
			if err != nil {
				return err
			}
			client := api.New(api.Options{
				BaseURL:    mustResolve(f, "base_url"),
				HTTPClient: validationClient(f, key),
			})
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			sp := io.NewSpinner("Validating API key…")
			sp.Start()
			_, verr := client.Me(ctx)
			sp.Stop()
			if verr != nil {
				return fmt.Errorf("could not validate API key: %w", verr)
			}

			store, err := f.CredentialStore()
			if err != nil {
				return err
			}
			cred := auth.Credential{Token: key, Method: auth.MethodAPIKey}
			if err := store.Set(profileName, cred); err != nil {
				return err
			}

			// Record the auth method on the profile.
			cfg, _ := f.Config()
			if cfg != nil {
				cfg.ProfileOrDefault(profileName).AuthMethod = auth.MethodAPIKey
				_ = f.SaveConfig(cfg)
			}

			io.RenderBanner(iostreams.Banner{
				Kind:     iostreams.BannerSuccess,
				Headline: fmt.Sprintf("Logged in to profile %q", profileName),
				Body:     fmt.Sprintf("Key %s — this credential grants access to the entire workspace.", auth.Fingerprint(key)),
				NextSteps: []string{
					"modjo info — see your configuration",
					"modjo calls list — pull recent calls",
					"modjo ask deal <id> \"What are the risks?\"",
				},
			})
			return nil
		},
	}
	cmd.Flags().BoolVar(&withToken, "with-token", false, "Read the API key from stdin")
	cmd.Flags().BoolVar(&useOAuth, "oauth", false, "Force the OAuth device flow (when available)")
	cmd.Flags().BoolVar(&useAPIKey, "api-key", false, "Force the API key paste flow")
	cmd.Flags().BoolVar(&web, "web", false, "Open the settings page in a browser")
	return cmd
}

func newLogoutCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove credentials for the active profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := f.ActiveProfile()
			if err != nil {
				return err
			}
			store, err := f.CredentialStore()
			if err != nil {
				return err
			}
			if err := store.Delete(profileName); err != nil {
				if err == auth.ErrNotFound {
					f.IOStreams.Errf("No credentials stored for profile %q\n", profileName)
					return nil
				}
				return err
			}
			f.IOStreams.Errf("%s Logged out of profile %q\n", f.IOStreams.Green("✓"), profileName)
			return nil
		},
	}
}

func newStatusCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show who you are, the workspace, method, scopes, and expiry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := f.ActiveProfile()
			if err != nil {
				return err
			}
			store, err := f.CredentialStore()
			if err != nil {
				return err
			}
			cred, err := store.Get(profileName)
			if err != nil {
				return cmdutil.NewSilentError(cmdutil.ExitAuth,
					fmt.Errorf("not logged in to profile %q (run `modjo auth login`)", profileName))
			}
			io := f.IOStreams
			io.Errf("%s Profile %s\n", io.Green("✓"), io.Bold(profileName))
			fmt.Fprintf(io.Out, "  Token:     %s\n", auth.Fingerprint(cred.Token))
			fmt.Fprintf(io.Out, "  Method:    %s\n", orDash(cred.Method))
			fmt.Fprintf(io.Out, "  Workspace: %s\n", orDash(cred.Workspace))
			if len(cred.Scopes) > 0 {
				fmt.Fprintf(io.Out, "  Scopes:    %s\n", strings.Join(cred.Scopes, ", "))
			}
			if !cred.Expiry.IsZero() {
				fmt.Fprintf(io.Out, "  Expires:   %s\n", cred.Expiry.Format(time.RFC3339))
			}
			return nil
		},
	}
}

func newRefreshCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Force an OAuth token refresh",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, _ := f.ActiveProfile()
			store, err := f.CredentialStore()
			if err != nil {
				return err
			}
			cred, err := store.Get(profileName)
			if err != nil {
				return err
			}
			if cred.Method != auth.MethodOAuth {
				return fmt.Errorf("profile %q uses an API key; nothing to refresh", profileName)
			}
			return fmt.Errorf("OAuth is not yet available for your workspace")
		},
	}
}

func newSwitchCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "switch <profile>",
		Short: "Change the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			cfg.ProfileOrDefault(args[0])
			cfg.ActiveProfile = args[0]
			if err := f.SaveConfig(cfg); err != nil {
				return err
			}
			f.IOStreams.Errf("%s Active profile is now %q\n", f.IOStreams.Green("✓"), args[0])
			return nil
		},
	}
}

func newTokenCmd(f *cmdutil.Factory) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Print the current access token (guarded)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return cmdutil.NewUsageError(fmt.Errorf(
					"printing the token exposes full-workspace access; re-run with --confirm"))
			}
			tok, err := f.TokenSource()()
			if err != nil {
				return err
			}
			fmt.Fprintln(f.IOStreams.Out, tok)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm exposing the full token")
	return cmd
}

// --- helpers ---

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func mustResolve(f *cmdutil.Factory, key string) string {
	r, err := f.Resolver()
	if err != nil {
		return config.DefaultBaseURL
	}
	return r.Resolve(key, "", cmdutil.OSLookup)
}

// validationClient builds an http.Client that injects the candidate key, so we
// can validate it before persisting.
func validationClient(f *cmdutil.Factory, key string) *http.Client {
	return httpclient.New(httpclient.Options{
		Token:      func() (string, error) { return key, nil },
		MaxRetries: 1,
		Insecure:   f.Flags.Insecure,
	})
}
