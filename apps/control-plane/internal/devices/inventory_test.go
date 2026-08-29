package devices

import (
	"context"
	"errors"
	"testing"
)

type inventoryRepositoryStub struct {
	listEnvironment string
	listLimit       int32
	getEnvironment  string
	getDevice       string
}

func (repository *inventoryRepositoryStub) ListInventoryDevices(_ context.Context, environmentID string, limit int32) ([]InventoryDevice, error) {
	repository.listEnvironment, repository.listLimit = environmentID, limit
	return []InventoryDevice{{EnvironmentID: environmentID}}, nil
}

func (repository *inventoryRepositoryStub) InventoryDevice(_ context.Context, environmentID, deviceID string) (InventoryDevice, error) {
	repository.getEnvironment, repository.getDevice = environmentID, deviceID
	return InventoryDevice{EnvironmentID: environmentID, DeviceID: deviceID}, nil
}

func TestInventoryServiceValidatesIdentifiersAndCapsLists(t *testing.T) {
	repository := &inventoryRepositoryStub{}
	service, err := NewInventoryService(repository)
	if err != nil {
		t.Fatal(err)
	}
	environmentID := "018f1f7e-6d31-7cc5-8db8-17547f78e6c1"
	deviceID := "018f1f7e-6d31-7cc5-8db8-17547f78e6c2"
	items, err := service.ListDevices(context.Background(), environmentID)
	if err != nil || len(items) != 1 || repository.listEnvironment != environmentID || repository.listLimit != 200 {
		t.Fatalf("ListDevices() = (%+v, %v), repository=(%q, %d)", items, err, repository.listEnvironment, repository.listLimit)
	}
	item, err := service.Device(context.Background(), environmentID, deviceID)
	if err != nil || item.DeviceID != deviceID || repository.getEnvironment != environmentID || repository.getDevice != deviceID {
		t.Fatalf("Device() = (%+v, %v), repository=(%q, %q)", item, err, repository.getEnvironment, repository.getDevice)
	}
	if _, err := service.ListDevices(context.Background(), "not-a-uuid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid environment error = %v", err)
	}
	if _, err := service.Device(context.Background(), environmentID, "not-a-uuid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid device error = %v", err)
	}
}

func TestInventoryServiceRequiresRepository(t *testing.T) {
	if _, err := NewInventoryService(nil); err == nil {
		t.Fatal("NewInventoryService(nil) unexpectedly succeeded")
	}
}
