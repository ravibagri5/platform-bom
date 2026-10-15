package cli

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/ravibagri5/platform-bom/internal/landscape"
)

func addLandscapeFlag(cmd *cobra.Command, url *string) {
	cmd.Flags().StringVar(url, "landscape-url", envOr("PBOM_LANDSCAPE_URL", landscape.DefaultURL),
		"CNCF landscape.yml used to classify tools the catalog does not know; empty uses only the built-in snapshot")
}

// refreshLandscape updates the CNCF landscape from url. Failure is not an
// error: the built-in snapshot keeps classifying.
func refreshLandscape(ctx context.Context, url string) {
	if url == "" {
		return
	}
	n, err := landscape.Refresh(ctx, url)
	if err != nil {
		slog.Warn("cannot refresh the CNCF landscape; using the built-in snapshot", "url", url, "error", err)
		return
	}
	slog.Info("refreshed the CNCF landscape", "projects", n)
}
