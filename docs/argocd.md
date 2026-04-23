# Using with ArgoCD

The f5xc-k8s-operator works with ArgoCD and standard GitOps workflows. The operator only writes to `.status`, never mutates `.spec`, so there is no conflict with ArgoCD's drift detection.

## Health Checks

ArgoCD does not know how to assess health for custom resources unless you configure custom health checks. Without them, ArgoCD reports all operator-managed resources as "Healthy" immediately, even if they failed to sync to F5 XC.

Add the health check configuration to your ArgoCD ConfigMap or `argocd-cm`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  resource.customizations.health.xc.f5.com_HTTPLoadBalancer: |
    hs = {}
    if obj.status == nil or obj.status.conditions == nil then
      hs.status = "Progressing"
      hs.message = "Waiting for reconciliation"
      return hs
    end
    for _, c in ipairs(obj.status.conditions) do
      if c.type == "Ready" and c.status == "True" then
        hs.status = "Healthy"
        hs.message = c.reason or ""
        return hs
      end
      if c.type == "Synced" and c.status == "False" then
        hs.status = "Degraded"
        hs.message = c.message or c.reason or "Sync failed"
        return hs
      end
    end
    hs.status = "Progressing"
    hs.message = "Waiting for Ready condition"
    return hs
```

This same health check logic applies to all 11 CRDs. A ready-to-use configuration file with all resource types is provided at [`docs/argocd-health-checks.yaml`](argocd-health-checks.yaml). You can merge it directly into your `argocd-cm` ConfigMap.

## Secret Management

The Helm chart accepts `credentials.apiToken` as a value. In a GitOps workflow, avoid storing this in plaintext in your Application spec. Common approaches:

- **External Secrets Operator** — sync the API token from a secret store (Vault, AWS Secrets Manager, etc.) into the `xc-credentials` K8s Secret
- **Sealed Secrets** — encrypt the Secret and commit the SealedSecret to Git
- **SOPS** — encrypt values files with SOPS and decrypt during ArgoCD sync

The same applies to TLS Secrets referenced by Certificate CRs — these need to exist in the cluster before (or alongside) the Certificate CR.

## Example Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: f5xc-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/your-infra-repo
    path: clusters/production/f5xc
    targetRevision: main
  destination:
    server: https://kubernetes.default.svc
    namespace: f5xc-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

> **Note on pruning:** When ArgoCD prunes a resource (removes it because it was deleted from Git), the operator's default behavior is to also delete the corresponding F5 XC resource. To prevent this, annotate resources with `f5xc.io/deletion-policy: orphan` before removing them from Git.
