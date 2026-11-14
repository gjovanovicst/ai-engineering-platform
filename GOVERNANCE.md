# Governance

## Maintainers

Maintainers review and merge changes, cut releases and own security triage. The
initial maintainer list is kept in `CODEOWNERS` (added when the public repository
is created). New maintainers are nominated by an existing maintainer and accepted
by lazy consensus after a week of review.

## Decisions

- Small changes: PR review by one maintainer.
- Architectural or breaking changes: an ADR in `docs/adr/`, public review, and
  lazy consensus.
- Security issues: private advisory and coordinated disclosure; details are
  published after a fix.
- Roadmap: issues and the roadmap document, not private chat.

## Releases

- Semantic versioning for the API and container images.
- `main` is releasable; tags are immutable.
- Release notes list breaking changes, migrations and rollback steps.
- No release is made from uncommitted work or without `make check`.

## Contact

- Security: private GitHub security advisory (see `SECURITY.md`).
- Conduct: the private contact listed on the repository profile until a dedicated
  address is published.

## Scope boundaries

The project is an open core. Hosted operations, support/SLA and compliance
attestations may be commercial. Security fixes and the core ASDLC are not
paywalled.
