#!/bin/sh
# Create three minimal, independent kind clusters for local environments.
set -eu

for environment in dev qa prod; do
  cluster="pbom-$environment"
  if kind get clusters 2>/dev/null | grep -qx "$cluster"; then
    echo "Cluster $cluster already exists"
  else
    kind create cluster --name "$cluster"
  fi
done

echo "Contexts: kind-pbom-dev, kind-pbom-qa, kind-pbom-prod"