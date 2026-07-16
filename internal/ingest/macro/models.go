// Package macroingest fetches or loads World Bank macro indicators and writes
// processed macro-risk scores for GFIP / AtlasGraph.
package macroingest

import (
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

// RawDirName is the default folder for downloaded or hand-placed macro CSV/JSON.
const RawDirName = "data/raw/worldbank_macro"

// DefaultCountries is the strategic panel ingested by `atlas ingest macro`.
var DefaultCountries = []string{
	"USA", "CHN", "DEU", "JPN", "KOR", "IND", "TWN", "RUS", "UKR", "SAU", "COD", "CHL",
}

// PipelineIndicators are the World Bank series used for macro exposure scoring.
var PipelineIndicators = []worldbank.Indicator{
	{Code: "NY.GDP.MKTP.CD", Name: "GDP (current US$)"},
	{Code: "FP.CPI.TOTL.ZG", Name: "Inflation, consumer prices (annual %)"},
	{Code: "NE.TRD.GNFS.ZS", Name: "Trade (% of GDP)"},
	{Code: "NV.IND.MANF.ZS", Name: "Manufacturing value added (% of GDP)"},
	{Code: "TM.VAL.FUEL.ZS.UN", Name: "Fuel imports (% of merchandise imports)"},
	{Code: "FI.RES.TOTL.CD", Name: "Total reserves (current US$)"},
}
