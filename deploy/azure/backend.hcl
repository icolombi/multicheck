# Shared Terraform state storage (created by k8s-homelab-azure/infra/bootstrap/state).
# Separate key: this state is independent of every k8s-homelab-azure state.
resource_group_name  = "rg-terraform-state-shared"
storage_account_name = "tfstate8c14006f25f54d2bb"
container_name       = "tfstate"
key                  = "multicheck-azure-prod.tfstate"
use_azuread_auth     = true
