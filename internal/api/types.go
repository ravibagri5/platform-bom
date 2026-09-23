// Package api defines the platform-bom resource model.
//
// The primary object is the Platform, not a cluster. Clusters, Helm, Crossplane
// and container images are only evidence sources used to build an Inventory.
package api

import "time"

// APIVersion is the version of every platform-bom document.
const APIVersion = "pbom.dev/v1alpha1"

// Document kinds.
const (
	KindPlatform  = "Platform"
	KindRelease   = "PlatformRelease"
	KindComponent = "Component"
	KindInventory = "Inventory"
)

// TypeMeta identifies a document.
type TypeMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

// Metadata is common document metadata.
type Metadata struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// Platform describes an internal platform as a product.
type Platform struct {
	TypeMeta `json:",inline"`
	Metadata Metadata     `json:"metadata"`
	Spec     PlatformSpec `json:"spec"`
}

// PlatformSpec is the product definition of a platform.
type PlatformSpec struct {
	Tagline     string   `json:"tagline,omitempty"`
	Description string   `json:"description,omitempty"`
	Owners      []Owner  `json:"owners,omitempty"`
	Links       []Link   `json:"links,omitempty"`
	Guarantees  []string `json:"guarantees,omitempty"`
	// Readme is inline markdown shown on the platform page.
	Readme string `json:"readme,omitempty"`
	// ReadmeFile is a markdown file, relative to the platform file.
	ReadmeFile   string        `json:"readmeFile,omitempty"`
	Offerings    []Offering    `json:"offerings,omitempty"`
	Environments []Environment `json:"environments"`
	// ReleasesDir holds PlatformRelease documents. Defaults to "releases".
	ReleasesDir string `json:"releasesDir,omitempty"`
	// ComponentsDir holds extra or overriding Component definitions.
	ComponentsDir string          `json:"componentsDir,omitempty"`
	Discovery     DiscoveryPolicy `json:"discovery,omitempty"`
}

// DiscoveryPolicy tunes discovery for the whole platform.
type DiscoveryPolicy struct {
	// HideUnclassified skips Helm releases that match no catalog Component.
	HideUnclassified bool `json:"hideUnclassified,omitempty"`
	// IgnoreNamespaces are skipped when scanning workloads and Helm releases.
	IgnoreNamespaces []string `json:"ignoreNamespaces,omitempty"`
}

// Owner is a team or person responsible for the platform.
type Owner struct {
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Channel string `json:"channel,omitempty"`
}

// Link is a named URL.
type Link struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Offering is a capability the platform provides to its users.
type Offering struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	// Status is the maturity of the offering: planned, alpha, beta, ga or deprecated.
	Status     string   `json:"status,omitempty"`
	Docs       string   `json:"docs,omitempty"`
	Components []string `json:"components,omitempty"`
}

// Environment is where the platform runs.
type Environment struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Tier        string `json:"tier,omitempty"`
	// KubeContext and Kubeconfig select the cluster to discover.
	KubeContext string `json:"kubeContext,omitempty"`
	Kubeconfig  string `json:"kubeconfig,omitempty"`
	// InCluster discovers the cluster pbom itself runs in, using its service account.
	InCluster bool `json:"inCluster,omitempty"`
	// InventoryFile loads a previously exported Inventory instead of a live cluster.
	InventoryFile string `json:"inventoryFile,omitempty"`
	// TargetRelease is the PlatformRelease this environment should be running.
	TargetRelease string `json:"targetRelease,omitempty"`
}

// PlatformRelease is a versioned bundle of components and offerings.
type PlatformRelease struct {
	TypeMeta `json:",inline"`
	Metadata Metadata    `json:"metadata"`
	Spec     ReleaseSpec `json:"spec"`
}

// ReleaseSpec is the content of a platform release.
type ReleaseSpec struct {
	Version    string   `json:"version"`
	Date       string   `json:"date,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Highlights []string `json:"highlights,omitempty"`
	// Notes is markdown release notes.
	Notes string `json:"notes,omitempty"`
	// Components maps component name to version.
	Components map[string]string `json:"components"`
	Offerings  []string          `json:"offerings,omitempty"`
}

// Component is a catalog definition of a platform building block.
type Component struct {
	TypeMeta `json:",inline"`
	Metadata Metadata      `json:"metadata"`
	Spec     ComponentSpec `json:"spec"`
}

// ComponentSpec describes how to recognise and track a component.
type ComponentSpec struct {
	DisplayName string `json:"displayName,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
	// PartOf nests this component under another, e.g. a Crossplane provider under crossplane.
	PartOf    string    `json:"partOf,omitempty"`
	Discovery Discovery `json:"discovery,omitempty"`
	Upstream  *Upstream `json:"upstream,omitempty"`
}

// Discovery lists the signals that identify a component in a cluster.
type Discovery struct {
	// Kubernetes marks the component as the cluster itself.
	Kubernetes bool `json:"kubernetes,omitempty"`
	// Images are glob patterns matched against image repositories, ignoring registry and mirror prefixes.
	Images []string `json:"images,omitempty"`
	// HelmCharts are glob patterns matched against Helm chart names.
	HelmCharts []string `json:"helmCharts,omitempty"`
	// APIGroups indicate presence when served by the cluster.
	APIGroups []string `json:"apiGroups,omitempty"`
	// CrossplanePackages are glob patterns matched against Crossplane package repositories.
	CrossplanePackages []string `json:"crossplanePackages,omitempty"`
}

// Upstream is where new releases of a component are published.
type Upstream struct {
	// GitHub is an owner/repo slug.
	GitHub string `json:"github,omitempty"`
	// TagPrefix is required on tags and stripped before parsing, e.g. "controller-v".
	TagPrefix          string `json:"tagPrefix,omitempty"`
	IncludePrereleases bool   `json:"includePrereleases,omitempty"`
	// SupportedMinors is how many of the newest minor versions upstream supports.
	SupportedMinors int `json:"supportedMinors,omitempty"`
}

// Inventory is what was actually found in one environment.
type Inventory struct {
	TypeMeta    `json:",inline"`
	Metadata    Metadata              `json:"metadata"`
	Environment string                `json:"environment"`
	CollectedAt time.Time             `json:"collectedAt"`
	Cluster     ClusterInfo           `json:"cluster"`
	Components  []DiscoveredComponent `json:"components"`
	Errors      []string              `json:"errors,omitempty"`
}

// ClusterInfo describes the discovered cluster.
type ClusterInfo struct {
	Context      string `json:"context,omitempty"`
	Version      string `json:"version,omitempty"`
	GitVersion   string `json:"gitVersion,omitempty"`
	Distribution string `json:"distribution,omitempty"`
	Nodes        int    `json:"nodes,omitempty"`
}

// DiscoveredComponent is a component found in an environment.
type DiscoveredComponent struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Category    string `json:"category,omitempty"`
	Version     string `json:"version,omitempty"`
	Known       bool   `json:"known"`
	PartOf      string `json:"partOf,omitempty"`
	// Kind is what the component is: cluster, controller, helm, provider, function, configuration or api.
	Kind     string     `json:"kind,omitempty"`
	Evidence []Evidence `json:"evidence,omitempty"`
}

// Evidence sources.
const (
	SourceKubernetes = "kubernetes"
	SourceCrossplane = "crossplane-package"
	SourceImage      = "image"
	SourceHelm       = "helm"
	SourceAPIGroup   = "api-group"
)

// Evidence is a single observation supporting a discovered component.
type Evidence struct {
	Source    string `json:"source"`
	Namespace string `json:"namespace,omitempty"`
	Object    string `json:"object,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Version   string `json:"version,omitempty"`
}
