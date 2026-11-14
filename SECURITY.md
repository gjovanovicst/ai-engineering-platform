# Security policy

## Supported versions

Security fixes target the `main` branch and the latest tagged release. Older
releases may receive a fix at the maintainers' discretion.

## Reporting a vulnerability

**Do not open a public issue for a vulnerability.**

Use GitHub's private vulnerability reporting on the repository
(Security → Advisories → Report a vulnerability). If that is unavailable,
contact a maintainer privately before disclosing anything.

Please include:

- affected component and version/commit
- a minimal reproduction or proof of concept
- impact and the conditions required
- whether credentials, tenant data or the sandbox boundary are involved
- any suggested fix

Never include live provider keys, session tokens, customer data or production
database dumps. Redact secrets before attaching logs.

## What to expect

- acknowledgement within 3 business days
- triage and severity assessment within 7 business days
- a coordinated disclosure date once a fix is available
- credit in the advisory unless you ask to remain anonymous

## Scope

In scope:

- API and web authentication/session handling
- tenant isolation and RBAC
- the agent sandbox boundary and egress controls
- secret handling, artifact storage and audit integrity
- CI/release supply chain

Out of scope:

- vulnerabilities in third-party providers that do not affect this code
- social engineering or physical attacks
- denial of service from an intentionally unbounded local test
- issues that require a compromised host or cluster admin

## Safe harbour

We will not pursue legal action for good-faith research that follows this policy,
avoids privacy violations and does not degrade service for others.

## Known dependency exceptions

- **GO-2026-6909** (`github.com/crewjam/saml`, no upstream fix as of 2026-11-14):
  nil-pointer panic on a signed assertion without `<Issuer>`, and namespace-blind
  `Subject.NameID`. The SAML ACS handler recovers parser panics and returns a
  login error; assertions are only accepted from the configured trusted IdP.
  The residual namespace ambiguity is tracked upstream. Remove this exception
  as soon as a fixed release exists.

`make security` accepts only this ID; any new vulnerability fails the scan.

## Security documentation

- threat model: [docs/31-threat-model.md](docs/31-threat-model.md)
- audit integrity: [docs/32-audit-integrity.md](docs/32-audit-integrity.md)
- sandbox hardening: [docs/55-sandbox-hardening.md](docs/55-sandbox-hardening.md)
- egress policy: [docs/73-egress-policy.md](docs/73-egress-policy.md)
