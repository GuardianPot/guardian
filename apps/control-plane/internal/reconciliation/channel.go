package reconciliation

import (
	"context"
	"errors"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel"
	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
)

type ChannelHandler struct {
	service *Service
}

func NewChannelHandler(service *Service) (*ChannelHandler, error) {
	if service == nil {
		return nil, errors.New("reconciliation service is required")
	}
	return &ChannelHandler{service: service}, nil
}

func (h *ChannelHandler) DesiredState(ctx context.Context, identity devicechannel.DeviceIdentity) (*devicev1.DesiredStateSnapshot, error) {
	snapshot, err := h.service.DesiredState(ctx, identity.DeviceID)
	if err != nil {
		return nil, err
	}
	return snapshotToWire(snapshot), nil
}

func (h *ChannelHandler) ObservedState(ctx context.Context, identity devicechannel.DeviceIdentity, wire *devicev1.ObservedState) error {
	observed, err := observedFromWire(wire)
	if err != nil {
		return err
	}
	return h.service.ObservedState(ctx, identity.DeviceID, observed)
}

func (h *ChannelHandler) Acknowledgement(ctx context.Context, identity devicechannel.DeviceIdentity, wire *devicev1.Acknowledgement) error {
	if wire == nil || wire.Kind != devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_DESIRED_STATE {
		return ErrInvalidObserved
	}
	return h.service.AcknowledgeDesired(ctx, identity.DeviceID, Acknowledgement{
		MessageID: wire.MessageId,
		Revision:  wire.Revision,
	})
}

func snapshotToWire(snapshot Snapshot) *devicev1.DesiredStateSnapshot {
	zones := make([]*devicev1.NetworkZoneMetadata, 0, len(snapshot.Zones))
	for _, zone := range snapshot.Zones {
		zones = append(zones, &devicev1.NetworkZoneMetadata{
			ZoneId: zone.ZoneID, DisplayName: zone.DisplayName, Cidr: zone.CIDR,
			SourceRevision: zone.SourceRevision,
		})
	}
	decoys := make([]*devicev1.DecoyDesiredObject, 0, len(snapshot.Decoys))
	for _, decoy := range snapshot.Decoys {
		decoys = append(decoys, &devicev1.DecoyDesiredObject{
			DecoyId: decoy.DecoyID, ZoneId: decoy.ZoneID, DisplayName: decoy.DisplayName,
			Family: decoyFamilyToWire(decoy.Family), Persona: decoyPersonaToWire(decoy.Persona),
			InteractionLevel: interactionLevelToWire(decoy.InteractionLevel),
			Address:          decoy.Address, Pack: decoy.Pack, PackVersion: decoy.PackVersion,
			PackDigest:     decoy.PackDigest,
			DesiredState:   desiredDecoyStateToWire(decoy.DesiredState),
			SourceRevision: decoy.SourceRevision,
		})
	}
	return &devicev1.DesiredStateSnapshot{
		MessageId: snapshot.MessageID,
		Revision:  snapshot.Revision,
		EdgeConfiguration: &devicev1.EdgeConfiguration{
			DeviceId: snapshot.EdgeConfiguration.DeviceID, EnvironmentId: snapshot.EdgeConfiguration.EnvironmentID,
		},
		Zones: zones, Decoys: decoys,
	}
}

// The four token-to-enum maps below fall through to UNSPECIFIED rather than
// guessing. An unspecified value fails the channel validator, so a decoy whose
// vocabulary the Control Plane does not recognise is never transmitted.
func decoyFamilyToWire(value string) devicev1.DecoyFamily {
	switch value {
	case "ssh":
		return devicev1.DecoyFamily_DECOY_FAMILY_SSH
	case "http":
		return devicev1.DecoyFamily_DECOY_FAMILY_HTTP
	case "postgres":
		return devicev1.DecoyFamily_DECOY_FAMILY_POSTGRES
	case "smb":
		return devicev1.DecoyFamily_DECOY_FAMILY_SMB
	default:
		return devicev1.DecoyFamily_DECOY_FAMILY_UNSPECIFIED
	}
}

func decoyPersonaToWire(value string) devicev1.DecoyPersona {
	switch value {
	case "linux_admin_server":
		return devicev1.DecoyPersona_DECOY_PERSONA_LINUX_ADMIN_SERVER
	case "internal_admin_web_app":
		return devicev1.DecoyPersona_DECOY_PERSONA_INTERNAL_ADMIN_WEB_APP
	case "database_server":
		return devicev1.DecoyPersona_DECOY_PERSONA_DATABASE_SERVER
	case "windows_file_service_host":
		return devicev1.DecoyPersona_DECOY_PERSONA_WINDOWS_FILE_SERVICE_HOST
	default:
		return devicev1.DecoyPersona_DECOY_PERSONA_UNSPECIFIED
	}
}

func interactionLevelToWire(value string) devicev1.DecoyInteractionLevel {
	switch value {
	case "low":
		return devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_LOW
	case "medium":
		return devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_MEDIUM
	default:
		return devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_UNSPECIFIED
	}
}

func desiredDecoyStateToWire(value string) devicev1.DecoyDesiredLifecycle {
	switch value {
	case "deployed":
		return devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_DEPLOYED
	case "disabled":
		return devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_DISABLED
	default:
		return devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_UNSPECIFIED
	}
}

func observedFromWire(wire *devicev1.ObservedState) (ObservedState, error) {
	if wire == nil || wire.Condition == nil || wire.Condition.LastTransitionTime == nil {
		return ObservedState{}, ErrInvalidObserved
	}
	transition := wire.Condition.LastTransitionTime.AsTime().UTC()
	if err := wire.Condition.LastTransitionTime.CheckValid(); err != nil {
		return ObservedState{}, ErrInvalidObserved
	}
	var retryAt *time.Time
	if wire.Condition.RetryAt != nil {
		if err := wire.Condition.RetryAt.CheckValid(); err != nil {
			return ObservedState{}, ErrInvalidObserved
		}
		value := wire.Condition.RetryAt.AsTime().UTC()
		retryAt = &value
	}
	status, ok := conditionStatusFromWire(wire.Condition.Status)
	if !ok {
		return ObservedState{}, ErrInvalidObserved
	}
	observed := ObservedState{
		MessageID: wire.MessageId, DesiredRevision: wire.DesiredRevision,
		ObservedRevision: wire.ObservedRevision, LastGoodRevision: wire.LastGoodRevision,
		Condition: Condition{
			Status: status, ReasonCode: wire.Condition.ReasonCode,
			AttemptCount: wire.Condition.AttemptCount, RetryAt: retryAt,
			LastTransitionTime: transition,
		},
	}
	if err := ValidateObserved(observed); err != nil {
		return ObservedState{}, err
	}
	return observed, nil
}

func conditionStatusFromWire(value devicev1.ReconciliationConditionStatus) (ConditionStatus, bool) {
	switch value {
	case devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_PENDING:
		return ConditionPending, true
	case devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_CONVERGED:
		return ConditionConverged, true
	case devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_RETRYING:
		return ConditionRetrying, true
	case devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_FAILED:
		return ConditionFailed, true
	default:
		return "", false
	}
}

var _ devicechannel.ReconciliationHandler = (*ChannelHandler)(nil)
