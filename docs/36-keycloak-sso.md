# Keycloak SSO (dev)

Status: Keycloak + realm import are in compose, and the full browser round trip
was verified with Playwright/Chromium (`scripts/oidc-drill.js`).

## Start

```bash
docker compose --profile sso up -d keycloak
```

Realm `aip` is imported from `deploy/keycloak/realm-aip.json`:

- client `aip` (confidential, secret `aip-dev-secret`)
- redirect `http://localhost:18080/api/v1/auth/oidc/sso/callback`
- user `demo` with the password from `scripts/seed-demo.sh`

## API configuration

Prefer a single issuer when it is reachable from the API (OIDC discovery fills the
three endpoints). For the split-host dev setup below, explicit URLs are required
because the browser uses `localhost:8081` while the API container uses
`keycloak:8080`:

```bash
AIP_OIDC_SSO_ISSUER=http://keycloak:8080/realms/aip
```

Discovery is implemented (`oidc.Discover`, unit-tested); if it fails the provider
is disabled with a warning instead of booting a broken login.


```bash
AIP_API_URL=http://localhost:18080
AIP_OIDC_SSO_AUTHORIZE_URL=http://localhost:8081/realms/aip/protocol/openid-connect/auth
AIP_OIDC_SSO_TOKEN_URL=http://keycloak:8080/realms/aip/protocol/openid-connect/token
AIP_OIDC_SSO_USERINFO_URL=http://keycloak:8080/realms/aip/protocol/openid-connect/userinfo
AIP_OIDC_SSO_CLIENT_ID=aip
AIP_OIDC_SSO_CLIENT_SECRET=aip-dev-secret
```

The authorize URL is browser-facing (`localhost:8081`); token/userinfo are
container-facing (`keycloak:8080`). The callback must be externally reachable and
match the realm redirect URI.

## SAML login (verified)

Env: `AIP_SAML_IDP_METADATA=http://keycloak:8080/realms/aip/protocol/saml/descriptor`,
`AIP_SAML_ENTITY_ID=aip-saml`; persistent dev key/cert via
`scripts/saml-sp-dev.sh` (`/saml-sp/key.pem`, `/saml-sp/cert.pem`). AuthnRequests
are signed, so the SP certificate must be set as the Keycloak client's
`saml.signing.certificate` (and `saml.client.signature=true`). The client signs
responses and assertions. `scripts/keycloak-saml-client.sh` registers the cert for
signing and encryption (`EncryptedAssertion` verified in the ACS body).

```
node scripts/saml-drill.js
url=http://localhost:3000/projects logged_in=true has_projects=true
```

SLO uses the POST binding (`/saml/logout` → Keycloak → `/saml/slo` → `/login?logged_out=1`). Replay protection is shared through Redis when `AIP_REDIS_ADDR` is set (`aip:saml:replay:*`). Signed AuthnRequests, encrypted assertions, SLO and group role mapping are implemented and covered by the drill; see `scripts/saml-drill.js`.

## SAML metadata (config only)

Keycloak's IdP descriptor is at
`http://keycloak:8080/realms/aip/protocol/saml/descriptor`. The SSO panel can
fetch it (`POST /orgs/{org}/sso/fetch`); the API stores the entity id and signing
certificate. Private/internal targets are blocked unless
`AIP_SSO_ALLOW_PRIVATE_METADATA=true` (set it for the local Keycloak). Live: blocked
400 by default, 200 with the flag. This section only covers metadata storage; the
login flow itself is implemented and drilled above.

## Verified

- discovery: `GET /realms/aip/.well-known/openid-configuration` → 200
- password grant to the realm token endpoint → 200
- `/api/v1/auth/oidc/sso/start` → 302 to the Keycloak authorize URL with the
  expected `client_id`, `redirect_uri` and `state`

## Browser drill (verified)

```
node scripts/oidc-drill.js
url=http://localhost:3000/projects logged_in=true
```

- Keycloak login form accepted `demo`
- API callback issued the session (refresh token in browser storage)
- `/projects` rendered **Novasoft CRM**, no sign-in form

Command-line clients (curl/requests) drop Keycloak's `Secure` cookies over HTTP,
so they fail with `cookie_not_found`; a browser treats `localhost` as a secure
context.

Keycloak runs `start-dev` with a bootstrap admin and is not a production IdP
configuration. SAML login is implemented and verified against Keycloak; OIDC is
the other supported login path.
