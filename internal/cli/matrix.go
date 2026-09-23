package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ravibagri5/platform-bom/internal/analysis"
)

var statusMark = map[string]string{
	analysis.StatusAligned:   "✓",
	analysis.StatusDrift:     "≠",
	analysis.StatusMissing:   "✗",
	analysis.StatusUntracked: "+",
}

func newMatrixCmd(g *globals) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "matrix",
		Short: "Show component versions across environments",
		Long: `Show component versions across environments.

Marks: ✓ matches the environment's target release, ≠ drift, ✗ missing,
+ running but not part of the target release. ↑ means a newer upstream release exists.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := g.service()
			if err != nil {
				return err
			}
			in, err := svc.Input(cmd.Context(), false)
			if err != nil {
				return err
			}
			m := analysis.BuildMatrix(in)
			if done, err := printStructured(cmd.OutOrStdout(), output, m); done {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Platform %s", svc.Platform.Metadata.Name)
			if m.CurrentRelease != "" {
				fmt.Fprintf(w, "  current release %s", m.CurrentRelease)
			}
			fmt.Fprintln(w)
			fmt.Fprintln(w)

			t := newTable(w)
			fmt.Fprintln(t, "ENVIRONMENT\tKUBERNETES\tTARGET\tRUNNING\tSTATUS")
			for _, e := range m.Environments {
				st := "compliant"
				switch {
				case len(e.Errors) > 0 && e.Cluster.Version == "":
					st = "unreachable"
				case e.TargetRelease == "":
					st = "no target"
				case !e.Compliant:
					st = fmt.Sprintf("%d drift, %d missing", e.Drift, e.Missing)
				}
				fmt.Fprintf(t, "%s\t%s %s\t%s\t%s\t%s\n", e.Name, dash(e.Cluster.Version), e.Cluster.Distribution,
					dash(e.TargetRelease), dash(e.MatchedRelease), st)
			}
			_ = t.Flush()
			fmt.Fprintln(w)

			t = newTable(w)
			header := []string{"CATEGORY", "COMPONENT"}
			for _, e := range m.Environments {
				header = append(header, strings.ToUpper(e.Name))
			}
			header = append(header, "RELEASE", "LATEST")
			fmt.Fprintln(t, strings.Join(header, "\t"))
			for _, r := range m.Rows {
				cols := []string{r.Category, r.DisplayName}
				for _, e := range m.Environments {
					c := r.Cells[e.Name]
					v := dash(c.Version)
					if mark := statusMark[c.Status]; mark != "" {
						v += " " + mark
					}
					if c.Freshness != analysis.FreshLatest && c.Freshness != analysis.FreshUnknown {
						v += " ↑"
					}
					cols = append(cols, v)
				}
				cols = append(cols, dash(r.Declared), dash(r.Latest))
				fmt.Fprintln(t, strings.Join(cols, "\t"))
			}
			_ = t.Flush()
			s := m.Summary
			fmt.Fprintf(w, "\n%d components, %d aligned, %d drift, %d missing, %d behind upstream (%d unsupported)\n",
				s.Components, s.Aligned, s.Drift, s.Missing, s.Behind, s.Unsupported)
			return nil
		},
	}
	addOutputFlag(cmd, &output)
	return cmd
}

func newDriftCmd(g *globals) *cobra.Command {
	var exitCode bool
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "List components that differ from each environment's target release",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := g.service()
			if err != nil {
				return err
			}
			in, err := svc.Input(cmd.Context(), false)
			if err != nil {
				return err
			}
			m := analysis.BuildMatrix(in)
			w := cmd.OutOrStdout()
			t := newTable(w)
			fmt.Fprintln(t, "ENVIRONMENT\tTARGET\tCOMPONENT\tEXPECTED\tACTUAL\tSTATUS")
			found := 0
			for _, e := range m.Environments {
				for _, r := range m.Rows {
					c := r.Cells[e.Name]
					if c.Status != analysis.StatusDrift && c.Status != analysis.StatusMissing {
						continue
					}
					found++
					fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Name, e.TargetRelease, r.DisplayName, c.Expected, dash(c.Version), c.Status)
				}
			}
			if found == 0 {
				fmt.Fprintln(w, "No drift: every environment matches its target release.")
				return nil
			}
			_ = t.Flush()
			if exitCode {
				return ExitError{Code: 2}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "exit with status 2 when drift is found")
	return cmd
}
