package simulation

import (
	"testing"

	"github.com/atlasgraph/atlas/internal/config"
)

func operationalReq(duration, inventory int, recovery, substitute string) ShockRequest {
	req := baseReq()
	req.OperationalEnabled = true
	req.DurationDays = duration
	req.RecoverySpeed = recovery
	req.SubstituteAvailability = substitute
	req.InventoryBufferDays = inventory
	return req
}

func TestSevereOperationalAssumptionsRaiseTopImpact(t *testing.T) {
	g := seedGraph(t)
	mild, err := Run(g, config.Default(), operationalReq(7, 60, "Fast", "High"))
	if err != nil {
		t.Fatal(err)
	}
	severe, err := Run(g, config.Default(), operationalReq(120, 0, "Slow", "Low"))
	if err != nil {
		t.Fatal(err)
	}
	if len(mild.AllAffected) == 0 || len(severe.AllAffected) == 0 {
		t.Fatal("expected affected entities")
	}
	if severe.AllAffected[0].Delta <= mild.AllAffected[0].Delta {
		t.Fatalf("severe top delta %.3f should exceed mild %.3f", severe.AllAffected[0].Delta, mild.AllAffected[0].Delta)
	}
}

func TestOperationalShockFragilityCappedAt100(t *testing.T) {
	g := seedGraph(t)
	res, err := Run(g, config.Default(), operationalReq(180, 0, "Slow", "Low"))
	if err != nil {
		t.Fatal(err)
	}
	for _, impact := range res.AllAffected {
		if impact.ShockFragility > 100 {
			t.Errorf("%s shock fragility %.3f exceeds 100", impact.Node.Name, impact.ShockFragility)
		}
		if impact.OperationalMultiplier < 0.40 || impact.OperationalMultiplier > 2.25 {
			t.Errorf("%s multiplier %.3f outside cap", impact.Node.Name, impact.OperationalMultiplier)
		}
		if impact.ResilienceNote == "" {
			t.Errorf("%s missing resilience note", impact.Node.Name)
		}
	}
}
