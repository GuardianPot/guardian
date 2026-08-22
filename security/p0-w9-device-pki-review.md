# P0-W9 device PKI security review

Status: Accepted for Phase 0 fixture scope

## Controls reviewed

- The CA is explicitly constructed as `NewTestAuthority`, kept in memory, and
  is not a production key store or enrollment service.
- Enrollment binds the device ID, CSR public key, and a server-issued 32-byte
  nonce. The device signs the canonical proof with the CSR private key.
- Challenges are replaced per device, expire, and are consumed only after a
  valid enrollment or rotation. A replayed request cannot be accepted twice.
- Device certificates are client-auth certificates signed by the test CA.
  Rotation revokes the previous serial while activating the new serial.
- Revocation is checked by the authority and by the TLS server's connection
  verifier. TLS is constrained to version 1.3 in both directions and requires
  a verified client certificate.
- Certificate serials are random 128-bit values and active-device state is
  maintained separately from the CA signing key.

## Residual risks

Production enrollment transport, OS-protected Edge key storage, CA
availability, and durable revocation distribution remain Phase 1 concerns.
This fixture intentionally does not create or accept production trust material.
