# ADR 0001 — SAML login: library, not hand-rolled

Status: **accepted and implemented as a first slice** (SP metadata, SP-initiated login, ACS, single-instance replay cache, Keycloak browser drill).

## Context

SAML login needs XML-DSig verification, canonicalization, audience/destination/
recipient checks, `InResponseTo`/replay protection, clock skew handling and
certificate rotation. The platform currently stores SAML configuration
(`org_sso`) but has no ACS; OIDC login is implemented and verified.

A hand-rolled verifier is not acceptable: canonicalization and signature-wrapping
mistakes are a known source of critical vulnerabilities.

## Options

| Option | Pros | Cons |
|---|---|---|
| A. Hand-rolled XML-DSig | no dependency | high risk; canonicalization/replay edge cases |
| B. `crewjam/saml` library | mature SP implementation, metadata/ACS helpers | new dependency; must pin and review |
| C. External SAML proxy (e.g. satosa/Keycloak broker) | no XML in our code | another service to operate; still need mapping |

## Decision (proposed)

Take **B**, after a security review of the pinned version, with **C** as fallback
if the library cannot satisfy the review. Do not ship SAML login without an
independent test against a real IdP (Keycloak realm with a SAML client) and the
negative tests below.

## Implemented

- SP key/cert: `AIP_SAML_KEY_FILE`/`AIP_SAML_CERT_FILE`; `scripts/saml-sp-dev.sh`
  writes a persistent dev pair under `.cache/saml-sp` (mounted at `/saml-sp`).
  Without files an in-memory pair is generated, but signed requests will fail
  after a restart until the IdP client certificate is refreshed
- `GET /saml/metadata`, `GET /saml/login`, `POST /saml/acs`
- RelayState carries the AuthnRequest id and is validated by ParseResponse
- assertion signature/audience/destination/recipient/InResponseTo/conditions are
  checked by the library
- replay cache keyed by assertion id until NotOnOrAfter: Redis when
  `AIP_REDIS_ADDR` is set (shared), in-memory otherwise
- Keycloak SAML client (`aip-saml`, signed responses/assertions) and
  `scripts/saml-drill.js`: `url=…/projects logged_in=true has_projects=true`

- signed AuthnRequests (RSA-SHA256): the SP certificate must be registered on the
  IdP client (`saml.signing.certificate`); `scripts/saml-rotate-sp.sh` rotates the
  dev key/cert, restarts the API and re-registers with Keycloak (backups kept)
- encrypted assertions: verified with Keycloak (`EncryptedAssertion` in the ACS
  body); `scripts/keycloak-saml-client.sh` registers sign/encrypt certs
- group→role mapping: `AIP_SAML_ROLE_MAP` maps signed group values to
  `admin|member|viewer` (`group:role,...`); `AIP_SAML_ADMIN_GROUPS` remains a
  shorthand for admin. Owners are never downgraded

## Not implemented

- SLO is implemented with the **HTTP-POST** binding (`/saml/logout` auto-submits a
  signed LogoutRequest, `/saml/slo` clears the nameid cookie). The redirect binding
  failed with Keycloak `invalid_signature`; POST is verified:
  final URL `…/login?logged_out=1`
- handler-level negative tests cover missing RelayState, parse/signature errors,
  replay rejection and the success redirect; signature-wrapping/canonicalization
  cases remain the library's responsibility and are not fuzzed

## Required behavior

- SP metadata endpoint with ACS URL and signing certificate.
- SP-initiated login; `RelayState` allowlist (no open redirects).
- Verify signature, audience, destination, recipient, `NotBefore/NotOnOrAfter`,
  `InResponseTo`, and reject replayed assertion IDs (store used IDs with TTL).
- Map `NameID`/email to an existing user; never auto-create an owner.
- Refuse login when `org_sso.enabled` is false or metadata is stale.
- Audit `auth.login.saml` with the IdP entity ID (never assertion content).

## Negative tests before "done"

1. Unsigned assertion → rejected.
2. Signature wrapping / duplicate ID → rejected.
3. Wrong audience/destination → rejected.
4. Expired/not-yet-valid or skewed assertion → rejected.
5. Replayed assertion → rejected.
6. Unknown email/org mismatch → rejected without account creation.
7. Disabled SSO config → rejected.

## Consequences

- New dependency and ADR pin; supply-chain/SBOM must include it.
- OIDC remains the recommended path; SAML is for IdPs that require it.
