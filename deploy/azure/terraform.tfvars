# Production values, versioned on purpose: this file is the record of what runs
# at multicheck.icolombi.net. Without it, an apply from a clean checkout would
# fall back to the defaults and unbind the custom domain.

# Custom domain + managed certificate. A fresh bootstrap starts with false and
# flips to true once the NS delegation at GoDaddy resolves (see README.md).
enable_custom_domain = true

# Release deployed in production: bump it and run `terraform apply`.
image_tag = "1.6.10"
