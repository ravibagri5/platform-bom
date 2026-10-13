package cloudauth

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const eksTokenPrefix = "k8s-aws-v1."

// awsAvailable reports whether the pod has IAM Roles for Service Accounts or
// EKS Pod Identity.
func awsAvailable() bool {
	return os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != "" && os.Getenv("AWS_ROLE_ARN") != "" ||
		os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != ""
}

func awsRegion(exec *clientcmdapi.ExecConfig) string {
	if r := flagValue(exec.Args, "--region"); r != "" {
		return r
	}
	if r := execEnv(exec, "AWS_REGION"); r != "" {
		return r
	}
	return execEnv(exec, "AWS_DEFAULT_REGION")
}

// awsToken returns tokens in the format of aws eks get-token: a presigned
// STS GetCallerIdentity URL bound to the cluster name.
func awsToken(cluster, region, roleARN string) (tokenFunc, error) {
	if cluster == "" {
		return nil, errors.New("the kubeconfig names no EKS cluster (--cluster-name)")
	}
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, err
	}
	if cfg.Region == "" {
		return nil, errors.New("no AWS region: add --region to the kubeconfig or set AWS_REGION")
	}
	if roleARN != "" {
		cfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), roleARN))
	}
	presigner := sts.NewPresignClient(sts.NewFromConfig(cfg))
	return func(ctx context.Context) (string, time.Time, error) {
		req, err := presigner.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(o *sts.PresignOptions) {
			o.ClientOptions = append(o.ClientOptions, func(o *sts.Options) {
				o.APIOptions = append(o.APIOptions,
					smithyhttp.SetHeaderValue("x-k8s-aws-id", cluster),
					smithyhttp.SetHeaderValue("X-Amz-Expires", "60"))
			})
		})
		if err != nil {
			return "", time.Time{}, err
		}
		// EKS accepts the URL for 15 minutes after signing.
		return eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(req.URL)), time.Now().Add(14 * time.Minute), nil
	}, nil
}
