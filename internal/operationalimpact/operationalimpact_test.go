package operationalimpact

import (
	"testing"

	"github.com/atlasgraph/atlas/internal/models"
)

func mustEvaluate(t *testing.T, a Assumptions) Assessment {
	t.Helper()
	got, err := Evaluate(a)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestLowSubstitutesIncreaseSectorMoreThanCountry(t *testing.T) {
	a := mustEvaluate(t, Assumptions{DurationDays: 30, RecoverySpeed: "Moderate", SubstituteAvailability: "Low", InventoryBufferDays: 30})
	if a.Multiplier(models.Sector) <= a.Multiplier(models.Country) {
		t.Fatalf("sector multiplier %.3f should exceed country %.3f", a.Multiplier(models.Sector), a.Multiplier(models.Country))
	}
}

func TestHighSubstitutesReduceSectorImpact(t *testing.T) {
	high := mustEvaluate(t, Assumptions{DurationDays: 30, RecoverySpeed: "Moderate", SubstituteAvailability: "High", InventoryBufferDays: 30})
	medium := mustEvaluate(t, Assumptions{DurationDays: 30, RecoverySpeed: "Moderate", SubstituteAvailability: "Medium", InventoryBufferDays: 30})
	if high.Multiplier(models.Sector) >= medium.Multiplier(models.Sector) {
		t.Fatalf("high-substitute sector multiplier %.3f should be below medium %.3f", high.Multiplier(models.Sector), medium.Multiplier(models.Sector))
	}
}

func TestLargeInventoryBufferReducesCountryImpact(t *testing.T) {
	large := mustEvaluate(t, Assumptions{DurationDays: 30, RecoverySpeed: "Moderate", SubstituteAvailability: "Medium", InventoryBufferDays: 60})
	low := mustEvaluate(t, Assumptions{DurationDays: 30, RecoverySpeed: "Moderate", SubstituteAvailability: "Medium", InventoryBufferDays: 0})
	if large.Multiplier(models.Country) >= low.Multiplier(models.Country) {
		t.Fatalf("large-buffer country multiplier %.3f should be below low-buffer %.3f", large.Multiplier(models.Country), low.Multiplier(models.Country))
	}
}

func TestSlowRecoveryIncreasesCountryAndSectorImpact(t *testing.T) {
	slow := mustEvaluate(t, Assumptions{DurationDays: 30, RecoverySpeed: "Slow", SubstituteAvailability: "Medium", InventoryBufferDays: 30})
	fast := mustEvaluate(t, Assumptions{DurationDays: 30, RecoverySpeed: "Fast", SubstituteAvailability: "Medium", InventoryBufferDays: 30})
	for _, typ := range []models.NodeType{models.Country, models.Sector} {
		if slow.Multiplier(typ) <= fast.Multiplier(typ) {
			t.Errorf("%s slow multiplier %.3f should exceed fast %.3f", typ, slow.Multiplier(typ), fast.Multiplier(typ))
		}
	}
}
