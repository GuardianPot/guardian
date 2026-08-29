package health

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

var ErrNotFound = errors.New("health projection not found")

// DeviceProjection binds persisted health truth to its inventory identity.
// Projection is nil until the first report or trusted disconnect evidence.
type DeviceProjection struct {
	DeviceID      string
	EnvironmentID string
	State         string
	Projection    *Projection
}

// Repository is the durable P1-W9 health boundary.
type Repository interface {
	StoreHealthReport(context.Context, string, Report, time.Time) (ApplyOutcome, error)
	MarkHealthDisconnected(context.Context, string, time.Time) error
	DeviceHealth(context.Context, string) (DeviceProjection, error)
	ActiveEnvironmentHealth(context.Context, string) ([]DeviceProjection, error)
}

type SourcedCondition struct {
	Condition
	SourceDeviceID string `json:"source_device_id,omitempty"`
}

type ViewAggregate struct {
	Status           Status        `json:"status"`
	BlockingType     ConditionType `json:"blocking_type,omitempty"`
	Reason           string        `json:"reason,omitempty"`
	BlockingDeviceID string        `json:"blocking_device_id,omitempty"`
}

type View struct {
	Aggregate     ViewAggregate      `json:"aggregate"`
	Conditions    []SourcedCondition `json:"conditions"`
	ReceivedAt    time.Time          `json:"received_at"`
	DeviceID      string             `json:"device_id,omitempty"`
	EnvironmentID string             `json:"environment_id,omitempty"`
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("health repository is required")
	}
	return &Service{repository: repository, now: func() time.Time {
		return time.Now().UTC().Truncate(TimestampPrecision)
	}}, nil
}

func (s *Service) Ingest(ctx context.Context, deviceID string, report Report) error {
	_, err := s.repository.StoreHealthReport(ctx, deviceID, report, s.now())
	return err
}

func (s *Service) Disconnected(ctx context.Context, deviceID string) error {
	return s.repository.MarkHealthDisconnected(ctx, deviceID, s.now())
}

func (s *Service) Device(ctx context.Context, deviceID string) (View, error) {
	record, err := s.repository.DeviceHealth(ctx, deviceID)
	if err != nil {
		return View{}, err
	}
	now := s.now()
	conditions, receivedAt, err := effectiveOrUnknown(record.Projection, now)
	if err != nil {
		return View{}, err
	}
	aggregate, err := AggregateConditions(conditions)
	if err != nil {
		return View{}, err
	}
	sourced := make([]SourcedCondition, len(conditions))
	for index, condition := range conditions {
		sourced[index] = SourcedCondition{Condition: condition, SourceDeviceID: record.DeviceID}
	}
	blockingDeviceID := ""
	if aggregate.Status != StatusTrue {
		blockingDeviceID = record.DeviceID
	}
	return View{
		Aggregate: ViewAggregate{Status: aggregate.Status, BlockingType: aggregate.BlockingType,
			Reason: aggregate.Reason, BlockingDeviceID: blockingDeviceID},
		Conditions: sourced, ReceivedAt: receivedAt, DeviceID: record.DeviceID,
		EnvironmentID: record.EnvironmentID,
	}, nil
}

func (s *Service) Environment(ctx context.Context, environmentID string) (View, error) {
	records, err := s.repository.ActiveEnvironmentHealth(ctx, environmentID)
	if err != nil {
		return View{}, err
	}
	now := s.now()
	if len(records) == 0 {
		conditions, err := UnknownConditions(now)
		if err != nil {
			return View{}, err
		}
		sourced := make([]SourcedCondition, len(conditions))
		for index, condition := range conditions {
			sourced[index] = SourcedCondition{Condition: condition}
		}
		return View{
			Aggregate:  ViewAggregate{Status: StatusUnknown, Reason: "no_active_devices"},
			Conditions: sourced, ReceivedAt: now, EnvironmentID: environmentID,
		}, nil
	}
	sort.Slice(records, func(left, right int) bool { return records[left].DeviceID < records[right].DeviceID })
	type evaluated struct {
		deviceID   string
		conditions []Condition
	}
	evaluatedRecords := make([]evaluated, 0, len(records))
	receivedAt := time.Time{}
	for _, record := range records {
		conditions, observed, evaluateErr := effectiveOrUnknown(record.Projection, now)
		if evaluateErr != nil {
			return View{}, evaluateErr
		}
		if observed.After(receivedAt) {
			receivedAt = observed
		}
		evaluatedRecords = append(evaluatedRecords, evaluated{record.DeviceID, conditions})
	}
	sourced := make([]SourcedCondition, 0, len(conditionTypeOrder))
	for index := range conditionTypeOrder {
		selected := evaluatedRecords[0]
		for _, candidate := range evaluatedRecords[1:] {
			if severity(candidate.conditions[index].Status) > severity(selected.conditions[index].Status) {
				selected = candidate
			}
		}
		sourced = append(sourced, SourcedCondition{
			Condition: selected.conditions[index], SourceDeviceID: selected.deviceID,
		})
	}
	plain := make([]Condition, len(sourced))
	for index := range sourced {
		plain[index] = sourced[index].Condition
	}
	aggregate, err := AggregateConditions(plain)
	if err != nil {
		return View{}, err
	}
	blockingDeviceID := ""
	if aggregate.Status != StatusTrue {
		for _, condition := range sourced {
			if condition.Type == aggregate.BlockingType {
				blockingDeviceID = condition.SourceDeviceID
				break
			}
		}
	}
	return View{
		Aggregate: ViewAggregate{Status: aggregate.Status, BlockingType: aggregate.BlockingType,
			Reason: aggregate.Reason, BlockingDeviceID: blockingDeviceID},
		Conditions: sourced, ReceivedAt: receivedAt, EnvironmentID: environmentID,
	}, nil
}

func effectiveOrUnknown(projection *Projection, now time.Time) ([]Condition, time.Time, error) {
	if projection == nil {
		conditions, err := UnknownConditions(now)
		return conditions, now, err
	}
	conditions, err := projection.EffectiveConditions(now)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("evaluate health projection: %w", err)
	}
	return conditions, projection.ReceivedAt, nil
}

func severity(status Status) int {
	switch status {
	case StatusFalse:
		return 3
	case StatusUnknown:
		return 2
	case StatusTrue:
		return 1
	default:
		return 0
	}
}
