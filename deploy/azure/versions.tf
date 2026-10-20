terraform {
  required_version = ">= 1.15.4"

  # Pessimistic constraints: a major upgrade never slips in through
  # `terraform init -upgrade` without a deliberate edit here.
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 5.10"
    }
    # Only used to bind the managed certificate to the custom domain: azurerm
    # can create both, but has no way to attach one to the other (see main.tf).
    azapi = {
      source  = "azure/azapi"
      version = "~> 2.13"
    }
  }

  # Same storage account as k8s-homelab-azure, separate state key: the two
  # projects never share state, so a destroy on one cannot touch the other.
  backend "azurerm" {}
}

provider "azurerm" {
  features {}

  # Microsoft.App is not registered on the subscription by default. Registering
  # it here (instead of with an azurerm_resource_provider_registration resource)
  # keeps it out of the state, so a `terraform destroy` never unregisters it.
  resource_providers_to_register = ["Microsoft.App"]
}

provider "azapi" {}
