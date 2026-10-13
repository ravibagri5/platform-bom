package cloudauth

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func clearCloudEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"AZURE_FEDERATED_TOKEN_FILE", "AZURE_CLIENT_ID", "AZURE_TENANT_ID",
		"AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_ROLE_ARN", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
		"AWS_REGION", "AWS_DEFAULT_REGION", "AWS_PROFILE",
		"GOOGLE_APPLICATION_CREDENTIALS",
	} {
		t.Setenv(k, "")
	}
}

func execConfig(command string, args ...string) *rest.Config {
	return &rest.Config{Host: "https://example.invalid", ExecProvider: &clientcmdapi.ExecConfig{Command: command, Args: args}}
}

func TestApplyLeavesConfigWithoutWorkloadIdentity(t *testing.T) {
	clearCloudEnv(t)
	for _, cfg := range []*rest.Config{
		{Host: "https://example.invalid", BearerToken: "static"},
		execConfig("kubelogin", "get-token", "--login", "devicecode"),
		execConfig("aws", "eks", "get-token", "--cluster-name", "prod"),
		execConfig("/usr/local/bin/custom-plugin", "token"),
	} {
		exec := cfg.ExecProvider
		provider, err := Apply(cfg)
		if err != nil || provider != "" || cfg.ExecProvider != exec || cfg.WrapTransport != nil {
			t.Errorf("Apply(%v) = %q, %v; want config unchanged", exec, provider, err)
		}
	}
}

func TestApplyAzure(t *testing.T) {
	clearCloudEnv(t)
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", filepath.Join(t.TempDir(), "token"))
	t.Setenv("AZURE_CLIENT_ID", "00000000-0000-0000-0000-000000000001")
	t.Setenv("AZURE_TENANT_ID", "00000000-0000-0000-0000-000000000002")
	cfg := execConfig("/usr/local/bin/kubelogin", "get-token", "--login", "devicecode", "--server-id", aksServerID)
	cfg.BearerToken = "stale"
	provider, err := Apply(cfg)
	if err != nil || provider != "azure" {
		t.Fatalf("Apply = %q, %v", provider, err)
	}
	if cfg.ExecProvider != nil || cfg.BearerToken != "" || cfg.WrapTransport == nil {
		t.Errorf("exec plugin not replaced: %+v", cfg)
	}
}

func TestApplyAWSRequiresCluster(t *testing.T) {
	clearCloudEnv(t)
	t.Setenv("AWS_CONTAINER_CREDENTIALS_FULL_URI", "http://169.254.170.23/v1/credentials")
	_, err := Apply(execConfig("aws", "eks", "get-token", "--region", "eu-west-1"))
	if err == nil || !strings.Contains(err.Error(), "aws workload identity") {
		t.Fatalf("Apply without --cluster-name: %v", err)
	}
}

func TestApplyGCP(t *testing.T) {
	clearCloudEnv(t)
	creds := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(creds, []byte(`{"type":"authorized_user","client_id":"id","client_secret":"secret","refresh_token":"refresh"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", creds)
	cfg := execConfig("gke-gcloud-auth-plugin")
	if provider, err := Apply(cfg); err != nil || provider != "gcp" || cfg.ExecProvider != nil {
		t.Fatalf("Apply = %q, %v; exec %v", provider, err, cfg.ExecProvider)
	}
}

func TestFlagValue(t *testing.T) {
	args := []string{"--region", "eu-west-1", "eks", "get-token", "--cluster-name=prod", "-i", "dev"}
	for name, want := range map[string]string{"--region": "eu-west-1", "--cluster-name": "prod", "-i": "dev", "--role-arn": ""} {
		if got := flagValue(args, name); got != want {
			t.Errorf("flagValue(%s) = %q, want %q", name, got, want)
		}
	}
	if got := flagValue([]string{"--server-id"}, "--server-id"); got != "" {
		t.Errorf("flag without value = %q", got)
	}
}

func TestAWSRegion(t *testing.T) {
	exec := &clientcmdapi.ExecConfig{Args: []string{"eks", "get-token"}, Env: []clientcmdapi.ExecEnvVar{{Name: "AWS_REGION", Value: "us-east-2"}}}
	if got := awsRegion(exec); got != "us-east-2" {
		t.Errorf("region from exec env = %q", got)
	}
	exec.Args = append(exec.Args, "--region", "eu-north-1")
	if got := awsRegion(exec); got != "eu-north-1" {
		t.Errorf("region from args = %q", got)
	}
}

func TestAWSTokenFormat(t *testing.T) {
	clearCloudEnv(t)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "none"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "none"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	get, err := awsToken("prod", "eu-west-1", "")
	if err != nil {
		t.Fatal(err)
	}
	token, expiry, err := get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(expiry) < 13*time.Minute {
		t.Errorf("expiry %v too soon", expiry)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, eksTokenPrefix))
	if !strings.HasPrefix(token, eksTokenPrefix) || err != nil {
		t.Fatalf("token %q: %v", token, err)
	}
	u, err := url.Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "sts.eu-west-1.amazonaws.com" || q.Get("Action") != "GetCallerIdentity" ||
		q.Get("X-Amz-Expires") != "60" || !strings.Contains(q.Get("X-Amz-SignedHeaders"), "x-k8s-aws-id") {
		t.Errorf("presigned URL %s", u)
	}
}

type recorder struct{ auth []string }

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.auth = append(r.auth, req.Header.Get("Authorization"))
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestBearerTransportCachesAndRefreshes(t *testing.T) {
	calls := 0
	expiry := time.Now().Add(time.Hour)
	base := &recorder{}
	tr := &bearerTransport{base: base, get: func(context.Context) (string, time.Time, error) {
		calls++
		return "t" + string(rune('0'+calls)), expiry, nil
	}}
	do := func(header string) {
		req, _ := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	do("")
	do("")
	expiry = time.Now().Add(30 * time.Second)
	tr.expiry = expiry
	do("")
	do("Bearer existing")
	want := []string{"Bearer t1", "Bearer t1", "Bearer t2", "Bearer existing"}
	if strings.Join(base.auth, ",") != strings.Join(want, ",") || calls != 2 {
		t.Errorf("auth headers %v after %d token calls, want %v", base.auth, calls, want)
	}
}

type closeTracker struct {
	io.Reader
	closed bool
}

func (c *closeTracker) Close() error { c.closed = true; return nil }

func TestBearerTransportClosesBodyOnTokenError(t *testing.T) {
	tr := &bearerTransport{base: &recorder{}, get: func(context.Context) (string, time.Time, error) {
		return "", time.Time{}, errors.New("denied")
	}}
	body := &closeTracker{Reader: strings.NewReader("{}")}
	req, _ := http.NewRequest(http.MethodPost, "https://example.invalid", body)
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || !body.closed {
		t.Errorf("err %v, body closed %v", err, body.closed)
	}
}
