-- +goose Up

-- Change proposal 0003: CSRF-proof re-issue.
--
-- One audit action joins the closed Phase 1 vocabulary. The proposal's impact
-- section anticipated exactly this and nothing else: "No schema is modified.
-- No database migration is required beyond whatever the audit vocabulary needs
-- for the new event type."
--
-- The pair constraint is a CHECK rather than a lookup table, so extending the
-- vocabulary means replacing it. Every existing pair is carried over verbatim;
-- `auth.csrf.reissued` is the only addition.
ALTER TABLE guardian_audit.events DROP CONSTRAINT audit_action_object_pair;

ALTER TABLE guardian_audit.events ADD CONSTRAINT audit_action_object_pair CHECK ((action, object_type) IN (
    ('auth.bootstrap_token.created', 'bootstrap_token'),
    ('auth.bootstrap.succeeded', 'user'),
    ('auth.bootstrap.failed', 'bootstrap_token'),
    ('auth.login.succeeded', 'user'),
    ('auth.login.failed', 'user'),
    ('auth.logout', 'session'),
    ('auth.password.changed', 'user'),
    ('auth.mfa.enrolled', 'user'),
    ('auth.recovery_code.used', 'user'),
    ('auth.session.revoked', 'session'),
    -- The new pair. The object is the session whose proof was replaced, so an
    -- auditor reading a session's history sees the re-issue in it.
    ('auth.csrf.reissued', 'session'),
    ('auth.security_setting.changed', 'security_setting'),
    ('device.enrollment_token.created', 'enrollment_token'),
    ('device.enrollment_token.revoked', 'enrollment_token'),
    ('device.enrollment.succeeded', 'device'),
    ('device.enrollment.failed', 'device'),
    ('device.certificate.issued', 'device_certificate'),
    ('device.certificate.rotated', 'device_certificate'),
    ('device.disabled', 'device'),
    ('device.revoked', 'device'),
    ('environment.created', 'environment'),
    ('environment.updated', 'environment'),
    ('zone.created', 'zone'),
    ('zone.updated', 'zone'),
    ('zone.removed', 'zone'),
    ('desired_state.revision.published', 'desired_state_revision'),
    ('security.action.denied', 'security_action')
));
