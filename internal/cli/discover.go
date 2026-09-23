package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/discovery"
)

func newDiscoverCmd(g *globals) *cobra.Command {
	var env, kubeContext, kubeconfig, output string
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover platform components in one or all environments",
		Long: `Discover platform components.

With --context or --kubeconfig a cluster is discovered directly using the builtin
catalog, without a platform file. Export the result with -o yaml to use it as an
environment's inventoryFile.`,
		Example: `  pbom discover --context kind-dev
  pbom discover --env prod -o yaml > inventories/prod.yaml`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			var invs []*api.Inventory
			if kubeContext != "" || kubeconfig != "" {
				cat, err := catalog.Load("")
				if err != nil {
					return err
				}
				cfg, err := discovery.RESTConfig(kubeconfig, kubeContext)
				if err != nil {
					return err
				}
				name := env
				if name == "" {
					name = kubeContext
				}
				inv, err := discovery.Discover(ctx, cfg, cat, discovery.Options{Environment: name, Context: kubeContext})
				if err != nil {
					return err
				}
				invs = append(invs, inv)
			} else {
				svc, err := g.service()
				if err != nil {
					return err
				}
				envs := svc.Platform.Spec.Environments
				if env != "" {
					e, err := svc.Environment(env)
					if err != nil {
						return err
					}
					envs = []api.Environment{*e}
				}
				for i := range envs {
					inv, err := svc.DiscoverEnvironment(ctx, &envs[i])
					if err != nil {
						return fmt.Errorf("%s: %w", envs[i].Name, err)
					}
					invs = append(invs, inv)
				}
			}
			var v any = invs
			if len(invs) == 1 {
				v = invs[0]
			}
			if done, err := printStructured(cmd.OutOrStdout(), output, v); done {
				return err
			}
			for _, inv := range invs {
				printInventory(cmd.OutOrStdout(), inv)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&env, "env", "e", "", "environment name")
	cmd.Flags().StringVar(&kubeContext, "context", "", "kubeconfig context to discover directly")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig file to discover directly")
	addOutputFlag(cmd, &output)
	return cmd
}

func printInventory(w io.Writer, inv *api.Inventory) {
	c := inv.Cluster
	fmt.Fprintf(w, "Environment %s  (kubernetes %s %s, %d nodes)\n\n", inv.Environment, dash(c.Version), c.Distribution, c.Nodes)
	comps := append([]api.DiscoveredComponent(nil), inv.Components...)
	sort.SliceStable(comps, func(i, j int) bool {
		return catalog.CategoryRank(comps[i].Category) < catalog.CategoryRank(comps[j].Category)
	})
	t := newTable(w)
	fmt.Fprintln(t, "CATEGORY\tCOMPONENT\tVERSION\tEVIDENCE")
	for _, comp := range comps {
		name := comp.DisplayName
		if !comp.Known {
			name += " (unclassified)"
		}
		fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", comp.Category, name, dash(comp.Version), evidenceSummary(comp.Evidence))
	}
	_ = t.Flush()
	for _, e := range inv.Errors {
		fmt.Fprintf(w, "warning: %s\n", e)
	}
	fmt.Fprintln(w)
}

func evidenceSummary(ev []api.Evidence) string {
	counts := map[string]int{}
	for _, e := range ev {
		counts[e.Source]++
	}
	parts := make([]string, 0, len(counts))
	for s, n := range counts {
		parts = append(parts, fmt.Sprintf("%s×%d", s, n))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}
