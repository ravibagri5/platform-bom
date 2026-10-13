package cloudauth

import (
	"context"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// aksServerID is the Entra ID application of the AKS API server, the
// default --server-id of kubelogin.
const aksServerID = "6dae42f8-4368-4678-94ff-3960e28e3630"

// azureAvailable reports whether the Azure Workload Identity webhook has
// configured the pod.
func azureAvailable() bool {
	return os.Getenv("AZURE_FEDERATED_TOKEN_FILE") != "" &&
		os.Getenv("AZURE_CLIENT_ID") != "" &&
		os.Getenv("AZURE_TENANT_ID") != ""
}

func azureToken(serverID string) (tokenFunc, error) {
	if serverID == "" {
		serverID = aksServerID
	}
	cred, err := azidentity.NewWorkloadIdentityCredential(nil)
	if err != nil {
		return nil, err
	}
	scope := serverID + "/.default"
	return func(ctx context.Context) (string, time.Time, error) {
		tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
		return tok.Token, tok.ExpiresOn, err
	}, nil
}
