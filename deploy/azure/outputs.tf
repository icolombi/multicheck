output "dns_zone_name_servers" {
  description = "Name servers to enter at GoDaddy as NS records for the 'multicheck' host of icolombi.net (one-time delegation)."
  value       = azurerm_dns_zone.this.name_servers
}

output "app_default_fqdn" {
  description = "Azure-provided hostname, reachable before the custom domain is bound."
  value       = azurerm_container_app.this.ingress[0].fqdn
}

output "environment_static_ip" {
  description = "Public IP of the Container Apps environment (target of the apex A record)."
  value       = azurerm_container_app_environment.this.static_ip_address
}

output "url" {
  description = "Public URL of the service."
  value       = var.enable_custom_domain ? "https://${var.dns_zone_name}" : "https://${azurerm_container_app.this.ingress[0].fqdn}"
}
