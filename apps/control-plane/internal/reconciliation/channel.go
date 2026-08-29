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
	decoys := make([]*devicev1.PlaceholderDecoyDesiredObject, 0, len(snapshot.PlaceholderDecoys))
	for _, decoy := range snapshot.PlaceholderDecoys {
		decoys = append(decoys, &devicev1.PlaceholderDecoyDesiredObject{
			ObjectId: decoy.ObjectID, ZoneId: decoy.ZoneID, DisplayName: decoy.DisplayName,
		})
	}
	return &devicev1.DesiredStateSnapshot{
		MessageId: snapshot.MessageID,
		Revision:  snapshot.Revision,
		EdgeConfiguration: &devicev1.EdgeConfiguration{
			DeviceId: snapshot.EdgeConfiguration.DeviceID, EnvironmentId: snapshot.EdgeConfiguration.EnvironmentID,
		},
		Zones: zones, PlaceholderDecoys: decoys,
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
