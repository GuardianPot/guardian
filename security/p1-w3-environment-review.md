# P1-W3 environment-domain security review

## Review scope and conclusion

This review covers the P1-W3 singleton organization, environment and private
IPv4 zone domain, HTTP boundary, PostgreSQL migration/query adapter, P1-W2
authorization integration, P1-W10 atomic audit integration, device foreign
key, and no-scan assertion. The implemented boundary satisfies the approved
W3-C1-A through W3-C7-A package within its stated Phase 1 limits.

The design does not add organization selection, environment deletion, public
networks, IPv6, nested-zone precedence, network discovery, routing/firewall
authority, health inference, or device/deception activation.

## Identity and scope conclusion

Migration `00005` inserts exactly one organization with a PostgreSQL 18
UUIDv7. A database trigger rejects update and delete. There is no organization
mutation or selector route. Environments reference that singleton and all
environment/zone storage reads verify the singleton relationship. Environment
and zone identities are PostgreSQL UUIDv7 values.

Names are valid UTF-8, trimmed and NFC-normalized before use. They are bounded
to 128 Unicode code points and 512 bytes, reject control characters, and use a
Unicode case-fold key under a parent-scoped unique constraint. Arbitrary
descriptions and tags are absent, reducing stored-content and audit-snapshot
exposure.

## Network-input conclusion

The application uses `net/netip` canonical prefix parsing and accepts only
host-bit-free IPv4 CIDRs wholly contained by the three RFC1918 roots. The
database adds an independent IPv4, positive-mask, RFC1918 containment check and
stores the value as `cidr`. The validation matrix rejects malformed, public,
unspecified, loopback, link-local, multicast, IPv6, zero-prefix, duplicate,
nested, and overlapping values.

A transaction-scoped advisory lock keyed by environment serializes zone-set
changes. Each transaction locks the owning environment, re-evaluates overlap,
and then mutates. Therefore two concurrent overlapping writes yield one commit
and one domain conflict. The lock and query scope deliberately permit the same
private prefix in separate environments.

## Authorization and HTTP conclusion

Every environment route first requires authenticated HTTPS state. Reads pass
the single bounded `__Host-guardian_session` cookie to P1-W2
`AuthorizeRead`. Mutations pass that session plus exactly one bounded
`X-CSRF-Token` and `Origin` value to `AuthorizeMutation`; production compares
the Origin to the configured public origin and validates the synchronizer
token in constant time. Constructors default to a deny authorizer.

The API exposes no organization input. UUIDv7 path validation returns a
not-found result without revealing alternate scope. Request JSON is capped at
16 KiB, rejects unknown fields and trailing values, and requires the declared
JSON media type. Lists accept only one optional `limit`, default 50, maximum
200. Response bodies are `no-store`; database errors are logged without their
detail and returned as generic statuses.

Environment and zone updates, and zone removal, require a single strong quoted
revision ETag. A missing precondition returns `428`; malformed input returns
`400`; a stale revision returns `412`. These paths return before mutation and
audit. Environment deletion has no handler and returns `405`.

## Transaction, audit, and device-integrity conclusion

Environment/zone writes and their P1-W10 event append share one PostgreSQL
transaction. Events use only approved action/object pairs and safe snapshots:
IDs, normalized display name, CIDR, revision, zone count, and configuration
status. Passwords, sessions, CSRF values, authorization headers, cookies,
tokens, private keys, and arbitrary request bodies are absent. A forced audit
insert failure proves the domain write rolls back.

Zone mutations also advance the environment revision so an environment ETag
changes when its configuration set changes. The restrictive device foreign
key rejects a device whose environment is absent and prevents a referenced
environment from being removed through direct SQL. Existing development data
with orphans must be reset and reseeded as approved; migration does not invent
ownership or preserve invalid rows.

## No-scan and authority conclusion

The save path contains validation, authorization, SQL, and audit operations
only. A focused static gate rejects socket dialing, raw-packet, process-exec,
scanner, routing, netlink, and firewall primitives in the P1-W3 domain, API,
and storage files. Integration saves two zones without any injected network
adapter. No endpoint claims that a configured CIDR is reachable, healthy,
protected, enrolled, or deployed.

## Evidence

The authoritative fixture is:

```console
task environment:integration
```

It runs against disposable PostgreSQL 18 and a real TLS test server with a
bootstrapped P1-W2 owner, TOTP login, server-side session, CSRF token, and exact
Origin. Covered evidence includes:

- immutable singleton and UUIDv7 identities;
- normalized-name uniqueness and CIDR boundary matrix;
- two valid RFC1918 zones and status projection;
- same-environment overlap, cross-environment reuse, and concurrent winner;
- stale/missing ETags, strict body/query limits, sanitized errors, and `405`;
- atomic audit success/rollback, bounded redacted snapshots, and restart;
- restrictive device referential integrity; and
- source-level no-scan/no-route/no-firewall and secret-shape gates.

Unit and repository gates additionally run through `task validate`. The local
Windows evidence can run the same Go integration selection against the pinned
PostgreSQL 18 Compose service when a POSIX shell is unavailable.

## Residual risks and limitations

- PostgreSQL remains the online consistency, authorization-data, and audit
  availability dependency. Backup, restore, high availability, production
  database roles, and monitoring are later operational work.
- The advisory-lock key uses PostgreSQL's stable hash function with a P1-W3
  namespace seed. A hash collision can serialize unrelated environments but
  cannot permit an overlap or cross-scope mutation.
- Unicode confusables are not collapsed; uniqueness is NFC plus Unicode
  case-fold, not a visual-identity policy. The immutable UUID is authoritative.
- Phase 1 has one owner and one implicit organization. Multi-user roles and
  multi-tenancy require a new approved architecture/security package.
- Configuration status is intentionally not runtime truth. Operators must not
  interpret `zones_defined` as discovered, reachable, healthy, or protected.
- The no-scan gate proves this write path lacks active-network primitives; it
  is not a substitute for host egress controls or later privileged-boundary
  review.

Product Owner acceptance remains required after CI and review. A green PR does
not itself grant release, security, merge, environment, secret, or production
authority.
