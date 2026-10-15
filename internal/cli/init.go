package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/catalog"
	"github.com/ravibagri5/platform-bom/internal/discovery"
	"github.com/ravibagri5/platform-bom/internal/draft"
	"github.com/ravibagri5/platform-bom/internal/release"
)

var (
	componentNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]{0,61}[a-z0-9])?$`)
	envNameRE       = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

func newInitCmd() *cobra.Command {
	var kubeContext, kubeconfig, env, name, version, namespace, landscapeURL string
	var drafts, force bool
	cmd := &cobra.Command{
		Use:   "init DIR",
		Short: "Create a platform directory from what a cluster runs",
		Long: `Discover a cluster and create a platform directory for the Helm chart:

  DIR/platform.yaml             Platform: name, offerings, environments
  DIR/releases/VERSION.yaml     PlatformRelease: what the cluster runs today
  DIR/components/*.yaml         Component drafts for unclassified Helm releases (--components)
  DIR/kustomization.yaml        applies the PBOM custom resources with kubectl apply -k

The environment is marked inCluster, for pbom running in that cluster with the
Helm chart.`,
		Example: `  pbom init pbom-config --context my-cluster --env prod
  kubectl apply -k pbom-config`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if !envNameRE.MatchString(env) {
				return fmt.Errorf("invalid environment name %q: use lowercase letters, digits and dashes", env)
			}
			if !release.ValidName(version) {
				return fmt.Errorf("invalid release name %q", version)
			}
			if _, err := os.Stat(filepath.Join(dir, "platform.yaml")); err == nil && !force {
				return fmt.Errorf("%s already contains platform.yaml (use --force to overwrite)", dir)
			}

			refreshLandscape(cmd.Context(), landscapeURL)
			cat, err := catalog.Load("")
			if err != nil {
				return err
			}
			cfg, err := discovery.RESTConfig(kubeconfig, kubeContext)
			if err != nil {
				return err
			}
			inv, err := discovery.Discover(cmd.Context(), cfg, cat, discovery.Options{Environment: env, Context: kubeContext})
			if err != nil {
				return err
			}

			offerings := initOfferings(inv)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			desc := draft.Description([]*api.Inventory{inv}, offerings)
			if err := writePlatform(filepath.Join(dir, "platform.yaml"), name, env, version, desc, offerings); err != nil {
				return err
			}

			summary := "Baseline of " + env
			if kubeContext != "" {
				summary += " (" + kubeContext + ")"
			}
			rel := release.FromInventory(version, inv, offerings, summary)
			if _, err := release.Write(filepath.Join(dir, "releases"), rel, force); err != nil {
				return err
			}

			var drafted []string
			if drafts {
				if drafted, err = writeComponentDrafts(filepath.Join(dir, "components"), inv, force); err != nil {
					return err
				}
			}
			if _, err := writeKustomization(dir, namespace); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Created %s from %d components in %s:\n", dir, len(inv.Components), env)
			fmt.Fprintf(out, "  %s/platform.yaml\n  %s/releases/%s.yaml (%d components)\n", dir, dir, version, len(rel.Spec.Components))
			for _, d := range drafted {
				fmt.Fprintf(out, "  %s/components/%s.yaml (draft)\n", dir, d)
			}
			fmt.Fprintf(out, "  %s/kustomization.yaml\n", dir)
			for _, e := range inv.Errors {
				fmt.Fprintf(out, "warning: %s\n", e)
			}
			fmt.Fprintf(out, "\nReview the files, then apply them with: kubectl apply -k %s\n", dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&kubeContext, "context", "", "kubeconfig context of the cluster (default: current context)")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	cmd.Flags().StringVar(&env, "env", "prod", "environment name for the cluster")
	cmd.Flags().StringVar(&name, "name", "my-platform", "platform name")
	cmd.Flags().StringVar(&version, "release", "1.0.0", "version of the first platform release")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "platform-bom", "namespace pbom is installed in")
	cmd.Flags().BoolVar(&drafts, "components", false, "write a draft Component for each unclassified Helm release")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	addLandscapeFlag(cmd, &landscapeURL)
	return cmd
}

// platformTemplate keeps a readable field order, with environments last so
// new environments can be appended. Values are JSON-quoted, which is valid YAML.
const platformTemplate = `# The platform PBOM presents: what application teams get, and where it runs.
# "pbom init" drafted it from what it discovered. Rewrite the texts in your own
# words and replace these comments with your own notes; PBOM ignores comments.
#
# Releases are in releases/, extra Component definitions in components/.
# After adding or removing files, run "pbom kustomize" on this directory, then
# "kubectl apply -k" it.
apiVersion: pbom.dev/v1alpha1
kind: Platform
metadata:
  name: %[1]s
  annotations:
    # The platform's name in the UI.
    pbom.dev/display-name: %[2]s
spec:
  # One line under the name on the home page.
  tagline: %[8]s
  # Who the platform is for and what it gives them.
  description: %[7]s
  # Who runs the platform and where users ask for help:
  # owners:
  #   - name: Platform Engineering
  #     email: platform@example.com
  #     channel: "#platform-help"
  # Links shown on the home page:
  # links:
  #   - title: Getting started
  #     url: https://docs.example.com/platform
  # Promises the platform makes to its users:
  # guarantees:
  #   - Every change delivered through GitOps
  #   - Secrets never stored in Git
  # Longer markdown shown on the home page:
  # readme: |
  #   ## Getting help
  #   Ask in #platform-help.
  componentsDir: components
  # Offerings are the capabilities teams get. One per category was drafted from
  # the components found; rename, split or merge them and describe them for your
  # users. The UI groups them by category, in the order they appear here.
  #   status:     planned, alpha, beta, ga or deprecated
  #   components: names from the Components page or "pbom catalog"; they show
  #               where an offering is available and which versions provide it
  offerings:
%[6]s    # Something you plan to offer but do not run yet:
    # - name: tracing
    #   displayName: Distributed tracing
    #   category: observability
    #   status: planned
    #   description: Follow a request across services with OpenTelemetry.
  # Where the platform runs, one cluster per environment. targetRelease is the
  # release in releases/ the environment should run; PBOM reports drift from it.
  # More clusters: kubeconfig and kubeContext, server and tokenFile, or an
  # exported inventory; see docs/multi-cluster.md.
  environments:
    - name: %[3]s
      displayName: %[4]s
      # The cluster PBOM runs in, read with its own service account.
      inCluster: true
      targetRelease: %[5]s
`

// initOfferings drafts one offering per category of the recognised top-level
// components, falling back to the Kubernetes runtime when nothing was recognised.
func initOfferings(inv *api.Inventory) []api.Offering {
	if out := draft.Offerings([]*api.Inventory{inv}); len(out) > 0 {
		return out
	}
	return []api.Offering{{
		Name: "runtime", DisplayName: draft.Title("runtime"), Category: "runtime", Status: "ga",
		Components: []string{"kubernetes"},
	}}
}

func writePlatform(path, name, env, version, description string, offerings []api.Offering) error {
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	var ob strings.Builder
	for _, o := range offerings {
		comps := make([]string, len(o.Components))
		for i, c := range o.Components {
			comps[i] = q(c)
		}
		fmt.Fprintf(&ob, "    - name: %s\n      displayName: %s\n      category: %s\n      status: %s\n",
			q(o.Name), q(o.DisplayName), q(o.Category), q(o.Status))
		if o.Description != "" {
			fmt.Fprintf(&ob, "      description: %s\n", q(o.Description))
		}
		fmt.Fprintf(&ob, "      components: [%s]\n", strings.Join(comps, ", "))
	}
	if description == "" {
		description = "An internal platform that gives application teams a paved road to production."
	}
	tagline := "The paved road for shipping services on " + titleCase(name) + "."
	data := fmt.Sprintf(platformTemplate, q(name), q(titleCase(name)), q(env), q(titleCase(env)), q(version), ob.String(), q(description), q(tagline))
	p := &api.Platform{}
	if err := yaml.UnmarshalStrict([]byte(data), p); err != nil {
		return fmt.Errorf("generated platform.yaml is invalid: %w", err)
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

func writeComponentDrafts(dir string, inv *api.Inventory, force bool) ([]string, error) {
	var names []string
	for _, c := range inv.Components {
		if c.Known || c.Kind != "helm" || !componentNameRE.MatchString(c.Name) {
			continue
		}
		comp := api.Component{
			TypeMeta: api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindComponent},
			Metadata: api.Metadata{Name: c.Name},
			Spec: api.ComponentSpec{
				DisplayName: titleCase(c.Name),
				Category:    "other",
				Description: "Draft from pbom init. Set the category and, for update checks, upstream.github.",
				Discovery:   api.Discovery{HelmCharts: []string{c.Name}},
			},
		}
		path := filepath.Join(dir, c.Name+".yaml")
		if _, err := os.Stat(path); err == nil && !force {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := writeYAML(path, "", comp); err != nil {
			return nil, err
		}
		names = append(names, c.Name)
	}
	return names, nil
}

func writeYAML(path, header string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(header), data...), 0o644)
}

func titleCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
