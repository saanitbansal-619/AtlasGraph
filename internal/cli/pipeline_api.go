package cli

import (
	"net/http"

	"github.com/atlasgraph/atlas/internal/pipeline"
)

func (s *apiServer) handlePipelineSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	summary, err := pipeline.Compute(r.Context(), pipeline.Config{
		TradeData:          s.cfg.TradeData,
		ProcessedMacroData: s.cfg.ProcessedMacroData,
		MacroData:          s.cfg.MacroData,
		ProcessedEventData: s.cfg.ProcessedEventData,
		EventData:          s.cfg.EventData,
		CommodityData:      s.cfg.CommodityData,
		GraphData:          s.cfg.GraphData,
	}, s.db)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"verify processed data paths and PostgreSQL connectivity")
		return
	}
	writeJSONStatus(w, http.StatusOK, summary)
}
