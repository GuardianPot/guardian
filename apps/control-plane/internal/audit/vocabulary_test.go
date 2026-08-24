package audit

import (
	"errors"
	"testing"
)

func TestClosedVocabularyAndActionObjectPairs(t *testing.T) {
	want := map[Action]ObjectType{
		"auth.bootstrap_token.created":     ObjectTypeBootstrapToken,
		"auth.bootstrap.succeeded":         ObjectTypeUser,
		"auth.bootstrap.failed":            ObjectTypeBootstrapToken,
		"auth.login.succeeded":             ObjectTypeUser,
		"auth.login.failed":                ObjectTypeUser,
		"auth.logout":                      ObjectTypeSession,
		"auth.password.changed":            ObjectTypeUser,
		"auth.mfa.enrolled":                ObjectTypeUser,
		"auth.recovery_code.used":          ObjectTypeUser,
		"auth.session.revoked":             ObjectTypeSession,
		"auth.security_setting.changed":    ObjectTypeSecuritySetting,
		"device.enrollment_token.created":  ObjectTypeEnrollmentToken,
		"device.enrollment_token.revoked":  ObjectTypeEnrollmentToken,
		"device.enrollment.succeeded":      ObjectTypeDevice,
		"device.enrollment.failed":         ObjectTypeDevice,
		"device.certificate.issued":        ObjectTypeDeviceCertificate,
		"device.certificate.rotated":       ObjectTypeDeviceCertificate,
		"device.disabled":                  ObjectTypeDevice,
		"device.revoked":                   ObjectTypeDevice,
		"environment.created":              ObjectTypeEnvironment,
		"environment.updated":              ObjectTypeEnvironment,
		"zone.created":                     ObjectTypeZone,
		"zone.updated":                     ObjectTypeZone,
		"zone.removed":                     ObjectTypeZone,
		"desired_state.revision.published": ObjectTypeDesiredStateRevision,
		"security.action.denied":           ObjectTypeSecurityAction,
	}

	actions := Actions()
	if len(actions) != len(want) {
		t.Fatalf("Actions() returned %d actions, want %d", len(actions), len(want))
	}
	seen := make(map[Action]bool, len(actions))
	for _, action := range actions {
		if seen[action] {
			t.Fatalf("duplicate action %q", action)
		}
		seen[action] = true
		if !action.Valid() {
			t.Errorf("declared action %q is invalid", action)
		}
		objectType, ok := action.ObjectType()
		if !ok || objectType != want[action] {
			t.Errorf("action %q object = %q, %v; want %q", action, objectType, ok, want[action])
		}
		if err := ValidateActionObject(action, objectType); err != nil {
			t.Errorf("valid pair %q/%q rejected: %v", action, objectType, err)
		}
	}

	actions[0] = "changed"
	if Actions()[0] == "changed" {
		t.Fatal("Actions returned shared mutable state")
	}
}

func TestVocabularyRejectsUnknownAndMismatchedValues(t *testing.T) {
	for _, actorType := range []ActorType{ActorTypeSystem, ActorTypeUser, ActorTypeDevice} {
		if err := actorType.Validate(); err != nil {
			t.Errorf("valid actor type %q rejected: %v", actorType, err)
		}
	}
	if err := ActorType("administrator").Validate(); !errors.Is(err, ErrInvalidActorType) {
		t.Fatalf("unknown actor error = %v", err)
	}
	if err := Action("incident.created").Validate(); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("future action error = %v", err)
	}
	if err := ObjectType("incident").Validate(); !errors.Is(err, ErrInvalidObjectType) {
		t.Fatalf("future object error = %v", err)
	}
	if err := ValidateActionObject(ActionLoginFailed, ObjectTypeSession); !errors.Is(err, ErrActionObject) {
		t.Fatalf("mismatched pair error = %v", err)
	}
}
