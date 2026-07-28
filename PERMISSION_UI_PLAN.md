# Permission And UI Plan

## Goals

- Project roles grant usable project access without requiring system scopes.
- The server publishes effective project scopes so API authorization, routes, menus, and controls share one source of truth.
- Read permission shows information; write permission alone exposes mutation controls.
- Project members select from project-approved, enabled models without viewing global Channel configuration.

## Role Presets

New projects create these project roles. Invitations bind to a concrete project role when created.

| Role | Scope set | Intended use |
| --- | --- | --- |
| Project Admin | `read/write_users`, `read/write_roles`, `read/write_api_keys`, `read/write_prompts`, `read/write_requests` | Manage the project, its members, and its resources. |
| Developer | `read/write_api_keys`, `read/write_prompts`, `write_requests` | Use the project, manage API keys and prompts, and use Playground. Personal request visibility is not treated as project-wide request access. |
| Viewer | `read_prompts`, `read_requests` | Inspect permitted project data without changing it. |

System Channels, Models, Data Storage, Settings, and global analytics remain system-level resources. A project role never receives `read_channels` or `write_channels` merely to use Playground.

## Authorization Contract

1. `UserProjectInfo.effectiveScopes` is the union of direct membership scopes and roles assigned in that project.
2. Frontend navigation, route guards, and mutation affordances use `effectiveScopes` for the selected project.
3. The server continues to authorize every query and mutation. A hidden control is never an authorization boundary.
4. Invitation creation validates that the selected role belongs to the invitation project and that the inviter may grant its scopes. Registration creates membership and role assignment in one transaction.

## UI Contract

| Capability | UI behavior |
| --- | --- |
| Read resource | Show route, list, detail values, and static state. |
| Write resource | Show create, edit, delete, bulk actions, switches, inline editors, and ordering controls. |
| Read-only Channel/Model/Prompt | Render status as a non-interactive badge. Hide mutation columns and controls. |
| Project Playground | Query project-profile-approved enabled models only. Disabled or excluded Channels are never listed. |

The system Channel list remains an audit/configuration view: a system user with `read_channels` can see disabled Channels as static rows. The project model picker is a separate availability view and excludes them at the API boundary.

## Delivery Order

1. Add effective scopes and correct default project roles.
2. Bind invitations to a validated project role.
3. Switch frontend permission consumers to effective scopes and align the Playground navigation guard.
4. Gate read-only controls in Channel, Model, Prompt, and API Key surfaces.
5. Add focused tests for role assignment, effective scopes, invitation registration, and read-only UI behavior.
