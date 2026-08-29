package reconciliation

import (
	"context"
	"errors"
)

type Repository interface {
	EnsureCurrent(context.Context, string, string) (Snapshot, error)
	RecordObserved(context.Context, string, ObservedState) error
	AcknowledgeDesired(context.Context, string, Acknowledgement) error
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("reconciliation repository is required")
	}
	return &Service{repository: repository}, nil
}

func (s *Service) DesiredState(ctx context.Context, deviceID string) (Snapshot, error) {
	if !validUUIDv7(deviceID) {
		return Snapshot{}, ErrInvalidSnapshot
	}
	return s.repository.EnsureCurrent(ctx, deviceID, "device-channel")
}

func (s *Service) ObservedState(ctx context.Context, deviceID string, observed ObservedState) error {
	if !validUUIDv7(deviceID) || ValidateObserved(observed) != nil {
		return ErrInvalidObserved
	}
	return s.repository.RecordObserved(ctx, deviceID, observed)
}

func (s *Service) AcknowledgeDesired(ctx context.Context, deviceID string, acknowledgement Acknowledgement) error {
	if !validUUIDv7(deviceID) || !validUUIDv7(acknowledgement.MessageID) || acknowledgement.Revision == 0 {
		return ErrInvalidObserved
	}
	return s.repository.AcknowledgeDesired(ctx, deviceID, acknowledgement)
}
