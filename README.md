# entra-mcp

Microsoft Entra ID (Azure AD) MCP server. Expose read-only identity and access data to AI agents via the Model Context Protocol.

## Features

- **User lookup** — search, get profile, group memberships, MFA methods, manager
- **Group lookup** — search, get details, list members and owners
- **Account hygiene reports** — disabled users, inactive users, guests, stale guests, password-never-expires, synced users, privileged role members
- **License reporting** — list subscribed SKUs with capacity and consumed counts
- **Combined report** — `entra.generate_account_review_report` aggregates all hygiene checks in one call
- **Policy enforcement** — YAML-based allow/deny rules via `mcp-policy`

## Requirements

- Entra ID App Registration (client credentials / app-only)
- Required API permissions:
  - `User.Read.All`
  - `Group.Read.All`
  - `GroupMember.Read.All`
  - `Directory.Read.All`
  - `RoleManagement.Read.Directory`
  - `Organization.Read.All`
  - `UserAuthenticationMethod.Read.All`
  - `AuditLog.Read.All` *(for sign-in timestamps — requires Entra ID P1/P2)*

## Configuration

| Environment variable | Required | Default | Description |
|----------------------|----------|---------|-------------|
| `ENTRA_MCP_TENANT_ID` | ✓ | — | Entra tenant ID |
| `ENTRA_MCP_CLIENT_ID` | ✓ | — | App registration client ID |
| `ENTRA_MCP_CLIENT_SECRET` | ✓ | — | App registration client secret |
| `ENTRA_MCP_ADDR` | | `:8080` | Listen address |
| `ENTRA_MCP_POLICY_FILE` | | `/etc/entra-mcp/policy.yaml` | Policy file path (optional) |

## Running

```bash
export ENTRA_MCP_TENANT_ID=your-tenant-id
export ENTRA_MCP_CLIENT_ID=your-client-id
export ENTRA_MCP_CLIENT_SECRET=your-client-secret

./entra-mcp
```

## Policy

Optional YAML policy file restricts which tools and resources are accessible. See `policy.yaml.example` for the format. If the file does not exist, all tools are allowed (open mode).

## Claude Code integration

Add to your `.mcp.json`:

```json
{
  "mcpServers": {
    "entra-mcp": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

## Tools

### Users

| Tool | Description |
|------|-------------|
| `entra.search_user` | Search by name, email, or UPN (max 50) |
| `entra.get_user` | Full profile by UPN or object ID |
| `entra.get_user_groups` | Direct group memberships |
| `entra.get_transitive_user_groups` | All memberships including nested/dynamic |
| `entra.get_user_auth_methods_summary` | MFA method types registered (counts only) |
| `entra.get_user_manager` | Direct manager |

### Groups

| Tool | Description |
|------|-------------|
| `entra.search_group` | Search by display name (max 50) |
| `entra.get_group` | Group details by object ID |
| `entra.list_group_members` | Direct members (max 200) |
| `entra.list_group_owners` | Group owners |

### Reports

| Tool | Description |
|------|-------------|
| `entra.find_disabled_users` | Accounts with accountEnabled = false (max 200) |
| `entra.find_inactive_users` | Members with no sign-in in N days (max 200) |
| `entra.find_guest_users` | External guest accounts (max 200) |
| `entra.find_stale_guests` | Guests with no sign-in in N days (max 200) |
| `entra.find_password_never_expires` | DisablePasswordExpiration policy accounts (max 200) |
| `entra.find_synced_users` | On-premises AD synced accounts (max 200) |
| `entra.find_privileged_role_members` | Members of high-privilege directory roles |
| `entra.list_subscribed_skus` | M365 license SKUs with capacity/consumed counts |
| `entra.generate_account_review_report` | Combined hygiene report |

> **Note:** Sign-in timestamp fields (`find_inactive_users`, `find_stale_guests`) require `AuditLog.Read.All` permission and Entra ID P1/P2 licensing.

## Build

```bash
go build -o entra-mcp .
```

## License

MIT
