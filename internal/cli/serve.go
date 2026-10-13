package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ravibagri5/platform-bom/internal/auth"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/server"
	"github.com/ravibagri5/platform-bom/internal/service"
	"github.com/ravibagri5/platform-bom/internal/ui"
)

func newServeCmd(g *globals) *cobra.Command {
	var addr string
	var interval, configInterval time.Duration
	var debug bool
	var authCfg auth.Config
	var trustedProxies []string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the platform web UI and API",
		Long: `Serve the platform web UI and API.

With --auth oidc, users sign in with an OpenID Connect provider. The client
secret is read from PBOM_OIDC_CLIENT_SECRET (leave it unset for a public
client) and the session encryption key from PBOM_SESSION_SECRET (at least 32
bytes; without it, users sign in again after a restart).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			level := slog.LevelInfo
			if debug {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

			for _, p := range trustedProxies {
				prefix, err := netip.ParsePrefix(p)
				if err != nil {
					return fmt.Errorf("--auth-trusted-proxies: %w", err)
				}
				authCfg.TrustedProxies = append(authCfg.TrustedProxies, prefix)
			}
			authCfg.ClientSecret = os.Getenv("PBOM_OIDC_CLIENT_SECRET")
			authCfg.SessionSecret = []byte(os.Getenv("PBOM_SESSION_SECRET"))
			authenticate, err := auth.New(authCfg)
			if err != nil {
				return err
			}

			svc, err := g.service()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			var current atomic.Pointer[service.Service]
			var handler atomic.Value
			use := func(s *service.Service) {
				current.Store(s)
				handler.Store(server.New(s, ui.FS()))
			}
			use(svc)
			if configInterval > 0 {
				go watchConfig(ctx, g, configInterval, func(s *service.Service) {
					use(s)
					go func() { _, _ = s.Input(ctx, false) }()
				})
			}

			go func() {
				// Warm the caches so the first page load is fast.
				_, _ = svc.Input(ctx, false)
				if interval <= 0 {
					return
				}
				tick := time.NewTicker(interval)
				defer tick.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-tick.C:
						_, _ = current.Load().Input(ctx, true)
					}
				}
			}()

			srv := &http.Server{
				Addr: addr,
				Handler: authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handler.Load().(http.Handler).ServeHTTP(w, r)
				})),
				ReadHeaderTimeout: 10 * time.Second,
				WriteTimeout:      5 * time.Minute,
				IdleTimeout:       2 * time.Minute,
			}
			errCh := make(chan error, 1)
			go func() { errCh <- srv.ListenAndServe() }()
			slog.Info("serving platform", "platform", svc.Platform.Metadata.Name, "url", "http://"+strings.Replace(addr, "0.0.0.0", "localhost", 1))

			select {
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			case <-ctx.Done():
				shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				return srv.Shutdown(shutdown)
			}
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "listen address")
	cmd.Flags().DurationVar(&interval, "refresh-interval", 10*time.Minute, "background rediscovery interval (0 disables)")
	cmd.Flags().DurationVar(&configInterval, "config-check-interval", 15*time.Second, "how often to check configuration files for changes and reload (0 disables)")
	cmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")
	cmd.Flags().StringVar(&authCfg.Mode, "auth", auth.ModeNone, "authentication: none, header (authenticating proxy) or oidc")
	cmd.Flags().StringVar(&authCfg.Header, "auth-header", "X-Forwarded-User", "header with the user name from the authenticating proxy (--auth header)")
	cmd.Flags().StringSliceVar(&trustedProxies, "auth-trusted-proxies", nil, "CIDRs the authenticating proxy connects from; other clients are refused (--auth header)")
	cmd.Flags().StringVar(&authCfg.Issuer, "oidc-issuer", "", "OpenID Connect issuer URL")
	cmd.Flags().StringVar(&authCfg.ClientID, "oidc-client-id", "", "OpenID Connect client ID")
	cmd.Flags().StringVar(&authCfg.RedirectURL, "oidc-redirect-url", "", "public URL of the callback, such as https://pbom.example.com"+auth.CallbackPath)
	cmd.Flags().StringSliceVar(&authCfg.Scopes, "oidc-scopes", []string{"openid", "profile", "email"}, "scopes to request")
	cmd.Flags().StringVar(&authCfg.UsernameClaim, "oidc-username-claim", "", "claim shown as the user's name (default: name, preferred_username, email or sub)")
	cmd.Flags().StringVar(&authCfg.GroupsClaim, "oidc-groups-claim", "groups", "claim that lists the user's groups")
	cmd.Flags().StringSliceVar(&authCfg.AllowedGroups, "oidc-allowed-groups", nil, "only members of these groups may sign in (default: anyone the provider signs in)")
	cmd.Flags().DurationVar(&authCfg.SessionDuration, "session-duration", 12*time.Hour, "how long a sign-in lasts")
	return cmd
}

func newCatalogCmd(g *globals) *cobra.Command {
	var output, dir string
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "List known component definitions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var cat *catalog.Catalog
			if svc, err := g.service(); err == nil && dir == "" {
				cat = svc.Catalog
			} else if cat, err = catalog.Load(dir); err != nil {
				return err
			}
			if done, err := printStructured(cmd.OutOrStdout(), output, cat.List()); done {
				return err
			}
			t := newTable(cmd.OutOrStdout())
			fmt.Fprintln(t, "NAME\tDISPLAY NAME\tCATEGORY\tUPSTREAM")
			for _, c := range cat.List() {
				up := ""
				if c.Spec.Upstream != nil {
					up = c.Spec.Upstream.GitHub
				}
				fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", c.Metadata.Name, c.Spec.DisplayName, c.Spec.Category, dash(up))
			}
			return t.Flush()
		},
	}
	cmd.Flags().StringVar(&dir, "components-dir", "", "extra component definitions directory")
	addOutputFlag(cmd, &output)
	return cmd
}
