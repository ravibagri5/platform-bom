#!/bin/sh
# End-to-end check of the local chart and image on a throwaway kind cluster:
# CRDs, live reconciliation, Inventory resources, drafts and CNCF classification.
# Usage: sh hack/kind-e2e.sh        (KEEP=1 keeps the cluster; UPGRADE_FROM=0.9.0 also tests upgrading from that chart)
set -eu

cluster=pbom-e2e
ns=platform-bom
image=platform-bom:e2e
port=18080
root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
failed=0

cleanup() {
  [ -n "${pf:-}" ] && kill "$pf" 2>/dev/null || true
  rm -rf "$tmp"
  if [ "${KEEP:-}" = 1 ]; then
    echo "Cluster kept: kubectl --context kind-$cluster -n $ns get pbom"
  else
    kind delete cluster --name "$cluster" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

check() {
  desc=$1
  shift
  if "$@" >/dev/null 2>&1; then echo "ok    $desc"; else echo "FAIL  $desc"; failed=1; fi
}

# Retries for up to 60s: reconciliation and discovery are asynchronous.
eventually() {
  desc=$1
  shift
  i=0
  until "$@" >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -ge 30 ]; then echo "FAIL  $desc"; failed=1; return 0; fi
    sleep 2
  done
  echo "ok    $desc"
}

api() { curl -fsS "http://127.0.0.1:$port$1"; }
api_has() { api "$1" | grep -q -- "$2"; }
api_lacks() { ! api "$1" | grep -q -- "$2"; }
logs_have() { kubectl -n "$ns" logs deploy/pbom-platform-bom | grep -q -- "$1"; }

echo "==> Building $image"
docker build -q -t "$image" --build-arg VERSION=e2e "$root" >/dev/null

echo "==> Creating kind cluster $cluster"
kind get clusters 2>/dev/null | grep -qx "$cluster" || kind create cluster --name "$cluster" --wait 120s
kubectl config use-context "kind-$cluster" >/dev/null
kind load docker-image "$image" --name "$cluster"

echo "==> Installing a Helm release named after a CNCF project that is not in the catalog (kured)"
mkdir -p "$tmp/kured/templates"
printf 'apiVersion: v2\nname: kured\nversion: 5.6.0\nappVersion: 1.17.0\n' > "$tmp/kured/Chart.yaml"
printf 'apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: kured-e2e\n' > "$tmp/kured/templates/cm.yaml"
helm upgrade --install kured "$tmp/kured" -n kube-system --wait >/dev/null

echo "==> Installing PBOM from the local chart"
helm upgrade --install pbom "$root/charts/platform-bom" -n "$ns" --create-namespace \
  --set image.repository=platform-bom --set image.tag=e2e --set image.pullPolicy=Never \
  --wait --timeout 3m >/dev/null
kubectl -n "$ns" port-forward svc/pbom-platform-bom "$port:80" >/dev/null 2>&1 &
pf=$!

echo "==> Checks"
check "four pbom.dev CRDs installed" sh -c "[ \$(kubectl get crd -o name | grep -c 'pbom.dev') -eq 4 ]"
check "Platform resource created by the chart" kubectl -n "$ns" get platform my-platform
eventually "readyz" api_has /readyz ok
eventually "kured classified from the CNCF landscape" api_has /api/inventory '"name":"kured"[^}]*"known":true'
eventually "description and offerings drafted" api_has /api/platform '"drafted":\["offerings","description"\]'

kubectl -n "$ns" patch platform my-platform --type merge -p '{"spec":{"description":"Edited by e2e"}}' >/dev/null
eventually "edited description served without restart" api_has /api/platform '"description":"Edited by e2e"'
check "edited description no longer drafted" api_has /api/platform '"drafted":\["offerings"\]'

sed 's/^  name: prod$/  name: edge-export/' "$root/examples/acme/inventories/prod.yaml" | kubectl -n "$ns" apply -f - >/dev/null
kubectl -n "$ns" patch platform my-platform --type json \
  -p '[{"op":"add","path":"/spec/environments/-","value":{"name":"edge","inventory":"edge-export"}}]' >/dev/null
eventually "Inventory resource used for the edge environment" api_has /api/inventory '"argocd"'

kubectl -n "$ns" patch platform my-platform --type json \
  -p '[{"op":"add","path":"/spec/environments/-","value":{"name":"bad","server":"http://insecure"}}]' >/dev/null
eventually "invalid change reported in the log" logs_have "keeping the previous configuration"
check "previous configuration still served" api_lacks /api/platform '"name":"bad"'
check "pod did not restart" sh -c "[ \$(kubectl -n $ns get pods -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}') -eq 0 ]"
# The chart owns my-platform; Helm refuses to take back fields kubectl edited, so start fresh.
kubectl -n "$ns" delete platform my-platform >/dev/null

echo "==> Upgrading with an unreachable landscape URL"
kill "$pf" 2>/dev/null || true
helm upgrade pbom "$root/charts/platform-bom" -n "$ns" --reuse-values \
  --set landscapeURL=http://127.0.0.1:1/landscape.yml --wait --timeout 3m >/dev/null
kubectl -n "$ns" rollout status deploy/pbom-platform-bom --timeout 120s >/dev/null
kubectl -n "$ns" port-forward svc/pbom-platform-bom "$port:80" >/dev/null 2>&1 &
pf=$!
eventually "landscape download failure logged" logs_have "using the built-in snapshot"
eventually "kured still classified from the built-in snapshot" api_has /api/inventory '"name":"kured"[^}]*"known":true'

echo "==> Switching to files from pbom init, in the order getting-started.md documents"
(cd "$root" && go build -o "$tmp/pbom" ./cmd/pbom)
"$tmp/pbom" init "$tmp/pbom-config" --context "kind-$cluster" --env prod --landscape-url "" >/dev/null
sed -i.bak 's/^  tagline: .*/  tagline: "From pbom init"/' "$tmp/pbom-config/platform.yaml"
kill "$pf" 2>/dev/null || true
helm upgrade pbom "$root/charts/platform-bom" -n "$ns" --reset-then-reuse-values \
  --set crdResources.create=false --set platform.metadata.name=my-platform >/dev/null
kubectl apply -k "$tmp/pbom-config" >/dev/null
kubectl -n "$ns" rollout status deploy/pbom-platform-bom --timeout 120s >/dev/null
kubectl -n "$ns" port-forward svc/pbom-platform-bom "$port:80" >/dev/null 2>&1 &
pf=$!
eventually "platform from pbom init served" api_has /api/platform 'From pbom init'
check "release from pbom init served" api_has /api/releases '"version":"1.0.0"'

if [ -n "${UPGRADE_FROM:-}" ]; then
  echo "==> Upgrading from the published chart $UPGRADE_FROM"
  kill "$pf" 2>/dev/null || true
  helm uninstall pbom -n "$ns" --wait >/dev/null
  kubectl delete crd platforms.pbom.dev platformreleases.pbom.dev components.pbom.dev inventories.pbom.dev >/dev/null
  helm install pbom oci://ghcr.io/ravibagri5/charts/platform-bom --version "$UPGRADE_FROM" -n "$ns" \
    --set-json 'platform={"apiVersion":"pbom.dev/v1alpha1","kind":"Platform","metadata":{"name":"old","displayName":"Old values"},"spec":{"environments":[{"name":"prod","inCluster":true}]}}' \
    --wait --timeout 3m >/dev/null
  helm show crds "$root/charts/platform-bom" | kubectl apply --server-side -f - >/dev/null
  helm upgrade pbom "$root/charts/platform-bom" -n "$ns" --reset-then-reuse-values \
    --set image.repository=platform-bom --set image.tag=e2e --set image.pullPolicy=Never --wait --timeout 3m >/dev/null
  kubectl -n "$ns" port-forward svc/pbom-platform-bom "$port:80" >/dev/null 2>&1 &
  pf=$!
  eventually "0.x chart values served after upgrade" api_has /api/platform '"displayName":"Old values"'
fi

if [ "$failed" -ne 0 ]; then
  echo "Some checks failed. Logs:"
  kubectl -n "$ns" logs deploy/pbom-platform-bom | tail -30
  exit 1
fi
echo "All checks passed."
