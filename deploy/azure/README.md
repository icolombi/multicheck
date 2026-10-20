# Multicheck on Azure Container Apps

Low-cost public deployment of Multicheck at **https://multicheck.icolombi.net**,
without a Kubernetes cluster. Independent of `k8s-homelab-azure`: own resource
group, own DNS zone, own Terraform state key.

## What it creates

| Resource | Name | Cost |
|---|---|---|
| Resource group | `rg-multicheck-prod-italynorth` | — |
| Container Apps environment (Consumption) | `cae-multicheck-prod-italynorth` | none when idle |
| Container App, 3 containers in one replica | `ca-multicheck-prod` | per second of activity, mostly inside the monthly free grant |
| Public DNS zone | `multicheck.icolombi.net` | ~€0.50/month |
| Managed TLS certificate | `cert-multicheck-icolombi-net` | free, auto-renewed |

```
Internet ──HTTPS──▶ Container Apps ingress ──▶ :8081 frontend (nginx, SPA)
                                                   │ /api/* (prefix stripped)
                                                   ▼
                                              :8080 backend (Go) ──▶ :6379 valkey
```

The three containers share the replica's network namespace, as with
`network_mode: host` in `docker-compose.yml`, so the published GHCR images run
unchanged: the backend's baked-in `config.toml` already targets Valkey at
`127.0.0.1:6379` and uses the system resolver (`nameServers = ""`).

Design choices:

- **Scale to zero** (`min_replicas = 0`, `max_replicas = 1`). The first request
  after an idle period has a cold start of a few seconds and an empty cache.
- **No logs**: no Log Analytics workspace. Logs exist only as a live stream:
  `az containerapp logs show -g rg-multicheck-prod-italynorth -n ca-multicheck-prod --container backend --follow`.
- **Separate DNS zone**, delegated from GoDaddy, instead of records in
  `dev.icolombi.net` or `lab.icolombi.net`: the AKS lab's external-dns and
  cert-manager cannot touch it, and the GoDaddy API is not usable from Terraform
  for small accounts.

## Prerequisites

- Terraform >= 1.15.4, Azure CLI logged in to the target subscription.
- The GHCR packages `multicheck-backend` and `multicheck-frontend` are public.

## First deployment (two applies)

The custom domain and its certificate can only be validated once the DNS
delegation resolves, so they are behind `enable_custom_domain`.

```bash
cd deploy/azure
cp backend.hcl.example backend.hcl   # fill in storage_account_name
terraform init -backend-config=backend.hcl

# 1) Everything except the custom domain
terraform apply
terraform output dns_zone_name_servers
```

2) At GoDaddy, in the `icolombi.net` DNS zone, add **four NS records** with host
   `multicheck`, one for each name server in the output (remove any trailing dot
   if GoDaddy complains). Then wait until delegation is live:

```bash
dig +short NS multicheck.icolombi.net   # must return the Azure name servers
dig +short multicheck.icolombi.net      # must return environment_static_ip
```

```bash
# 3) Custom domain + managed certificate + binding
cp terraform.tfvars.example terraform.tfvars   # set enable_custom_domain = true
terraform apply
curl -sSI https://multicheck.icolombi.net | head -n1
```

Certificate issuance usually takes a few minutes; the apply waits for it.

## Day-2

- **New release**: change `image_tag` (variable or tfvars) and `terraform apply`.
  A new revision replaces the old one; the domain binding is kept.
- **Teardown**: `terraform destroy`, then remove the `multicheck` NS records at
  GoDaddy. Nothing in `k8s-homelab-azure` is affected.

## Possible improvements

- **Dedicated resolver (unbound sidecar).** Lookups go through the Azure
  resolver, and DNSBL providers (Spamhaus in particular) refuse queries from
  shared cloud resolvers. Multicheck reports those refusals as errors rather
  than false positives, but the affected lists then give no answer. The fix is
  a fourth container running unbound in recursive mode, with
  `nameServers = "127.0.0.1"` in a `config.toml` mounted from a secret volume
  (`GSS_CONFIG_PATH`). Not done yet: deferred until refusals actually show up.
