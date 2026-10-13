#!/bin/sh
# Give a central PBOM read-only access to one cluster and add it to a kubeconfig.
#
#   hack/remote-kubeconfig.sh ADMIN_CONTEXT NAME [OUTPUT]
#
# ADMIN_CONTEXT  your kubectl context with admin rights on the target cluster
# NAME           context name PBOM will use, e.g. prod (kubeContext: prod)
# OUTPUT         kubeconfig file to add NAME to (default: pbom-kubeconfig)
# SERVER         optional API server URL as reached from the PBOM pod, when it
#                differs from the one in ADMIN_CONTEXT
set -eu

admin=${1:?usage: remote-kubeconfig.sh ADMIN_CONTEXT NAME [OUTPUT]}
name=${2:?usage: remote-kubeconfig.sh ADMIN_CONTEXT NAME [OUTPUT]}
out=${3:-pbom-kubeconfig}

deploy="$(dirname -- "$0")/../deploy"
[ -f "$deploy/remote-reader.yaml" ] || deploy=https://raw.githubusercontent.com/ravibagri5/platform-bom/main/deploy
kubectl --context "$admin" apply -f "$deploy/remote-reader-role.yaml" -f "$deploy/remote-reader.yaml"

secret() {
  kubectl --context "$admin" -n platform-bom get secret pbom-reader-token -o "jsonpath={.data.$1}"
}

# The token controller fills the Secret shortly after it is created.
tries=0
until [ -n "$(secret token)" ]; do
  tries=$((tries + 1))
  [ "$tries" -lt 30 ] || { echo "token for pbom-reader was not issued" >&2; exit 1; }
  sleep 1
done

server=${SERVER:-$(kubectl config view --raw --minify --context "$admin" -o 'jsonpath={.clusters[0].cluster.server}')}
ca=$(mktemp)
trap 'rm -f "$ca"' EXIT
secret 'ca\.crt' | base64 -d > "$ca"

touch "$out"
chmod 600 "$out"
kubectl config --kubeconfig "$out" set-cluster "$name" --server "$server" --certificate-authority "$ca" --embed-certs >/dev/null
kubectl config --kubeconfig "$out" set-credentials "$name" --token "$(secret token | base64 -d)" >/dev/null
kubectl config --kubeconfig "$out" set-context "$name" --cluster "$name" --user "$name" >/dev/null
echo "Added context $name ($server) to $out"
