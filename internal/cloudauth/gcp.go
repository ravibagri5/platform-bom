package cloudauth

import (
	"context"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/oauth2/google"
)

// gcpScopes match gke-gcloud-auth-plugin.
var gcpScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

// gcpAvailable reports whether Application Default Credentials are
// configured or the GKE metadata server is reachable.
func gcpAvailable() bool {
	return os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" || metadata.OnGCE()
}

func gcpToken() (tokenFunc, error) {
	ts, err := google.DefaultTokenSource(context.Background(), gcpScopes...)
	if err != nil {
		return nil, err
	}
	return func(context.Context) (string, time.Time, error) {
		tok, err := ts.Token()
		if err != nil {
			return "", time.Time{}, err
		}
		return tok.AccessToken, tok.Expiry, nil
	}, nil
}
