package graphfusion

import (
	"strings"

	"github.com/atlasgraph/atlas/internal/simulation"
)

// PropagationNote builds a compact human-readable fusion summary for shock results.
func PropagationNote(meta Meta, simCtx simulation.Context) string {
	var parts []string
	if meta.RealTradeEdgesUsed || simCtx.RealTradeFusionActive {
		parts = append(parts, "trade")
	}
	if meta.RealPriceStressUsed || simCtx.RealPriceStressUsed {
		parts = append(parts, "commodity prices")
	}
	if meta.RealEventRiskUsed || simCtx.RealEventRiskUsed {
		parts = append(parts, "event risk")
	}
	if len(parts) == 0 {
		return ""
	}
    return "Real-data-backed model propagation: " + strings.Join(parts, " + ")
}
