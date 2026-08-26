package enrollment

import (
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
)

func TestRotationScheduleIsStableInsideFinalWindow(t *testing.T) {
	notAfter := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	metadata := identity.Metadata{CertificateSHA256: "0123456789abcdef", NotAfter: notAfter}
	first, second := rotationTime(metadata), rotationTime(metadata)
	if first != second {
		t.Fatalf("rotation schedule changed: %s != %s", first, second)
	}
	if first.Before(notAfter.Add(-rotationWindow)) || !first.Before(notAfter.Add(-minimumExpiryMargin)) {
		t.Fatalf("rotation time %s is outside the approved window", first)
	}
	if nextRetry(8*time.Minute) != 15*time.Minute || nextRetry(time.Minute) != 2*time.Minute {
		t.Fatal("retry schedule is not bounded")
	}
}
