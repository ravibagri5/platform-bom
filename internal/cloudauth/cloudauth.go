// Package cloudauth authenticates to AKS, EKS and GKE clusters with the pod's
// cloud workload identity instead of the kubeconfig's exec plugin, so the
// image needs no kubelogin, aws or gke-gcloud-auth-plugin binary.
package cloudauth

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// tokenFunc returns a bearer token for the Kubernetes API and when it expires.
type tokenFunc func(ctx context.Context) (string, time.Time, error)

// Apply replaces a recognised cloud exec plugin in cfg with tokens from the
// matching workload identity and returns the provider used. It returns "" and
// leaves cfg unchanged when the plugin is not recognised or the pod has no
// workload identity for that cloud.
func Apply(cfg *rest.Config) (string, error) {
	if cfg.ExecProvider == nil {
		return "", nil
	}
	provider, get, err := detect(cfg.ExecProvider)
	if err != nil {
		return "", fmt.Errorf("%s workload identity: %w", provider, err)
	}
	if get == nil {
		return "", nil
	}
	cfg.ExecProvider = nil
	cfg.BearerToken, cfg.BearerTokenFile = "", ""
	cfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return &bearerTransport{base: rt, get: get}
	})
	return provider, nil
}

func detect(exec *clientcmdapi.ExecConfig) (string, tokenFunc, error) {
	args := exec.Args
	switch filepath.Base(exec.Command) {
	case "kubelogin":
		if slices.Contains(args, "get-token") && azureAvailable() {
			get, err := azureToken(flagValue(args, "--server-id"))
			return "azure", get, err
		}
	case "aws":
		if slices.Contains(args, "eks") && slices.Contains(args, "get-token") && awsAvailable() {
			get, err := awsToken(flagValue(args, "--cluster-name", "--cluster-id"), awsRegion(exec), flagValue(args, "--role-arn"))
			return "aws", get, err
		}
	case "aws-iam-authenticator":
		if slices.Contains(args, "token") && awsAvailable() {
			get, err := awsToken(flagValue(args, "-i", "--cluster-id"), awsRegion(exec), flagValue(args, "-r", "--role"))
			return "aws", get, err
		}
	case "gke-gcloud-auth-plugin":
		if gcpAvailable() {
			get, err := gcpToken()
			return "gcp", get, err
		}
	}
	return "", nil, nil
}

// flagValue returns the value of the first of names given as "--flag value"
// or "--flag=value".
func flagValue(args []string, names ...string) string {
	for i, a := range args {
		for _, n := range names {
			if a == n && i+1 < len(args) {
				return args[i+1]
			}
			if v, ok := strings.CutPrefix(a, n+"="); ok {
				return v
			}
		}
	}
	return ""
}

func execEnv(exec *clientcmdapi.ExecConfig, name string) string {
	for _, e := range exec.Env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

// bearerTransport adds a cached token to requests, refreshing it shortly
// before it expires.
type bearerTransport struct {
	base http.RoundTripper
	get  tokenFunc

	mu     sync.Mutex
	token  string
	expiry time.Time
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Authorization") != "" {
		return t.base.RoundTrip(req)
	}
	token, err := t.current(req.Context())
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(r)
}

func (t *bearerTransport) current(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && time.Until(t.expiry) > time.Minute {
		return t.token, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	token, expiry, err := t.get(ctx)
	if err != nil {
		return "", fmt.Errorf("get cluster token: %w", err)
	}
	t.token, t.expiry = token, expiry
	return token, nil
}

func (t *bearerTransport) WrappedRoundTripper() http.RoundTripper { return t.base }
