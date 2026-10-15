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

	"github.com/ravibagri5/platform-bom/internal/analysis"
	"github.com/ravibagri5/platform-bom/internal/auth"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/landscape"
	"github.com/ravibagri5/platform-bom/internal/metrics"
	"github.com/ravibagri5/platform-bom/internal/operator"
	"github.com/ravibagri5/platform-bom/internal/server"
	"github.com/ravibagri5/platform-bom/internal/service"
	"github.com/ravibagri5/platform-bom/internal/ui"
)

func newServeCmd(g *globals) *cobra.Command {
	var addr string
	var interval, configInterval time.Duration
	var configSource, namespace, platformName, landscapeURL, crdContext string
	var metricsAddr, logFormat, logLevel string
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
			if debug {
				logLevel = "debug"
			}
			if err := setupLogging(logFormat, logLevel); err != nil {
				return err
			}
			metrics.SetBuildInfo(Version)

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

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			var current atomic.Pointer[service.Service]
			var handler atomic.Pointer[serveHandler]
			handler.Store(&serveHandler{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/healthz" {
					_, _ = w.Write([]byte("ok"))
					return
				}
				http.Error(w, "PBOM resources are not ready", http.StatusServiceUnavailable)
			})})
			errCh := make(chan error, 1)
			use := func(s *service.Service) {
				current.Store(s)
				handler.Store(&serveHandler{server.New(s, ui.FS())})
				metrics.ConfigReload(true)
				go func() { _, _ = s.Input(ctx, false) }()
			}
			switch configSource {
			case "file":
				svc, err := g.service()
				if err != nil {
					return err
				}
				use(svc)
				if configInterval > 0 {
					go watchConfig(ctx, g, configInterval, use)
				}
			case "crd":
				if namespace == "" {
					return errors.New("--crd-namespace or POD_NAMESPACE is required with --config-source=crd")
				}
				go func() {
					if err := operator.Run(ctx, operator.Options{
						Namespace: namespace, PlatformName: platformName, Context: crdContext,
						Service: service.Options{NoUpstream: g.noUpstream},
					}, use); err != nil && ctx.Err() == nil {
						errCh <- err
					}
				}()
			default:
				return fmt.Errorf("unknown config source %q (expected file or crd)", configSource)
			}

			if landscapeURL != "" {
				go func() {
					before := landscape.Source()
					refreshLandscape(ctx, landscapeURL)
					if svc := current.Load(); svc != nil && landscape.Source() != before {
						_, _ = svc.Input(ctx, true)
					}
				}()
			}

			go func() {
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
						if svc := current.Load(); svc != nil {
							_, _ = svc.Input(ctx, true)
						}
					}
				}
			}()

			srv := &http.Server{
				Addr: addr,
				Handler: metrics.InstrumentHTTP(authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handler.Load().handler.ServeHTTP(w, r)
				}))),
				ReadHeaderTimeout: 10 * time.Second,
				WriteTimeout:      5 * time.Minute,
				IdleTimeout:       2 * time.Minute,
			}
			go func() { errCh <- srv.ListenAndServe() }()
			slog.Info("serving platform", "version", Version, "configSource", configSource, "url", "http://"+strings.Replace(addr, "0.0.0.0", "localhost", 1))

			// Metrics get their own listener: outside sign-in, and firewalled separately.
			var metricsSrv *http.Server
			if metricsAddr != "" {
				metrics.Registry.MustRegister(server.PlatformCollector{Input: func() (analysis.Input, bool) {
					if s := current.Load(); s != nil {
						return s.CachedInput()
					}
					return analysis.Input{}, false
				}})
				mux := http.NewServeMux()
				mux.Handle("GET /metrics", metrics.Handler())
				metricsSrv = &http.Server{Addr: metricsAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
				go func() { errCh <- metricsSrv.ListenAndServe() }()
				slog.Info("serving metrics", "url", "http://"+strings.Replace(metricsAddr, "0.0.0.0", "localhost", 1)+"/metrics")
			}

			select {
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			case <-ctx.Done():
				shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if metricsSrv != nil {
					_ = metricsSrv.Shutdown(shutdown)
				}
				return srv.Shutdown(shutdown)
			}
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "listen address")
	cmd.Flags().StringVar(&configSource, "config-source", "file", "configuration source: file or crd")
	cmd.Flags().StringVar(&namespace, "crd-namespace", envOr("POD_NAMESPACE", ""), "namespace containing PBOM custom resources")
	cmd.Flags().StringVar(&crdContext, "crd-context", "", "kubeconfig context of the cluster holding PBOM custom resources (default: current context, or in-cluster)")
	cmd.Flags().StringVar(&platformName, "platform-name", "", "Platform resource to serve (required when the namespace contains multiple Platforms)")
	addLandscapeFlag(cmd, &landscapeURL)
	cmd.Flags().DurationVar(&interval, "refresh-interval", 10*time.Minute, "background rediscovery interval (0 disables)")
	cmd.Flags().DurationVar(&configInterval, "config-check-interval", 15*time.Second, "how often to check configuration files for changes and reload (0 disables)")
	cmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging (same as --log-level debug)")
	cmd.Flags().StringVar(&logFormat, "log-format", "text", "log format: text or json")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn or error")
	cmd.Flags().StringVar(&metricsAddr, "metrics-addr", "", "listen address for Prometheus metrics at /metrics, such as 0.0.0.0:9090 (empty disables)")
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

type serveHandler struct{ handler http.Handler }

// setupLogging sends structured logs to stderr; json suits log collectors.
func setupLogging(format, level string) error {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return fmt.Errorf("--log-level: %w", err)
	}
	opts := &slog.HandlerOptions{Level: lvl}
	switch format {
	case "text":
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
	case "json":
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, opts)))
	default:
		return fmt.Errorf("--log-format: unknown format %q (expected text or json)", format)
	}
	return nil
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
