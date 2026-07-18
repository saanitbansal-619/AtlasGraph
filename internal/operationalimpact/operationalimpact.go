// Package operationalimpact translates operational resilience assumptions into
// entity-specific post-propagation impact adjustments.
package operationalimpact

import (
	"fmt"
	"strings"

	"github.com/atlasgraph/atlas/internal/models"
)

const (
	MinMultiplier = 0.40
	MaxMultiplier = 2.25
)

type Assumptions struct {
	DurationDays           int    `json:"duration_days"`
	RecoverySpeed          string `json:"recovery_speed"`
	SubstituteAvailability string `json:"substitute_availability"`
	InventoryBufferDays    int    `json:"inventory_buffer_days"`
}

type Assessment struct {
	DurationDays           int     `json:"duration_days"`
	RecoverySpeed          string  `json:"recovery_speed"`
	SubstituteAvailability string  `json:"substitute_availability"`
	InventoryBufferDays    int     `json:"inventory_buffer_days"`
	DurationFactor         float64 `json:"duration_factor"`
	RecoveryFactor         float64 `json:"recovery_factor"`
	SubstituteFactor       float64 `json:"substitute_factor"`
	InventoryFactor        float64 `json:"inventory_factor"`
	Explanation            string  `json:"explanation"`
}

func Evaluate(a Assumptions) (Assessment, error) {
	if a.DurationDays < 0 {
		return Assessment{}, fmt.Errorf("duration_days must be >= 0")
	}
	if a.InventoryBufferDays < 0 {
		return Assessment{}, fmt.Errorf("inventory_buffer_days must be >= 0")
	}
	recovery, recoveryFactor, err := recoveryValue(a.RecoverySpeed)
	if err != nil {
		return Assessment{}, err
	}
	substitute, substituteFactor, err := substituteValue(a.SubstituteAvailability)
	if err != nil {
		return Assessment{}, err
	}
	out := Assessment{
		DurationDays:           a.DurationDays,
		RecoverySpeed:          recovery,
		SubstituteAvailability: substitute,
		InventoryBufferDays:    a.InventoryBufferDays,
		DurationFactor:         durationFactor(a.DurationDays),
		RecoveryFactor:         recoveryFactor,
		SubstituteFactor:       substituteFactor,
		InventoryFactor:        inventoryFactor(a.InventoryBufferDays),
	}
	out.Explanation = explanation(out)
	return out, nil
}

func (a Assessment) Multiplier(t models.NodeType) float64 {
	var value float64
	switch t {
	case models.Country:
		value = a.DurationFactor * a.RecoveryFactor * a.InventoryFactor
	case models.Sector:
		value = a.DurationFactor * a.RecoveryFactor * a.SubstituteFactor * a.InventoryFactor
	case models.Commodity:
		value = a.DurationFactor * a.SubstituteFactor
	case models.Route:
		value = a.DurationFactor * a.RecoveryFactor
	default:
		value = a.DurationFactor * a.RecoveryFactor
	}
	return clamp(value, MinMultiplier, MaxMultiplier)
}

func (a Assessment) ResilienceNote(t models.NodeType) string {
	notes := []string{}
	switch t {
	case models.Country:
		if a.InventoryBufferDays >= 60 {
			notes = append(notes, "Large inventory buffer reduces short-term country exposure.")
		} else if a.InventoryBufferDays < 7 {
			notes = append(notes, "Low inventory buffers increase short-term country exposure.")
		}
		if a.RecoverySpeed == "Slow" {
			notes = append(notes, "Slow recovery increases duration-adjusted impact.")
		} else if a.RecoverySpeed == "Fast" {
			notes = append(notes, "Fast recovery reduces country exposure.")
		}
	case models.Sector:
		if a.SubstituteAvailability == "Low" {
			notes = append(notes, "Low substitute availability increases downstream sector exposure.")
		} else if a.SubstituteAvailability == "High" {
			notes = append(notes, "High substitute availability reduces downstream sector exposure.")
		}
		if a.InventoryBufferDays >= 60 {
			notes = append(notes, "Large inventory buffers cushion near-term sector disruption.")
		} else if a.InventoryBufferDays < 7 {
			notes = append(notes, "Low inventory buffers amplify near-term sector disruption.")
		}
		if a.RecoverySpeed == "Slow" {
			notes = append(notes, "Slow recovery increases duration-adjusted impact.")
		}
	case models.Commodity:
		if a.SubstituteAvailability == "Low" {
			notes = append(notes, "Low substitute availability increases commodity exposure.")
		} else if a.SubstituteAvailability == "High" {
			notes = append(notes, "High substitute availability reduces commodity exposure.")
		}
	case models.Route:
		if a.RecoverySpeed == "Slow" {
			notes = append(notes, "Slow recovery extends route disruption.")
		} else if a.RecoverySpeed == "Fast" {
			notes = append(notes, "Fast recovery reduces route disruption.")
		}
	}
	if len(notes) == 0 {
		notes = append(notes, "Operational assumptions produce a balanced adjustment for this entity type.")
	}
	return strings.Join(notes, " ")
}

func durationFactor(days int) float64 {
	switch {
	case days <= 7:
		return 0.70
	case days <= 30:
		return 1.00
	case days <= 90:
		return 1.25
	default:
		return 1.50
	}
}

func inventoryFactor(days int) float64 {
	switch {
	case days >= 60:
		return 0.70
	case days >= 30:
		return 0.90
	case days >= 7:
		return 1.15
	default:
		return 1.35
	}
}

func recoveryValue(raw string) (string, float64, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fast":
		return "Fast", 0.80, nil
	case "moderate":
		return "Moderate", 1.00, nil
	case "slow":
		return "Slow", 1.30, nil
	default:
		return "", 0, fmt.Errorf("recovery_speed must be Fast, Moderate, or Slow")
	}
}

func substituteValue(raw string) (string, float64, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "high":
		return "High", 0.75, nil
	case "medium":
		return "Medium", 1.00, nil
	case "low":
		return "Low", 1.35, nil
	default:
		return "", 0, fmt.Errorf("substitute_availability must be High, Medium, or Low")
	}
}

func explanation(a Assessment) string {
	if a.RecoverySpeed == "Slow" && a.SubstituteAvailability == "Low" && a.InventoryBufferDays < 7 {
		return "Operational assumptions increase modeled exposure because recovery is slow, substitutes are limited, and inventory buffers are low."
	}
	if a.RecoverySpeed == "Fast" && a.SubstituteAvailability == "High" && a.InventoryBufferDays >= 60 {
		return "Operational assumptions reduce modeled exposure because recovery is fast, substitutes are available, and inventory buffers are high."
	}
	return "Operational assumptions adjust baseline graph exposure by entity type: sectors are most sensitive to substitutes and inventory, while countries are most sensitive to recovery and inventory."
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
