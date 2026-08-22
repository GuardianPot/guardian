# Security policy

## Reporting

Do not disclose suspected vulnerabilities, credentials, private incident
details, or exploit material in public channels. This repository is private;
report security findings to the repository owner through a private GitHub
channel or the organization owner account.

Include the affected component, reproduction conditions, impact, and any safe
mitigation. Do not include real secrets in issues, pull requests, logs, or
test fixtures.

## Responsible testing boundary

Guardian decoys, lab networking, replay, and attacker-simulation tests must
run only in an isolated environment that the operator owns or is explicitly
authorized to test. Never direct test traffic at an unauthorized network,
host, identity system, or third-party service.

## Secrets and release security

Production secrets, root trust material, signing keys, and privileged release
authority must never be placed in an agent environment or committed to this
repository. Report accidental exposure immediately and rotate the affected
credential.
