# P1-W10 audit security review

- Review date: 2026-08-24
- Work package: P1-W10
- Decisions: CP-02, DA-01, SA-13, AUTH-06, H1-A through H6-A
- Scope: Phase 1 append-oriented PostgreSQL audit baseline and authorized read
  API

## Data and trust boundary conclusion

Audit is a first-class Control Plane module backed by the existing PostgreSQL
system of record. No broker, second datastore, retention service, WORM device,
or cryptographic ledger is introduced. The audit package owns typed records,
the closed Phase 1 vocabulary, validation, snapshot redaction, pagination
contracts, and append/read interfaces. PostgreSQL implementation details remain
under the storage package; the API depends only on the audit contract.

The externally durable identifier is a PostgreSQL 18 UUIDv7. A separate
identity sequence provides a total append order for keyset pagination and is
not returned as an event field. `occurred_at` represents the caller's event
time; database-generated `recorded_at` remains the trusted persistence time.
Both timestamps are constrained, after PostgreSQL microsecond normalization, to
finite UTC instants above the Go zero time that remain serializable as RFC 3339
years 1 through 9999. Neither timestamp is presented as cryptographic time
attestation.

Snapshot columns use PostgreSQL `json`, not `jsonb`, so the database can enforce
the approved 16 KiB limit against the exact canonical bytes emitted by the
snapshot constructor. PostgreSQL still validates the JSON and the versioned
object envelope. Snapshot contents are deliberately not queried or indexed;
all searchable audit dimensions are typed columns.

## Append-only and transaction conclusion

The product exposes only append and read queries. PostgreSQL triggers reject
row update/delete and table truncation. The serving role receives table-level
`SELECT`, column-level `INSERT` only for caller-owned event fields, and sequence
`USAGE`; it cannot supply `sequence`, `event_id`, or `recorded_at`, including
with `INSERT ... OVERRIDING SYSTEM VALUE`. Database checks also reject
non-positive sequence values and event IDs that are not RFC-variant UUIDv7.
Required audited mutations use an appender backed by the same `pgx` transaction
as the domain write. Validation, redaction, SQL, or commit failure therefore
rolls back both writes.

This is defense in depth against product bugs and runtime-role compromise, not
immutability against the database owner or an administrator. A privileged
operator can alter triggers, change schema, or manipulate storage outside the
product. Phase 1 deliberately makes no cryptographic non-repudiation, hash
chain, notarization, or legal-archive claim.

## Snapshot and hostile-data conclusion

Before/after data is accepted only through an explicit safe-projection
constructor. The `guardian.audit.snapshot.v1` envelope recursively redacts
known credential fields and enforces exact limits for encoded bytes, nesting,
node count, members, key bytes, and string bytes. Unsupported values, invalid
text, and limit violations return an error; they are never silently truncated.
The error propagates through the transaction boundary.

The redactor covers passwords and password hash/digest variants; session,
bootstrap, enrollment, recovery, auth, bearer, identity, access and refresh
tokens; API/client secrets and API keys; TOTP/MFA and CSRF values;
authorization/cookie values; and encoded private-key material. Explicit token
IDs, key fingerprints, and password-policy references remain visible. Snapshot
values are JSON-encoded on output, so hostile strings are data rather than
executable markup or log format. Invalid UTF-16 surrogate escapes are rejected
rather than silently converted. The API and server logs do not emit raw
snapshots or database errors.

Known-key redaction cannot prove that a caller did not place a secret under an
arbitrary misleading key. Callers remain responsible for minimal safe
projections, and later domain packages require their own representative
redaction tests. Passing raw request/domain/database objects is prohibited.

## Read authorization and pagination conclusion

Audit collection reads require the operation-level
`__Host-guardian_session` cookie authorizer. Before P1-W2 supplies that
authorizer, the endpoint returns `401` before invoking the reader. Unsupported
collection methods have no mutation handler and return `405`. Authorization is
re-evaluated for every page; the cursor is not a credential or authorization
grant.

The cursor is versioned, Base64URL-encoded, structurally validated, and carries
an initial sequence anchor plus an exclusive next position. Appends take a
shared transaction advisory lock before identity allocation and hold it through
commit; first-page anchor acquisition takes the corresponding exclusive lock.
The anchor therefore waits for lower allocated sequences to commit or roll
back, while later appends allocate above it. New appends cannot cause duplicates
or skipped rows in an existing traversal. Filters are exact and bounded; object
type and object ID must be supplied together. Cursor contents are not signed
because changing an anchor can only select other audit rows already visible to
the same authorized principal.

## Required abuse and failure evidence

Acceptance requires all of the following to pass on the final candidate:

- closed actor/action/object vocabulary rejection;
- recursive secret redaction and every snapshot boundary;
- mutation plus audit commit and rollback in one transaction;
- database `UPDATE`, `DELETE`, and `TRUNCATE` rejection;
- constrained-runtime normal append plus generated-column override rejection;
- default-deny and denied-session behavior before reader access;
- unsupported HTTP mutation-method rejection;
- malformed query/cursor and filter-pair rejection;
- stable newest-first anchor pagination during concurrent append;
- restart persistence and concurrent unique UUIDv7/sequence append;
- generated SQL freshness, Go vet/unit/integration tests, OpenAPI validation,
  vulnerability/module verification, and the full repository validation gate.

## Residual risks and later gates

- Audit availability and durability inherit the single PostgreSQL deployment's
  backup, restore, access-control, and failure characteristics.
- `occurred_at`, actor IDs, object IDs, correlation IDs, and request IDs are
  bounded caller assertions; `recorded_at` is the database persistence time.
- A compromised serving process can append misleading rows within its allowed
  columns and database constraints. Column grants and append-only triggers stop
  generated-field forgery and historical mutation, but do not make the runtime
  process a trusted audit oracle. Application safe-projection validation is not
  a replacement for database access control and process isolation.
- The Phase 1 action vocabulary intentionally excludes incident, AI, update,
  decoy, and suppression/allowlist operations. Their owner-reviewed packages
  must extend both Go and database constraints atomically.
- P1-W2 must supply authenticated session lookup before audit reads become
  available. Later role differentiation, if approved, must remain default-deny.
- Retention deletion, archive export, SIEM integration, cryptographic
  notarization, and database-administrator controls remain outside P1-W10.

No P1-W10 implementation may be represented as tamper-proof against a database
administrator, and no audit acceptance may substitute for the Product Owner's
security review.
