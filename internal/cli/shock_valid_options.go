package cli

import (
	"net/http"
	"strings"

	"github.com/atlasgraph/atlas/internal/shockguide"
)

type jsonValidCommodityOption struct {
	Commodity     string   `json:"commodity"`
	ShockTypes    []string `json:"shock_types"`
	Relationships []string `json:"relationships"`
}

type jsonValidSourceOption struct {
	Source      string                     `json:"source"`
	Type        string                     `json:"type"`
	Commodities []jsonValidCommodityOption `json:"commodities"`
}

type jsonShockValidOptions struct {
	Sources []jsonValidSourceOption `json:"sources"`
}

func buildShockValidOptionsJSON(opts shockguide.ValidOptions) jsonShockValidOptions {
	out := jsonShockValidOptions{Sources: make([]jsonValidSourceOption, 0, len(opts.Sources))}
	for _, s := range opts.Sources {
		entry := jsonValidSourceOption{
			Source:      s.Source,
			Type:        s.Type,
			Commodities: make([]jsonValidCommodityOption, 0, len(s.Commodities)),
		}
		for _, c := range s.Commodities {
			entry.Commodities = append(entry.Commodities, jsonValidCommodityOption{
				Commodity:     c.Commodity,
				ShockTypes:    c.ShockTypes,
				Relationships: c.Relationships,
			})
		}
		out.Sources = append(out.Sources, entry)
	}
	return out
}

func (s *apiServer) handleShockValidOptions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	ds, err := loadDataset(s.cfg.GraphData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"build a graph with `atlas graph build-trade` or pass an existing --data dir")
		return
	}
	filter := strings.TrimSpace(r.URL.Query().Get("source"))
	opts := shockguide.BuildValidOptions(ds.Graph, filter)
	writeJSONStatus(w, http.StatusOK, buildShockValidOptionsJSON(opts))
}
