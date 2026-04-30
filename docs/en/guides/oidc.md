# OpenID Connect (OIDC) Integration Guide

AxonHub supports authentication via any OIDC-compliant Identity Provider (IdP) such as Google, GitHub, or Logto.

## Configuration

Configure your OIDC providers in the `conf/config.yml` file under the `oidc` section.

```yaml
# Public URL of your AxonHub instance (critical for building redirect URIs)
server:
  public_url: "https://axonhub.example.com"

oidc:
  providers:
    - id: "google"
      name: "google"
      display_name: "Google SSO"
      issuer_url: "https://accounts.google.com"
      client_id: "YOUR_GOOGLE_CLIENT_ID"
      client_secret: "YOUR_GOOGLE_CLIENT_SECRET"
      extra_scopes: ["openid", "profile", "email"]
      jit_enabled: true              # Automatically create user on first login (Just-In-Time)
      auto_link_by_email: true       # Link to existing user by verified email
      require_email_verified: true   # Only link if email is verified by IdP
      enable_pkce: true              # Recommended for enhanced security (RFC 7636)
      sync_user_info: true           # Sync name and avatar on every login
      button_color: "#DB4437"        # UI Customization: Button color
      icon_url: "https://www.google.com/favicon.ico" # UI Customization: Icon URL
```

### Provider Options

| Option | Description |
|---|---|
| `id` | Unique identifier for the provider (used for callback URLs). |
| `name` | Human-readable internal name. |
| `display_name` | Name shown on the login button. |
| `issuer_url` | The OIDC issuer URL (discovery endpoint). |
| `client_id` | OAuth2 Client ID. |
| `client_secret` | OAuth2 Client Secret. |
| `extra_scopes` | Extra OAuth2 scopes. Note: must include `openid` if provided. |
| `redirect_url` | (Optional) Manually override the absolute redirect URI. |
| `jit_enabled` | If true, new users will be created automatically on first login. |
| `auto_link_by_email` | If true, matches existing users by email. |
| `require_email_verified` | If true, only link if email is verified by IdP. |
| `enable_pkce` | Enables Proof Key for Code Exchange (RFC 7636). Highly recommended. |
| `sync_user_info` | Updates user profile (name, avatar) from IdP claims. |
| `button_color` | Hex color code for the login button. |
| `icon_url` | Icon URL or local path for the login button (supports base64). |
| `default_roles` | List of roles to assign to new JIT users (e.g., `["Viewer"]`). |
| `oidc_login_only` | If true, forces users of this provider to only use SSO login. |

## Advanced Configuration

### Role Mapping

You can automatically assign AxonHub roles or specific permission scopes to users based on groups or claims returned by the OIDC provider.

```yaml
oidc:
  providers:
    - id: "logto"
      # ... base configuration
      group_claims: ["groups", "roles"] # Claim keys to look for group info
      role_mappings:
        - match_group: "admin-group"    # Group name from OIDC
          db_role: "system:owner"       # Mapped AxonHub role (System Owner)
          priority: 100
        - match_group: "viewer-*"       # Supports wildcard matching
          db_role: "Viewer"
          priority: 10
        - match_group: "api-users"
          db_role: "scope:chat:create"  # Directly map to a specific scope
          priority: 5
```

| Mapping Option | Description |
|---|---|
| `group_claims` | List of claim names in the OIDC token containing group info (default: `["groups", "roles"]`). |
| `role_mappings` | List of mapping rules. |
| `match_group` | Group name to match, supports wildcard `*`. |
| `is_regex` | Whether to treat `match_group` as a regular expression. |
| `db_role` | Target role name (e.g., `Viewer`) or scope prefixed with `scope:` (e.g., `scope:chat:create`). |
| `priority` | Priority used when multiple rules match and `role_precedence_mode` is set to `highest`. |

## Setup Examples

### Logto (Self-Hosted)

1. Create a **Traditional Web** application in Logto.
2. Set the **Redirect URI**:
   - If you have **multiple** OIDC providers: `<public_url>/oauth/oidc/callback/logto`
   - If this is the **only** provider: `<public_url>/oauth/oidc/callback`
3. Set the **Post sign-out URI** to `<public_url>/sign-in`.
4. Add the following to your AxonHub config:

```yaml
oidc:
  providers:
    - id: "logto"
      name: "logto"
      display_name: "Logto SSO"
      issuer_url: "https://your-logto-domain.com/oidc"
      client_id: "<App ID>"
      client_secret: "<App Secret>"
      enable_pkce: true
      jit_enabled: true
      button_color: "#5C5CFF"
```

## Security Considerations

- **Public URL (Critical)**: Always set `server.public_url` in production environments. If not set, AxonHub will fall back to the `Host` header of the request to build redirect URIs, which can be vulnerable to **Host Header Injection** attacks in reverse proxy setups.
- **PKCE**: It is highly recommended to enable `enable_pkce: true` for all providers.
- **SSO Only**: If you want to enforce SSO and block password-based logins for specific users, you can enable `oidc_login_only: true` for a provider. **Note: When enabled, the frontend login page will automatically hide the password form and only show SSO options.** You can also use the user management UI to set an individual user's password to the magic placeholder `!OIDC_SSO_ONLY!` to lock specific accounts.
