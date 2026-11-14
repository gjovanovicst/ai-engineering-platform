# Open-source readiness

Status: **prepared, not published**. The repository still has no public remote
and its full history has not been audited.

## Decision

License: **Apache-2.0** for the core, with `NOTICE` and a trademark policy to be
added before launch. Commercial hosting, support/SLA and compliance work may be
separate; security fixes and the core ASDLC are not paywalled.

## Public vs internal

Publish:

- `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `GOVERNANCE.md`, `LICENSE`
- product, architecture, API, data model, auth, security model, testing,
  deployment and the user guide
- `site/` and the generated docs portal

Keep private or sanitize first:

- `docs/58`–`docs/68` drill reports (hostnames, topology, restore evidence)
- `docs/37`–`docs/53` HA/failover drills with internal addresses
- `docs/61`/`docs/62` Vault AppRole details
- `docs/56` fencing preflight host details
- demo credentials and any internal customer name

The public docs site (`site/docs/`) is separate from the internal `docs/` tree.

## Checklist

- [x] Apache-2.0 `LICENSE` and `NOTICE`
- [x] `SECURITY.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `GOVERNANCE.md`
- [x] issue and PR templates
- [x] `make oss-audit` working-tree secret/artifact check
- [ ] scan the **entire git history** with gitleaks/trufflehog
- [ ] rotate any credential ever committed (assume compromise)
- [ ] split or sanitize the internal docs listed above
- [ ] create a **fresh public repository** from the cleaned tree (do not publish
      the current history)
- [ ] enable GitHub private vulnerability reporting and branch protection
- [ ] add CODEOWNERS, support policy and a public roadmap
- [ ] publish container images with SBOM/provenance
- [ ] confirm no third-party dependency conflicts with Apache-2.0

## Publishing procedure

```bash
make oss-audit
gitleaks detect --source . --redact   # or trufflehog git file://.
# then export the cleaned tree to a new repository, never the old .git
```

The new repository gets a fresh history. The private repository remains the
internal record until the split is complete.

## Known gaps that must stay in the README

- host fencing is not available on the reference host
- sync RPO is off; replication is asynchronous
- multi-region is a routing guard, not a data plane
- egress is offline/fail-closed, not an allowlist
- K8s is live-drilled on kind only
