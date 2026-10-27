variable "location" {
  description = "Azure region for all resources."
  type        = string
  default     = "italynorth"
}

variable "resource_group_name" {
  description = "Dedicated resource group: nothing here is shared with k8s-homelab-azure."
  type        = string
  default     = "rg-multicheck-prod-italynorth"
}

variable "environment_name" {
  description = "Name of the Container Apps environment (Consumption plan)."
  type        = string
  default     = "cae-multicheck-prod-italynorth"
}

variable "container_app_name" {
  description = "Name of the Container App (lowercase, max 32 characters)."
  type        = string
  default     = "ca-multicheck-prod"
}

variable "dns_zone_name" {
  description = "Public DNS zone delegated from the GoDaddy registrar. The app is served at its apex."
  type        = string
  default     = "multicheck.icolombi.net"
}

variable "image_tag" {
  description = "Tag of ghcr.io/icolombi/multicheck-backend and multicheck-frontend to deploy (immutable semver, never latest)."
  type        = string
  default     = "1.6.10"

  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+$", var.image_tag))
    error_message = "image_tag must be a semver tag such as 1.6.10."
  }
}

variable "valkey_image" {
  description = "Valkey image used as an in-memory cache sidecar."
  type        = string
  default     = "docker.io/valkey/valkey:9.1.3-alpine"
}

variable "max_replicas" {
  description = "Upper bound on replicas. Kept at 1: it caps the bill, and Valkey is per replica anyway."
  type        = number
  default     = 1

  validation {
    condition     = var.max_replicas >= 1 && var.max_replicas <= 3
    error_message = "max_replicas must be between 1 and 3."
  }
}

variable "enable_custom_domain" {
  description = <<-EOT
    Bind the custom domain and its managed certificate. Leave false on the first
    apply: both validations fail until the NS delegation at GoDaddy is live.
  EOT
  type        = bool
  default     = false
}

variable "tags" {
  description = "Common tags (same mandatory keys as k8s-homelab-azure)."
  type        = map(string)
  default = {
    owner      = "topper"
    env        = "prod"
    purpose    = "multicheck"
    costCenter = "homelab"
  }
}
