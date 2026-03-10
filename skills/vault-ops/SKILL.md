---
name: vault-ops
description: "Manage HashiCorp Vault secrets: read/write secrets, dynamic credentials, policy management, token renewal, authentication methods. Trigger: when using HashiCorp Vault, vault read, vault write, Vault secrets, dynamic credentials, Vault policy, Vault token, Vault auth"
version: 1
argument-hint: "[read|write|list|policy|token|lease] [path]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# HashiCorp Vault Operations

You are now operating in HashiCorp Vault secrets management mode.

## Installation and Setup

```bash
# macOS (via Homebrew)
brew tap hashicorp/tap
brew install hashicorp/tap/vault

# Verify installation
vault version

# Start a dev server (local development only — not for production)
vault server -dev

# Set environment variables for the dev server
export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN='root'  # dev mode root token

# Authenticate with a production Vault server
export VAULT_ADDR='https://vault.example.com:8200'
vault login -method=token  # prompts for token
```

## Authentication Methods

```bash
# Token authentication (most common)
vault login <token>
vault login -method=token token=<token>

# AppRole authentication (CI/CD)
vault write auth/approle/login \
  role_id="$ROLE_ID" \
  secret_id="$SECRET_ID"

# AWS IAM authentication
vault login -method=aws role=my-role

# Kubernetes authentication (in-cluster)
vault write auth/kubernetes/login \
  role=my-role \
  jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"

# Check current token info
vault token lookup
vault token lookup -accessor <accessor>

# Check authentication status
vault auth list
```

## Reading and Writing Secrets

```bash
# KV v2 secrets engine (default path: secret/)
# Write a secret
vault kv put secret/myapp/config \
  db_password="supersecret" \
  api_key="abc123"

# Read a secret
vault kv get secret/myapp/config

# Read a specific field
vault kv get -field=db_password secret/myapp/config

# List secrets at a path
vault kv list secret/myapp

# Delete a secret (creates a soft delete)
vault kv delete secret/myapp/config

# Destroy a specific version permanently
vault kv destroy -versions=1,2 secret/myapp/config

# Get secret metadata (versions, deletion status)
vault kv metadata get secret/myapp/config

# Read an older version
vault kv get -version=2 secret/myapp/config

# KV v1 secrets engine
vault read kv/myapp/config
vault write kv/myapp/config db_password="supersecret"
```

## Dynamic Credentials

Dynamic credentials are short-lived credentials generated on demand by Vault.

```bash
# Database dynamic credentials (PostgreSQL example)
# Read generates new temporary credentials
vault read database/creds/my-role

# Output:
# Key                Value
# ---                -----
# lease_id           database/creds/my-role/abc123
# lease_duration     1h
# lease_renewable    true
# password           A1a-xyz789
# username           v-root-my-role-abc123

# AWS dynamic credentials
vault read aws/creds/my-role

# Renew a lease before it expires
vault lease renew database/creds/my-role/abc123

# Revoke a lease immediately
vault lease revoke database/creds/my-role/abc123

# Revoke all leases under a prefix
vault lease revoke -prefix database/creds/my-role
```

## Policy Management

```bash
# List existing policies
vault policy list

# Read a policy
vault policy read default
vault policy read my-app-policy

# Write a policy from a file
cat > my-app-policy.hcl <<'EOF'
# Read secrets at specific path
path "secret/data/myapp/*" {
  capabilities = ["read", "list"]
}

# Allow reading dynamic database credentials
path "database/creds/my-role" {
  capabilities = ["read"]
}

# Allow token self-renewal
path "auth/token/renew-self" {
  capabilities = ["update"]
}

# Deny access to other paths
path "secret/data/other/*" {
  capabilities = ["deny"]
}
EOF

vault policy write my-app-policy my-app-policy.hcl

# Delete a policy
vault policy delete my-app-policy

# Format/validate a policy file
vault policy fmt my-app-policy.hcl
```

## Token Management

```bash
# Create a token with a specific policy
vault token create \
  -policy="my-app-policy" \
  -ttl="1h" \
  -renewable=true \
  -display-name="my-app"

# Create a token with multiple policies
vault token create \
  -policy="my-app-policy" \
  -policy="database-policy" \
  -ttl="24h"

# Renew your own token
vault token renew

# Renew a specific token
vault token renew <token>

# Renew with a new TTL increment
vault token renew -increment=2h <token>

# Revoke a token
vault token revoke <token>

# Revoke all child tokens recursively
vault token revoke -mode=tree <token>

# Look up token details
vault token lookup <token>

# Create an orphan token (no parent, won't be revoked when parent expires)
vault token create -orphan -policy="my-app-policy"
```

## Secrets Engine Management

```bash
# List enabled secrets engines
vault secrets list

# Enable KV v2 secrets engine at a custom path
vault secrets enable -path=apps kv-v2

# Enable database secrets engine
vault secrets enable database

# Configure database connection (PostgreSQL)
vault write database/config/my-postgres \
  plugin_name="postgresql-database-plugin" \
  allowed_roles="my-role" \
  connection_url="postgresql://{{username}}:{{password}}@localhost:5432/mydb" \
  username="vault-user" \
  password="vault-password"

# Create database role for dynamic credentials
vault write database/roles/my-role \
  db_name="my-postgres" \
  creation_statements="CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';" \
  default_ttl="1h" \
  max_ttl="24h"

# Disable a secrets engine
vault secrets disable apps/
```

## Environment Variable Injection

```bash
# Use vault agent for automatic secret injection
# vault-agent.hcl
cat > vault-agent.hcl <<'EOF'
auto_auth {
  method "approle" {
    mount_path = "auth/approle"
    config = {
      role_id_file_path = "/etc/vault/role_id"
      secret_id_file_path = "/etc/vault/secret_id"
    }
  }
}

template {
  source = "/etc/vault/config.ctmpl"
  destination = "/app/config.env"
  command = "systemctl reload myapp"
}
EOF

vault agent -config=vault-agent.hcl

# Direct env injection with envconsul
envconsul -vault-addr=$VAULT_ADDR -secret=secret/myapp/config myapp
```

## Audit and Compliance

```bash
# Enable audit logging to a file
vault audit enable file file_path=/var/log/vault/audit.log

# Enable audit logging to syslog
vault audit enable syslog

# List enabled audit devices
vault audit list

# Check Vault health/status
vault status

# Check Vault operator info (seal status, cluster)
vault operator init       # initial setup
vault operator unseal     # unseal after restart
vault operator seal       # manually seal

# Check HA status
vault operator members
```

## Common Usage Patterns

```bash
# Read a secret and use it in a script
DB_PASSWORD=$(vault kv get -field=db_password secret/myapp/config)
export DATABASE_URL="postgres://app:${DB_PASSWORD}@localhost:5432/mydb"

# Check if a secret exists before reading
if vault kv get secret/myapp/config > /dev/null 2>&1; then
  echo "Secret exists"
else
  echo "Secret not found"
fi

# Rotate a secret
vault kv patch secret/myapp/config db_password="newpassword123"

# Copy a secret from one path to another
vault kv get -format=json secret/source/config | \
  jq '.data.data' | \
  vault kv put secret/dest/config -

# List all secrets recursively (requires list capability)
vault kv list -format=json secret/ | jq -r '.[]' | while read path; do
  if [[ "$path" == */ ]]; then
    vault kv list "secret/${path}"
  fi
done
```

## Troubleshooting

```bash
# Debug connection issues
VAULT_LOG_LEVEL=debug vault status

# Check token capabilities for a path
vault token capabilities secret/data/myapp/config

# Verify auth method configuration
vault auth list -detailed

# Check lease information
vault list sys/leases/lookup/database/creds/

# Unwrap a wrapped token
vault unwrap <wrapping_token>

# Common errors:
# "permission denied" — check token policy with: vault token capabilities <path>
# "no secret exists" — verify path, remember KV v2 uses 'secret/data/<path>'
# "token has expired" — renew token or generate a new one
```
