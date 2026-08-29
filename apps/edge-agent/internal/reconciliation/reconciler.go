// Package reconciliation owns the Edge desired-state convergence loop.
package reconciliation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const retryPollInterval = 200 * time.Millisecond

type Publisher interface {
	PublishObserved(*devicev1.ObservedState) error
}

type PublisherFunc func(*devicev1.ObservedState) error

func (function PublisherFunc) PublishObserved(observed *devicev1.ObservedState) error {
	return function(observed)
}

type Applier interface {
	Apply(context.Context, *devicev1.DesiredStateSnapshot) error
}

type ApplyFailure struct {
	ReasonCode string
	Retryable  bool
}

func (failure *ApplyFailure) Error() string { return failure.ReasonCode }

type metadataApplier struct{}

func (metadataApplier) Apply(context.Context, *devicev1.DesiredStateSnapshot) error { return nil }

type Reconciler struct {
	store     *storage.Store
	deviceID  string
	publisher Publisher
	applier   Applier
	now       func() time.Time

	applyMu sync.Mutex
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
}

func New(store *storage.Store, deviceID string, publisher Publisher, applier Applier) (*Reconciler, error) {
	if store == nil || !validUUIDv7(deviceID) || publisher == nil {
		return nil, errors.New("reconciliation store, device identity, and publisher are required")
	}
	if applier == nil {
		applier = metadataApplier{}
	}
	return &Reconciler{store: store, deviceID: deviceID, publisher: publisher, applier: applier, now: time.Now}, nil
}

func (*Reconciler) Name() string { return "reconciler" }

func (r *Reconciler) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.mu.Unlock()
		return errors.New("reconciler already started")
	}
	runContext, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.done = make(chan struct{})
	r.mu.Unlock()
	if err := r.Trigger(runContext); err != nil {
		cancel()
		r.mu.Lock()
		r.cancel = nil
		r.done = nil
		r.mu.Unlock()
		return err
	}
	if err := r.store.SetHealth(runContext, storage.HealthCondition{Name: "reconciler", Status: "healthy", ReasonCode: "ready"}); err != nil {
		cancel()
		r.mu.Lock()
		r.cancel = nil
		r.done = nil
		r.mu.Unlock()
		return err
	}
	go r.retryLoop(runContext, r.done)
	return nil
}

func (r *Reconciler) Stop(ctx context.Context) error {
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		r.mu.Lock()
		if r.done == done {
			r.cancel = nil
			r.done = nil
		}
		r.mu.Unlock()
		return r.store.SetHealth(ctx, storage.HealthCondition{Name: "reconciler", Status: "stopped", ReasonCode: "shutdown"})
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Reconciler) DesiredState(ctx context.Context, desired *devicev1.DesiredStateSnapshot) error {
	if desired == nil || desired.Revision == 0 || !validUUIDv7(desired.MessageId) {
		return errors.New("desired-state envelope is invalid")
	}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(desired)
	if err != nil {
		return errors.New("desired-state serialization failed")
	}
	digest, err := desiredContentDigest(desired)
	if err != nil {
		return err
	}
	reason := validateSnapshot(desired, r.deviceID)
	observedMessageID, err := newUUIDv7(r.now().UTC())
	if err != nil {
		return err
	}
	accepted, err := r.store.AcceptReconciliationCandidate(ctx, storage.ReconciliationCandidate{
		MessageID: desired.MessageId, Revision: desired.Revision, Digest: digest,
		Payload: payload, TerminalReason: reason, ObservedMessageID: observedMessageID,
		Now: r.now().UTC(),
	})
	if err != nil {
		return err
	}
	record := accepted.Record
	if accepted.ShouldApply {
		record, err = r.applyRecord(ctx, record)
		if err != nil {
			return err
		}
	}
	return r.publish(record)
}

func (r *Reconciler) Acknowledgement(ctx context.Context, acknowledgement *devicev1.Acknowledgement) error {
	if acknowledgement == nil || acknowledgement.Kind != devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE {
		return errors.New("observed acknowledgement is invalid")
	}
	return r.store.AcknowledgeObserved(ctx, acknowledgement.MessageId, acknowledgement.Revision)
}

func (r *Reconciler) Trigger(ctx context.Context) error {
	record, err := r.store.ReconciliationState(ctx)
	if errors.Is(err, storage.ErrReconciliationStateNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	now := r.now().UTC()
	if record.ConditionStatus == "pending" ||
		(record.ConditionStatus == "retrying" && record.RetryAt != nil && !record.RetryAt.After(now)) {
		record, err = r.applyRecord(ctx, record)
		if err != nil {
			return err
		}
	}
	return r.publish(record)
}

func (r *Reconciler) applyRecord(ctx context.Context, record storage.ReconciliationRecord) (storage.ReconciliationRecord, error) {
	r.applyMu.Lock()
	defer r.applyMu.Unlock()
	current, err := r.store.ReconciliationState(ctx)
	if err != nil {
		return storage.ReconciliationRecord{}, err
	}
	if current.DesiredMessageID != record.DesiredMessageID || current.DesiredRevision != record.DesiredRevision ||
		current.ConditionStatus == "converged" || current.ConditionStatus == "failed" {
		return current, nil
	}
	var desired devicev1.DesiredStateSnapshot
	applyErr := proto.Unmarshal(current.DesiredPayload, &desired)
	if applyErr == nil {
		if reason := validateSnapshot(&desired, r.deviceID); reason != "" {
			applyErr = &ApplyFailure{ReasonCode: reason}
		} else {
			applyErr = r.applier.Apply(ctx, &desired)
		}
	}
	result := storage.ReconciliationResult{
		ExpectedMessageID: current.DesiredMessageID, ExpectedRevision: current.DesiredRevision,
		ExpectedDigest: current.DesiredDigest, Now: r.now().UTC(),
	}
	result.ObservedMessageID, err = newUUIDv7(result.Now)
	if err != nil {
		return storage.ReconciliationRecord{}, err
	}
	if applyErr == nil {
		result.Success = true
	} else {
		var failure *ApplyFailure
		if errors.As(applyErr, &failure) && validReasonCode(failure.ReasonCode) {
			result.Retryable = failure.Retryable
			result.ReasonCode = failure.ReasonCode
		} else {
			result.Retryable = true
			result.ReasonCode = "apply_failed"
		}
	}
	return r.store.RecordReconciliationResult(ctx, result)
}

func (r *Reconciler) publish(record storage.ReconciliationRecord) error {
	if !record.ObservedPending {
		return nil
	}
	condition := &devicev1.ReconciliationCondition{
		Status: conditionStatus(record.ConditionStatus), ReasonCode: record.ReasonCode,
		AttemptCount:       record.AttemptCount,
		LastTransitionTime: timestamppb.New(record.LastTransitionAt.UTC()),
	}
	if record.RetryAt != nil {
		condition.RetryAt = timestamppb.New(record.RetryAt.UTC())
	}
	return r.publisher.PublishObserved(&devicev1.ObservedState{
		MessageId: record.ObservedMessageID, DesiredRevision: record.DesiredRevision,
		ObservedRevision: record.ObservedRevision, LastGoodRevision: record.LastGoodRevision,
		Condition: condition,
	})
}

func (r *Reconciler) retryLoop(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(retryPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = r.Trigger(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func validateSnapshot(snapshot *devicev1.DesiredStateSnapshot, deviceID string) string {
	if snapshot == nil || snapshot.EdgeConfiguration == nil {
		return "invalid_snapshot"
	}
	if hasUnknown(snapshot) {
		return "unsupported_object"
	}
	if snapshot.EdgeConfiguration.DeviceId != deviceID {
		return "identity_mismatch"
	}
	if !validUUID(snapshot.EdgeConfiguration.EnvironmentId) || len(snapshot.Zones) > 200 || len(snapshot.PlaceholderDecoys) > 64 {
		return "invalid_snapshot"
	}
	zones := make(map[string]struct{}, len(snapshot.Zones))
	previous := ""
	for _, zone := range snapshot.Zones {
		if zone == nil || !validUUID(zone.ZoneId) || !validText(zone.DisplayName, 512) || zone.SourceRevision == 0 ||
			!validPrivateIPv4Prefix(zone.Cidr) || (previous != "" && zone.ZoneId <= previous) {
			return "invalid_snapshot"
		}
		previous = zone.ZoneId
		zones[zone.ZoneId] = struct{}{}
	}
	previous = ""
	for _, decoy := range snapshot.PlaceholderDecoys {
		if decoy == nil || !validUUID(decoy.ObjectId) || !validUUID(decoy.ZoneId) || !validText(decoy.DisplayName, 512) ||
			(previous != "" && decoy.ObjectId <= previous) {
			return "invalid_snapshot"
		}
		if _, ok := zones[decoy.ZoneId]; !ok {
			return "invalid_snapshot"
		}
		previous = decoy.ObjectId
	}
	return ""
}

func desiredContentDigest(snapshot *devicev1.DesiredStateSnapshot) ([sha256.Size]byte, error) {
	copySnapshot, ok := proto.Clone(snapshot).(*devicev1.DesiredStateSnapshot)
	if !ok {
		return [sha256.Size]byte{}, errors.New("clone desired-state snapshot")
	}
	copySnapshot.MessageId = ""
	copySnapshot.Revision = 0
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(copySnapshot)
	if err != nil {
		return [sha256.Size]byte{}, errors.New("serialize desired-state content")
	}
	return sha256.Sum256(payload), nil
}

func hasUnknown(snapshot *devicev1.DesiredStateSnapshot) bool {
	if len(snapshot.ProtoReflect().GetUnknown()) != 0 ||
		(snapshot.EdgeConfiguration != nil && len(snapshot.EdgeConfiguration.ProtoReflect().GetUnknown()) != 0) {
		return true
	}
	for _, zone := range snapshot.Zones {
		if zone != nil && len(zone.ProtoReflect().GetUnknown()) != 0 {
			return true
		}
	}
	for _, decoy := range snapshot.PlaceholderDecoys {
		if decoy != nil && len(decoy.ProtoReflect().GetUnknown()) != 0 {
			return true
		}
	}
	return false
}

func conditionStatus(status string) devicev1.ReconciliationConditionStatus {
	switch status {
	case "pending":
		return devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_PENDING
	case "converged":
		return devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_CONVERGED
	case "retrying":
		return devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_RETRYING
	case "failed":
		return devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_FAILED
	default:
		return devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_UNSPECIFIED
	}
}

func newUUIDv7(now time.Time) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[6:]); err != nil {
		return "", fmt.Errorf("generate reconciliation UUIDv7: %w", err)
	}
	milliseconds := uint64(now.UTC().UnixMilli())
	value[0] = byte(milliseconds >> 40)
	value[1] = byte(milliseconds >> 32)
	value[2] = byte(milliseconds >> 24)
	value[3] = byte(milliseconds >> 16)
	value[4] = byte(milliseconds >> 8)
	value[5] = byte(milliseconds)
	value[6] = (value[6] & 0x0f) | 0x70
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], value[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], value[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], value[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], value[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], value[10:16])
	return string(encoded), nil
}

func validUUIDv7(value string) bool { return validUUID(value) && value[14] == '7' }

func validUUID(value string) bool {
	if len(value) != 36 || value != strings.ToLower(value) || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || !strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil && len(decoded) == 16
}

func validText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validReasonCode(value string) bool {
	if len(value) < 1 || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validPrivateIPv4Prefix(value string) bool {
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.Addr().Is4() || prefix != prefix.Masked() || prefix.String() != value {
		return false
	}
	for _, root := range []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.168.0.0/16"),
	} {
		if prefix.Bits() >= root.Bits() && root.Contains(prefix.Addr()) {
			return true
		}
	}
	return false
}

var _ interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
} = (*Reconciler)(nil)
