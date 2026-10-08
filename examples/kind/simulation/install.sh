#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
for environment in dev qa prod; do
  echo "Installing PBOM fixtures in kind-pbom-$environment"
  helm upgrade --install pbom-simulation "$root/chart" \
    --kube-context "kind-pbom-$environment" \
    --namespace pbom-simulation --create-namespace \
    --values "$root/$environment.yaml" --wait --timeout 2m
done