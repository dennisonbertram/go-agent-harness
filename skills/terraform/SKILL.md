---
name: terraform
description: "Manage infrastructure with Terraform: init, plan, apply, destroy, state management, variables, workspaces, modules. Trigger: when using Terraform, infrastructure as code, terraform plan, terraform apply, Terraform state, Terraform modules, IaC"
version: 1
argument-hint: "[init|plan|apply|destroy|state|workspace]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Terraform / Infrastructure-as-Code

You are now operating in Terraform infrastructure management mode.

## Installation

```bash
# macOS (via Homebrew)
brew tap hashicorp/tap && brew install hashicorp/tap/terraform

# Verify installation
terraform version
```

## Core Workflow

```bash
# 1. Initialize — download providers and modules
terraform init

# 2. Format code (auto-fix indentation and style)
terraform fmt -recursive

# 3. Validate configuration
terraform validate

# 4. Preview changes
terraform plan

# 5. Apply changes (with confirmation prompt)
terraform apply

# 6. Apply without prompt (for CI/automation)
terraform apply -auto-approve

# 7. Destroy all resources
terraform destroy
```

## Variables

```bash
# Define variables in variables.tf
# variable "region" {
#   type    = string
#   default = "us-east-1"
# }

# Override variables on command line
terraform plan -var="region=us-west-2"

# Use a variable file (.tfvars)
terraform plan -var-file="prod.tfvars"

# Terraform auto-loads terraform.tfvars and *.auto.tfvars

# Use environment variables (TF_VAR_ prefix)
export TF_VAR_region="us-west-2"
terraform plan
```

## Terraform Files

```hcl
# main.tf — core resources
terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  backend "s3" {
    bucket = "my-terraform-state"
    key    = "prod/terraform.tfstate"
    region = "us-east-1"
  }
}

provider "aws" {
  region = var.region
}

resource "aws_s3_bucket" "app" {
  bucket = "my-app-assets-${var.environment}"
  tags = {
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

output "bucket_name" {
  value = aws_s3_bucket.app.bucket
}
```

## State Management

```bash
# List all resources in state
terraform state list

# Show details of a specific resource
terraform state show aws_s3_bucket.app

# Move a resource (rename in state)
terraform state mv aws_s3_bucket.app aws_s3_bucket.assets

# Remove a resource from state (does NOT destroy it)
terraform state rm aws_s3_bucket.old

# Import an existing resource into state
terraform import aws_s3_bucket.app my-existing-bucket-name

# Pull remote state to local (for inspection)
terraform state pull > current.tfstate
```

## Remote State Backend (S3)

```bash
# Create S3 bucket for state (one-time setup)
aws s3api create-bucket \
  --bucket my-terraform-state \
  --region us-east-1

# Enable versioning (for state history)
aws s3api put-bucket-versioning \
  --bucket my-terraform-state \
  --versioning-configuration Status=Enabled

# Enable encryption
aws s3api put-bucket-encryption \
  --bucket my-terraform-state \
  --server-side-encryption-configuration \
    '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
```

## Workspaces

```bash
# List workspaces
terraform workspace list

# Create and switch to a new workspace
terraform workspace new staging

# Switch between workspaces
terraform workspace select production
terraform workspace select staging

# Show current workspace
terraform workspace show

# Use workspace name in resources
# resource "aws_s3_bucket" "app" {
#   bucket = "myapp-${terraform.workspace}"
# }
```

## Modules

```bash
# Module usage in main.tf
# module "vpc" {
#   source  = "terraform-aws-modules/vpc/aws"
#   version = "~> 5.0"
#   name    = "my-vpc"
#   cidr    = "10.0.0.0/16"
# }

# Download/update module sources
terraform init -upgrade

# List all module calls in config
terraform providers
```

## Planning and Targeting

```bash
# Save plan to file (for apply in CI)
terraform plan -out=tfplan
terraform apply tfplan

# Plan with detailed diff
terraform plan -detailed-exitcode
# Exit 0 = no changes, Exit 1 = error, Exit 2 = changes present

# Apply only a specific resource
terraform apply -target=aws_s3_bucket.app

# Destroy only a specific resource
terraform destroy -target=aws_s3_bucket.old

# Refresh state from real infrastructure
terraform refresh
```

## Outputs

```bash
# Show all outputs after apply
terraform output

# Show a specific output value
terraform output bucket_name

# Output as JSON
terraform output -json
```

## Debugging

```bash
# Enable verbose logging
TF_LOG=DEBUG terraform plan 2>&1 | head -100

# Log to file
TF_LOG=INFO TF_LOG_PATH=./terraform.log terraform apply

# Show the dependency graph (requires graphviz)
terraform graph | dot -Tsvg > graph.svg
```

## CI/CD Pattern

```bash
# Typical CI pipeline steps
terraform init -input=false
terraform validate
terraform fmt -check -recursive        # fail if not formatted
terraform plan -out=tfplan -input=false
terraform apply -input=false tfplan
```

## Best Practices

- Store state in a remote backend (S3 + DynamoDB for locking) — never commit `.tfstate` files.
- Use workspaces or separate state files for production vs. staging.
- Pin provider versions with `~>` constraints in `required_providers`.
- Always run `terraform plan` and review output before `terraform apply`.
- Use `terraform apply -target` with caution — it can leave state inconsistent.
- Tag all resources with environment, owner, and `ManagedBy = "terraform"`.
- Use `terraform fmt -recursive` in pre-commit hooks to keep code consistent.
