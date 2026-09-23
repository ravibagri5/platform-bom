#!/bin/sh
# Creates a kind cluster with a few platform components for trying out pbom.
set -e
CLUSTER=${CLUSTER:-pbom}
CTX=kind-$CLUSTER

kind get clusters 2>/dev/null | grep -qx "$CLUSTER" || kind create cluster --name "$CLUSTER"

helm repo add argo https://argoproj.github.io/argo-helm >/dev/null
helm repo add jetstack https://charts.jetstack.io >/dev/null
helm repo add crossplane-stable https://charts.crossplane.io/stable >/dev/null
helm repo update >/dev/null

helm upgrade --install argocd argo/argo-cd --kube-context "$CTX" -n argocd --create-namespace
helm upgrade --install cert-manager jetstack/cert-manager --kube-context "$CTX" -n cert-manager --create-namespace \
  --set crds.enabled=true
helm upgrade --install crossplane crossplane-stable/crossplane --kube-context "$CTX" -n crossplane-system \
  --create-namespace --wait

echo "Waiting for Crossplane package APIs..."
until kubectl --context "$CTX" get crd functions.pkg.crossplane.io providers.pkg.crossplane.io >/dev/null 2>&1; do sleep 2; done

kubectl --context "$CTX" apply -f - <<'EOF'
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-patch-and-transform
spec:
  package: xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-kubernetes
spec:
  package: xpkg.crossplane.io/crossplane-contrib/provider-kubernetes:v0.18.0
EOF

echo
echo "Cluster $CTX is ready. Try:"
echo "  ./bin/pbom discover --context $CTX"
echo "  ./bin/pbom serve -c examples/kind/pbom.yaml"
