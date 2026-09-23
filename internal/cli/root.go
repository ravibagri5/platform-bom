// Package cli implements the pbom command line.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/service"
)

// Version is set at build time.
var Version = "dev"

// ExitError carries a process exit code without printing a message.
type ExitError struct{ Code int }

func (e ExitError) Error() string { return fmt.Sprintf("exit code %d", e.Code) }

type globals struct {
	config     string
	noUpstream bool
}

func (g *globals) service() (*service.Service, error) {
	return service.New(g.config, service.Options{NoUpstream: g.noUpstream})
}

// NewRoot returns the root command.
func NewRoot() *cobra.Command {
	g := &globals{}
	root := &cobra.Command{
		Use:   "pbom",
		Short: "Platform Bill of Materials: your internal platform as a versioned product",
		Long: `pbom discovers the components that make up your platform across environments,
tracks them against versioned platform releases and upstream releases, and
presents the platform as a product.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&g.config, "config", "c", envOr("PBOM_CONFIG", "pbom.yaml"), "platform definition file")
	root.PersistentFlags().BoolVar(&g.noUpstream, "no-upstream", false, "do not fetch upstream release information")

	root.AddCommand(
		newDiscoverCmd(g),
		newMatrixCmd(g),
		newDriftCmd(g),
		newUpdatesCmd(g),
		newReleaseCmd(g),
		newCatalogCmd(g),
		newServeCmd(g),
		&cobra.Command{
			Use:   "version",
			Short: "Print the pbom version",
			Run:   func(cmd *cobra.Command, _ []string) { fmt.Fprintln(cmd.OutOrStdout(), Version) },
		},
	)
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	err := NewRoot().Execute()
	var exit ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.Code
	default:
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func addOutputFlag(cmd *cobra.Command, p *string) {
	cmd.Flags().StringVarP(p, "output", "o", "table", "output format: table, yaml or json")
}

// printStructured writes v as yaml or json and reports whether it handled the format.
func printStructured(w io.Writer, format string, v any) (bool, error) {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return true, enc.Encode(v)
	case "yaml":
		data, err := yaml.Marshal(v)
		if err != nil {
			return true, err
		}
		_, err = w.Write(data)
		return true, err
	case "table", "":
		return false, nil
	}
	return true, fmt.Errorf("unknown output format %q", format)
}

func newTable(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
