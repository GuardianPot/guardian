package audit

import (
	"errors"
	"fmt"
)

// ActorType is the closed Phase 1 audit-actor vocabulary.
type ActorType string

const (
	ActorTypeSystem ActorType = "system"
	ActorTypeUser   ActorType = "user"
	ActorTypeDevice ActorType = "device"
)

// Action is the closed Phase 1 audit-action vocabulary.
type Action string

const (
	ActionBootstrapTokenCreated  Action = "auth.bootstrap_token.created"
	ActionBootstrapSucceeded     Action = "auth.bootstrap.succeeded"
	ActionBootstrapFailed        Action = "auth.bootstrap.failed"
	ActionLoginSucceeded         Action = "auth.login.succeeded"
	ActionLoginFailed            Action = "auth.login.failed"
	ActionLogout                 Action = "auth.logout"
	ActionPasswordChanged        Action = "auth.password.changed"
	ActionMFAEnrolled            Action = "auth.mfa.enrolled"
	ActionRecoveryCodeUsed       Action = "auth.recovery_code.used"
	ActionSessionRevoked         Action = "auth.session.revoked"
	ActionSecuritySettingChanged Action = "auth.security_setting.changed"
	ActionEnrollmentTokenCreated Action = "device.enrollment_token.created"
	ActionEnrollmentTokenRevoked Action = "device.enrollment_token.revoked"
	ActionEnrollmentSucceeded    Action = "device.enrollment.succeeded"
	ActionEnrollmentFailed       Action = "device.enrollment.failed"
	ActionCertificateIssued      Action = "device.certificate.issued"
	ActionCertificateRotated     Action = "device.certificate.rotated"
	ActionDeviceDisabled         Action = "device.disabled"
	ActionDeviceRevoked          Action = "device.revoked"
	ActionEnvironmentCreated     Action = "environment.created"
	ActionEnvironmentUpdated     Action = "environment.updated"
	ActionZoneCreated            Action = "zone.created"
	ActionZoneUpdated            Action = "zone.updated"
	ActionZoneRemoved            Action = "zone.removed"
	ActionDesiredStatePublished  Action = "desired_state.revision.published"
	ActionSecurityActionDenied   Action = "security.action.denied"
)

// ObjectType is the closed Phase 1 audit-object vocabulary.
type ObjectType string

const (
	ObjectTypeBootstrapToken       ObjectType = "bootstrap_token"
	ObjectTypeUser                 ObjectType = "user"
	ObjectTypeSession              ObjectType = "session"
	ObjectTypeSecuritySetting      ObjectType = "security_setting"
	ObjectTypeEnrollmentToken      ObjectType = "enrollment_token"
	ObjectTypeDevice               ObjectType = "device"
	ObjectTypeDeviceCertificate    ObjectType = "device_certificate"
	ObjectTypeEnvironment          ObjectType = "environment"
	ObjectTypeZone                 ObjectType = "zone"
	ObjectTypeDesiredStateRevision ObjectType = "desired_state_revision"
	ObjectTypeSecurityAction       ObjectType = "security_action"
)

var (
	ErrInvalidActorType  = errors.New("invalid audit actor type")
	ErrInvalidAction     = errors.New("invalid audit action")
	ErrInvalidObjectType = errors.New("invalid audit object type")
	ErrActionObject      = errors.New("audit action and object type do not match")

	actionObjectTypes = map[Action]ObjectType{
		ActionBootstrapTokenCreated:  ObjectTypeBootstrapToken,
		ActionBootstrapSucceeded:     ObjectTypeUser,
		ActionBootstrapFailed:        ObjectTypeBootstrapToken,
		ActionLoginSucceeded:         ObjectTypeUser,
		ActionLoginFailed:            ObjectTypeUser,
		ActionLogout:                 ObjectTypeSession,
		ActionPasswordChanged:        ObjectTypeUser,
		ActionMFAEnrolled:            ObjectTypeUser,
		ActionRecoveryCodeUsed:       ObjectTypeUser,
		ActionSessionRevoked:         ObjectTypeSession,
		ActionSecuritySettingChanged: ObjectTypeSecuritySetting,
		ActionEnrollmentTokenCreated: ObjectTypeEnrollmentToken,
		ActionEnrollmentTokenRevoked: ObjectTypeEnrollmentToken,
		ActionEnrollmentSucceeded:    ObjectTypeDevice,
		ActionEnrollmentFailed:       ObjectTypeDevice,
		ActionCertificateIssued:      ObjectTypeDeviceCertificate,
		ActionCertificateRotated:     ObjectTypeDeviceCertificate,
		ActionDeviceDisabled:         ObjectTypeDevice,
		ActionDeviceRevoked:          ObjectTypeDevice,
		ActionEnvironmentCreated:     ObjectTypeEnvironment,
		ActionEnvironmentUpdated:     ObjectTypeEnvironment,
		ActionZoneCreated:            ObjectTypeZone,
		ActionZoneUpdated:            ObjectTypeZone,
		ActionZoneRemoved:            ObjectTypeZone,
		ActionDesiredStatePublished:  ObjectTypeDesiredStateRevision,
		ActionSecurityActionDenied:   ObjectTypeSecurityAction,
	}
)

// Valid reports whether t is in the closed actor vocabulary.
func (t ActorType) Valid() bool {
	switch t {
	case ActorTypeSystem, ActorTypeUser, ActorTypeDevice:
		return true
	default:
		return false
	}
}

// Validate rejects actor values outside the closed vocabulary.
func (t ActorType) Validate() error {
	if !t.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidActorType, t)
	}
	return nil
}

// Valid reports whether a is in the closed action vocabulary.
func (a Action) Valid() bool {
	_, ok := actionObjectTypes[a]
	return ok
}

// Validate rejects action values outside the closed vocabulary.
func (a Action) Validate() error {
	if !a.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidAction, a)
	}
	return nil
}

// ObjectType returns the sole Phase 1 object type allowed for this action.
func (a Action) ObjectType() (ObjectType, bool) {
	t, ok := actionObjectTypes[a]
	return t, ok
}

// Valid reports whether t is in the closed object vocabulary.
func (t ObjectType) Valid() bool {
	switch t {
	case ObjectTypeBootstrapToken,
		ObjectTypeUser,
		ObjectTypeSession,
		ObjectTypeSecuritySetting,
		ObjectTypeEnrollmentToken,
		ObjectTypeDevice,
		ObjectTypeDeviceCertificate,
		ObjectTypeEnvironment,
		ObjectTypeZone,
		ObjectTypeDesiredStateRevision,
		ObjectTypeSecurityAction:
		return true
	default:
		return false
	}
}

// Validate rejects object values outside the closed vocabulary.
func (t ObjectType) Validate() error {
	if !t.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidObjectType, t)
	}
	return nil
}

// ValidateActionObject enforces the approved one-to-one Phase 1 action/object
// pairing. It deliberately rejects individually valid but mismatched values.
func ValidateActionObject(action Action, objectType ObjectType) error {
	if err := action.Validate(); err != nil {
		return err
	}
	if err := objectType.Validate(); err != nil {
		return err
	}
	want, _ := action.ObjectType()
	if objectType != want {
		return fmt.Errorf("%w: action %q requires %q, got %q", ErrActionObject, action, want, objectType)
	}
	return nil
}

// Actions returns a stable copy of every approved action in declaration order.
func Actions() []Action {
	return []Action{
		ActionBootstrapTokenCreated,
		ActionBootstrapSucceeded,
		ActionBootstrapFailed,
		ActionLoginSucceeded,
		ActionLoginFailed,
		ActionLogout,
		ActionPasswordChanged,
		ActionMFAEnrolled,
		ActionRecoveryCodeUsed,
		ActionSessionRevoked,
		ActionSecuritySettingChanged,
		ActionEnrollmentTokenCreated,
		ActionEnrollmentTokenRevoked,
		ActionEnrollmentSucceeded,
		ActionEnrollmentFailed,
		ActionCertificateIssued,
		ActionCertificateRotated,
		ActionDeviceDisabled,
		ActionDeviceRevoked,
		ActionEnvironmentCreated,
		ActionEnvironmentUpdated,
		ActionZoneCreated,
		ActionZoneUpdated,
		ActionZoneRemoved,
		ActionDesiredStatePublished,
		ActionSecurityActionDenied,
	}
}
