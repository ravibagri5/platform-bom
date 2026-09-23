package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/server"
	"github.com/ravibagri5/platform-bom/internal/ui"
)

func newServeCmd(g *globals) *cobra.Command {
	var addr string
	var interval time.Duration
	var debug bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the platform web UI and API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			level := slog.LevelInfo
			if debug {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

			svc, err := g.service()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

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
						_, _ = svc.Input(ctx, true)
					}
				}
			}()

			srv := &http.Server{
				Addr:              addr,
				Handler:           server.New(svc, ui.FS()),
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
	cmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")
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
