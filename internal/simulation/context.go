package simulation

import (
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/models"
)

// Context carries optional real-data vulnerability adjustments for shock
// propagation. Nil or zero-valued fields preserve legacy demo behaviour.
type Context struct {
	EventRiskByCountry         map[string]float64
	CommodityStressByName      map[string]float64
	RealTradeFusionActive      bool
	RealEventRiskUsed          bool
	RealPriceStressUsed        bool
	EventRiskMultiplierApplied bool
}

// EventRiskMultiplier amplifies country impact from real event-risk scores.
// Capped at 1.25 per v1 fusion rules.
func EventRiskMultiplier(score float64) float64 {
	m := 1 + (score/100)*0.25
	if m > 1.25 {
		return 1.25
	}
	if m < 1 {
		return 1
	}
	return m
}

// CommodityStressMultiplier amplifies commodity impact from price stress scores.
// Capped at 1.20 per v1 fusion rules.
func CommodityStressMultiplier(score float64) float64 {
	m := 1 + (score/100)*0.20
	if m > 1.20 {
		return 1.20
	}
	if m < 1 {
		return 1
	}
	return m
}

func (c Context) countryMultiplier(name string) float64 {
	if len(c.EventRiskByCountry) == 0 {
		return 1
	}
	score, ok := eventrisk.LookupScore(c.EventRiskByCountry, name)
	if !ok {
		return 1
	}
	return EventRiskMultiplier(score)
}

func (c Context) commodityMultiplier(name string) float64 {
	if len(c.CommodityStressByName) == 0 {
		return 1
	}
	score, ok := c.CommodityStressByName[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return 1
	}
	return CommodityStressMultiplier(score)
}

func (c *Context) applyVulnerability(g interface {
	Node(id models.NodeID) (models.Node, bool)
}, impact map[models.NodeID]float64) {
	if len(c.EventRiskByCountry) == 0 && len(c.CommodityStressByName) == 0 {
		return
	}
	for id, imp := range impact {
		node, ok := g.Node(id)
		if !ok {
			continue
		}
		switch node.Type {
		case models.Country:
			mult := c.countryMultiplier(node.Name)
			if mult > 1 {
				c.EventRiskMultiplierApplied = true
			}
			impact[id] = clamp01(imp * mult)
		case models.Commodity:
			impact[id] = clamp01(imp * c.commodityMultiplier(node.Name))
		}
	}
}
