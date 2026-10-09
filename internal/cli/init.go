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
	"github.com/ravibagri5/platform-bom/internal/release"
)

var (
	componentNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]{0,61}[a-z0-9])?$`)
	envNameRE       = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

func newInitCmd() *cobra.Command {
	var kubeContext, kubeconfig, env, name, version, namespace string
	var drafts, force bool
	cmd := &cobra.Command{
		Use:   "init DIR",
		Short: "Create a platform directory from what a cluster runs",
		Long: `Discover a cluster and create a platform directory for the Helm chart:

  DIR/platform.yaml             Platform: name, offerings, environments
  DIR/releases/VERSION.yaml     PlatformRelease: what the cluster runs today
  DIR/components/*.yaml         Component drafts for unclassified Helm releases (--components)
  DIR/kustomization.yaml        turns the files into ConfigMaps for kubectl apply -k

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

			p := api.Platform{
				TypeMeta: api.TypeMeta{APIVersion: api.APIVersion, Kind: api.KindPlatform},
				Metadata: api.Metadata{Name: name, DisplayName: titleCase(name)},
				Spec: api.PlatformSpec{
					Offerings: []api.Offering{{
						Name: "kubernetes-workloads", DisplayName: "Kubernetes workloads",
						Category: "runtime", Status: "ga", Components: []string{"kubernetes"},
					}},
				},
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := writePlatform(filepath.Join(dir, "platform.yaml"), name, env, version); err != nil {
				return err
			}

			summary := "Baseline of " + env
			if kubeContext != "" {
				summary += " (" + kubeContext + ")"
			}
			rel := release.FromInventory(version, inv, p.Spec.Offerings, summary)
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
	return cmd
}

// platformTemplate keeps a readable field order, with environments last so
// new environments can be appended. Values are JSON-quoted, which is valid YAML.
const platformTemplate = `# The platform pbom presents. Edit offerings to describe what teams get; each
# lists component names from the Components page or "pbom catalog".
# Releases are in releases/, extra Component definitions in components/.
# After adding or removing files, run "pbom kustomize" on this directory.
apiVersion: pbom.dev/v1alpha1
kind: Platform
metadata:
  name: %[1]s
  displayName: %[2]s
spec:
  tagline: What our platform provides and where it runs.
  componentsDir: components
  offerings:
    - name: kubernetes-workloads
      displayName: Kubernetes workloads
      category: runtime
      status: ga
      components: [kubernetes]
  environments:
    - name: %[3]s
      displayName: %[4]s
      inCluster: true
      targetRelease: %[5]s
`

func writePlatform(path, name, env, version string) error {
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	data := fmt.Sprintf(platformTemplate, q(name), q(titleCase(name)), q(env), q(titleCase(env)), q(version))
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
