locals {
  # The backend and nginx share the replica's network namespace, so they must
  # listen on different ports. 8080 stays the backend's (it is baked into the
  # image's config.toml); nginx-unprivileged cannot bind below 1024.
  backend_port  = 8080
  frontend_port = 8081
}

resource "azurerm_resource_group" "this" {
  name     = var.resource_group_name
  location = var.location
  tags     = var.tags
}

# Consumption-only environment: no fixed cost, billed per second of active
# replica. No Log Analytics workspace and no logs_destination, so logs are only
# available as a live stream (`az containerapp logs show`) and cost nothing.
resource "azurerm_container_app_environment" "this" {
  name                = var.environment_name
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  tags                = var.tags
}

# One app, three containers in the same replica. They talk over 127.0.0.1,
# exactly like the docker-compose setup with network_mode: host, so the images
# run unmodified: the backend's baked-in config.toml already points Redis at
# 127.0.0.1:6379 and uses the system resolver.
resource "azurerm_container_app" "this" {
  name                         = var.container_app_name
  container_app_environment_id = azurerm_container_app_environment.this.id
  resource_group_name          = azurerm_resource_group.this.name
  revision_mode                = "Single"
  tags                         = var.tags

  template {
    # Scale to zero when idle: the first request after a quiet period pays a
    # cold start of a few seconds, and starts with an empty Valkey cache.
    min_replicas = 0
    max_replicas = var.max_replicas

    http_scale_rule {
      name                = "http"
      concurrent_requests = "50"
    }

    # Billing is per allocated (not used) vCPU/GiB-second, and in the Consumption
    # plan the containers must add up to a valid combination. The split below
    # totals 0.5 vCPU / 1Gi: actual usage is a few tens of MiB and near-zero CPU
    # (the AKS requests are 10m / 32Mi per container). The smallest combination,
    # 0.25 / 0.5Gi, is out of reach with three containers: azurerm requires at
    # least 0.1 vCPU each, and the API accepts at most two decimal places.
    container {
      name   = "frontend"
      image  = "ghcr.io/icolombi/multicheck-frontend:${var.image_tag}"
      cpu    = 0.1
      memory = "0.2Gi"

      # The nginx template proxies /api/* to the backend, stripping the prefix
      # (the role the Envoy HTTPRoute plays on AKS).
      env {
        name  = "NGINX_PORT"
        value = tostring(local.frontend_port)
      }
      env {
        name  = "API_UPSTREAM"
        value = "http://127.0.0.1:${local.backend_port}"
      }
    }

    container {
      name   = "backend"
      image  = "ghcr.io/icolombi/multicheck-backend:${var.image_tag}"
      cpu    = 0.3
      memory = "0.6Gi"

      liveness_probe {
        transport = "HTTP"
        port      = local.backend_port
        path      = "/health"
      }
    }

    # Pure cache: no persistence, bounded memory, LRU eviction. Same flags as
    # the Valkey deployment in k8s-homelab-azure. Started through sh because
    # `--save ""` needs an empty argument, and azurerm turns an empty string in
    # `args` into null and crashes ("interface {} is nil, not string").
    container {
      name    = "valkey"
      image   = var.valkey_image
      cpu     = 0.1
      memory  = "0.2Gi"
      command = ["/bin/sh", "-c"]
      args    = ["exec valkey-server --maxmemory 96mb --maxmemory-policy allkeys-lru --save '' --appendonly no"]
    }
  }

  ingress {
    external_enabled = true
    target_port      = local.frontend_port
    transport        = "auto"

    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }
}

# --- DNS --------------------------------------------------------------------
# A zone of its own, delegated with one NS record set at GoDaddy. It is
# deliberately separate from dev.icolombi.net / lab.icolombi.net: external-dns
# and cert-manager in the AKS lab only hold rights on their own zone, and this
# project holds none on theirs.
resource "azurerm_dns_zone" "this" {
  name                = var.dns_zone_name
  resource_group_name = azurerm_resource_group.this.name
  tags                = var.tags
}

# Apex record: a CNAME is not allowed at a zone apex, so point an A record at
# the environment's static IP. The IP only changes if the environment is
# recreated, and then this record follows it on the same apply.
resource "azurerm_dns_a_record" "apex" {
  name                = "@"
  zone_name           = azurerm_dns_zone.this.name
  resource_group_name = azurerm_resource_group.this.name
  ttl                 = 300
  records             = [azurerm_container_app_environment.this.static_ip_address]
  tags                = var.tags
}

# Domain ownership proof required by Container Apps before it accepts the
# hostname.
resource "azurerm_dns_txt_record" "asuid" {
  name                = "asuid"
  zone_name           = azurerm_dns_zone.this.name
  resource_group_name = azurerm_resource_group.this.name
  ttl                 = 300
  tags                = var.tags

  record {
    value = azurerm_container_app.this.custom_domain_verification_id
  }
}

# --- Custom domain + free managed certificate --------------------------------
# Azure needs three steps, in this order: (1) add the hostname to the app with
# binding disabled, (2) issue the managed certificate, which requires the
# hostname to be on an app already, (3) bind the certificate. azurerm covers
# (1) and (2) but cannot express (3), hence the azapi resource below.
resource "azurerm_container_app_custom_domain" "this" {
  count = var.enable_custom_domain ? 1 : 0

  name             = var.dns_zone_name
  container_app_id = azurerm_container_app.this.id

  depends_on = [azurerm_dns_a_record.apex, azurerm_dns_txt_record.asuid]

  lifecycle {
    # Step (3) changes these out of band; without this, every plan would
    # propose to unbind the certificate.
    ignore_changes = [certificate_binding_type, container_app_environment_certificate_id]
  }
}

# HTTP validation: the only method available for a zone apex (CNAME validation
# needs a CNAME, which an apex cannot have). It relies on the A record above.
resource "azurerm_container_app_environment_managed_certificate" "this" {
  count = var.enable_custom_domain ? 1 : 0

  name                         = "cert-${replace(var.dns_zone_name, ".", "-")}"
  container_app_environment_id = azurerm_container_app_environment.this.id
  subject_name                 = var.dns_zone_name
  domain_control_validation    = "HTTP"
  tags                         = var.tags

  depends_on = [azurerm_container_app_custom_domain.this]
}

# PATCH-style update: azapi merges this into the existing app, replacing only
# the customDomains array. A later image change on azurerm_container_app does not
# undo it: the provider sends ingress again only when the ingress block changes,
# and even then keeps the custom domains it read back.
resource "azapi_update_resource" "bind_certificate" {
  count = var.enable_custom_domain ? 1 : 0

  type        = "Microsoft.App/containerApps@2026-01-01"
  resource_id = azurerm_container_app.this.id

  body = {
    properties = {
      configuration = {
        ingress = {
          customDomains = [{
            name          = var.dns_zone_name
            certificateId = azurerm_container_app_environment_managed_certificate.this[0].id
            bindingType   = "SniEnabled"
          }]
        }
      }
    }
  }
}
