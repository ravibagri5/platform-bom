package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ravibagri5/platform-bom/internal/analysis"
)

func newUpdatesCmd(g *globals) *cobra.Command {
	var output string
	var why bool
	cmd := &cobra.Command{
		Use:   "updates",
		Short: "Recommend upgrades based on upstream releases",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := g.service()
			if err != nil {
				return err
			}
			in, err := svc.Input(cmd.Context(), false)
			if err != nil {
				return err
			}
			ups := analysis.BuildUpdates(in)
			if done, err := printStructured(cmd.OutOrStdout(), output, ups); done {
				return err
			}
			w := cmd.OutOrStdout()
			t := newTable(w)
			fmt.Fprintln(t, "COMPONENT\tCURRENT\tLATEST\tSTATUS\tNEWER RELEASES")
			for _, u := range ups {
				latest := ""
				if u.Latest != nil {
					latest = u.Latest.Version
				}
				fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%d\n", u.DisplayName, dash(u.Current), dash(latest), u.Freshness, len(u.Newer))
			}
			_ = t.Flush()
			if why {
				for _, u := range ups {
					if u.Freshness == analysis.FreshLatest || u.Freshness == analysis.FreshUnknown {
						continue
					}
					fmt.Fprintf(w, "\n%s %s\n  - %s\n", u.DisplayName, u.Current, strings.Join(u.Reasons, "\n  - "))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&why, "why", false, "explain each recommendation")
	addOutputFlag(cmd, &output)
	return cmd
}
