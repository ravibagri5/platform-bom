package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ravibagri5/platform-bom/internal/release"
)

func newReleaseCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Manage versioned platform releases",
	}
	cmd.AddCommand(newReleaseListCmd(g), newReleaseShowCmd(g), newReleaseCreateCmd(g), newReleaseDiffCmd(g))
	return cmd
}

func newReleaseListCmd(g *globals) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List platform releases, newest first",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := g.service()
			if err != nil {
				return err
			}
			rels, err := svc.Releases()
			if err != nil {
				return err
			}
			if done, err := printStructured(cmd.OutOrStdout(), output, rels); done {
				return err
			}
			t := newTable(cmd.OutOrStdout())
			fmt.Fprintln(t, "RELEASE\tDATE\tCOMPONENTS\tOFFERINGS\tSUMMARY")
			for _, r := range rels {
				fmt.Fprintf(t, "%s\t%s\t%d\t%d\t%s\n", r.Metadata.Name, dash(r.Spec.Date), len(r.Spec.Components), len(r.Spec.Offerings), r.Spec.Summary)
			}
			return t.Flush()
		},
	}
	addOutputFlag(cmd, &output)
	return cmd
}

func newReleaseShowCmd(g *globals) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "show NAME",
		Short: "Show a platform release",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := g.service()
			if err != nil {
				return err
			}
			rels, err := svc.Releases()
			if err != nil {
				return err
			}
			for i := range rels {
				if rels[i].Metadata.Name == args[0] {
					if output == "table" {
						output = "yaml"
					}
					_, err := printStructured(cmd.OutOrStdout(), output, rels[i])
					return err
				}
			}
			return fmt.Errorf("release %q not found", args[0])
		},
	}
	addOutputFlag(cmd, &output)
	return cmd
}

func newReleaseCreateCmd(g *globals) *cobra.Command {
	var fromEnv, summary string
	var highlights []string
	var force bool
	cmd := &cobra.Command{
		Use:   "create NAME --from-env ENV",
		Short: "Cut a platform release from the components running in an environment",
		Example: `  pbom release create v1.2.0 --from-env staging \
    --summary "Kubernetes 1.35 and Crossplane 2.1" \
    --highlight "Kubernetes upgraded to 1.35"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !release.ValidName(args[0]) {
				return fmt.Errorf("invalid release name %q", args[0])
			}
			svc, err := g.service()
			if err != nil {
				return err
			}
			env, err := svc.Environment(fromEnv)
			if err != nil {
				return err
			}
			inv, err := svc.DiscoverEnvironment(cmd.Context(), env)
			if err != nil {
				return err
			}
			rel := release.FromInventory(args[0], inv, svc.Platform.Spec.Offerings, summary)
			rel.Spec.Highlights = highlights
			path, err := release.Write(svc.ReleasesDir(), rel, force)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created release %s with %d components from %s: %s\n",
				args[0], len(rel.Spec.Components), fromEnv, path)
			fmt.Fprintln(cmd.OutOrStdout(), "Edit the file to add release notes, then commit it.")
			return nil
		},
	}
	cmd.Flags().StringVar(&fromEnv, "from-env", "", "environment to snapshot")
	cmd.Flags().StringVar(&summary, "summary", "", "one line release summary")
	cmd.Flags().StringArrayVar(&highlights, "highlight", nil, "release highlight (repeatable)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing release")
	_ = cmd.MarkFlagRequired("from-env")
	return cmd
}

var changeMark = map[string]string{
	release.ChangeAdded:     "+",
	release.ChangeRemoved:   "-",
	release.ChangeUpgraded:  "↑",
	release.ChangeDowngrade: "↓",
	release.ChangeUnchanged: " ",
}

func newReleaseDiffCmd(g *globals) *cobra.Command {
	var output string
	var all bool
	cmd := &cobra.Command{
		Use:   "diff FROM TO",
		Short: "Compare two platform releases",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := g.service()
			if err != nil {
				return err
			}
			d, err := svc.Diff(args[0], args[1])
			if err != nil {
				return err
			}
			if done, err := printStructured(cmd.OutOrStdout(), output, d); done {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Platform %s → %s\n\n", d.From, d.To)
			t := newTable(w)
			for _, c := range d.Components {
				if c.Change == release.ChangeUnchanged && !all {
					continue
				}
				fmt.Fprintf(t, "%s\t%s\t%s\t→\t%s\n", changeMark[c.Change], c.DisplayName, dash(c.From), dash(c.To))
			}
			_ = t.Flush()
			if len(d.OfferingsAdded)+len(d.OfferingsRemoved) > 0 {
				fmt.Fprintln(w)
			}
			if len(d.OfferingsAdded) > 0 {
				fmt.Fprintf(w, "New offerings: %s\n", strings.Join(d.OfferingsAdded, ", "))
			}
			if len(d.OfferingsRemoved) > 0 {
				fmt.Fprintf(w, "Removed offerings: %s\n", strings.Join(d.OfferingsRemoved, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include unchanged components")
	addOutputFlag(cmd, &output)
	return cmd
}
