# Users and access

Workspace access is role-based and auditable. The same role model applies to the UI and the API.

## Roles

| Role | Can |
|---|---|
| Owner | Manage members, workspace settings, security and workspace deletion |
| Admin | Manage projects, blueprints, providers, integrations, rules and audit |
| Member | Work with tasks, runs, reviews and pull requests in accessible projects |
| Viewer | Read projects, runs, checks and metrics; cannot change state |

Project access can be narrower than the workspace role. A member of the workspace does not automatically see every project.

## Invite a user

1. Open **Settings → Invites**.
2. Enter the email and choose a role.
3. Send the invite.
4. The user accepts it and joins the workspace.

Pending invites can be revoked. The invite email is sent by the configured mailer (Mailhog locally, Postmark/SES in production).

## Members

**Settings → Members** lists everyone in the workspace. Owners and admins can change a member's role or remove them.

Removing a user revokes their sessions and API keys for that workspace. Their audit history remains.

## Single sign-on

The platform supports OIDC and SAML:

- **OIDC** — GitHub, Google, Keycloak or another provider
- **SAML 2.0** — enterprise identity providers
- **SCIM** — provision and deprovision users and groups from the identity provider

SSO can be required for a workspace. When it is enforced, password login is disabled for members.

## API keys

**Settings → API keys** creates a scoped key for automation. A key belongs to a user and inherits their workspace role; it can be revoked at any time.

- Store the key in a secret manager.
- Use one key per integration.
- Rotate on a schedule and after a team change.
- Never put a key in a task, a prompt or a repository.

## Sessions

Users can sign out of the current session or all sessions. Revoking all sessions is the right action after a lost device or a role change.

## Next

- [Security and audit](security-audit.html)
- [API basics](api-basics.html)
