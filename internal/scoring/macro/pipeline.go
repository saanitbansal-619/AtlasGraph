package macro

import (
	"math"
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

// Pipeline indicator codes for the processed macro-risk panel.
const (
	codePipelineTrade = "NE.TRD.GNFS.ZS"
	codeFuelImports   = "TM.VAL.FUEL.ZS.UN"
	codeReserves      = "FI.RES.TOTL.CD"
)

var pipelineIndicatorCodes = []string{
	codeGDP,
	codeInflation,
	codePipelineTrade,
	codeManuf,
	codeFuelImports,
	codeReserves,
}

const (
	fuelLo, fuelHi               = 5.0, 45.0
	reservesLogLo, reservesLogHi = 8.5, 12.5
)

type pipelineWeights struct {
	inflation float64
	trade     float64
	fuel      float64
	reserves  float64
	buffer    float64
}

func defaultPipelineWeights() pipelineWeights {
	return pipelineWeights{
		inflation: 0.25,
		trade:     0.20,
		fuel:      0.20,
		reserves:  0.20,
		buffer:    0.15,
	}
}

// ScorePipelineCountries computes processed macro-risk scores from World Bank
// indicators. targetYear <= 0 uses the latest available year per country.
// requestedCountries ensures every requested code appears in the output even
// when the World Bank API returned no data (e.g. Taiwan/TWN).
func ScorePipelineCountries(file worldbank.IndicatorFile, targetYear int, requestedCountries []string) ProcessedScoreFile {
	type group struct {
		name string
		recs []worldbank.CountryIndicatorRecord
	}
	byCountry := map[string]*group{}
	for _, r := range file.Records {
		g, ok := byCountry[r.CountryCode]
		if !ok {
			g = &group{}
			byCountry[r.CountryCode] = g
		}
		if g.name == "" && r.CountryName != "" {
			g.name = r.CountryName
		}
		g.recs = append(g.recs, r)
	}

	order := append([]string{}, requestedCountries...)
	if len(order) == 0 {
		for code := range byCountry {
			order = append(order, code)
		}
		sort.Strings(order)
	}

	scores := make([]ProcessedCountryScore, 0, len(order))
	for _, code := range order {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		g := byCountry[code]
		if g == nil {
			g = &group{name: countryNameFromRecords(file, code)}
		}
		scores = append(scores, scorePipelineCountry(code, g.name, g.recs, targetYear))
	}
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].MacroExposureScore != scores[j].MacroExposureScore {
			return scores[i].MacroExposureScore > scores[j].MacroExposureScore
		}
		return scores[i].CountryName < scores[j].CountryName
	})

	countries := append([]string{}, requestedCountries...)
	if len(countries) == 0 {
		countries = append(countries, file.Countries...)
	}

	return ProcessedScoreFile{
		Source:      ProcessedSourceName,
		GeneratedAt: file.FetchedAt,
		StartYear:   file.StartYear,
		EndYear:     file.EndYear,
		Countries:   countries,
		Scores:      scores,
	}
}

func countryNameFromRecords(file worldbank.IndicatorFile, code string) string {
	for _, r := range file.Records {
		if r.CountryCode == code && r.CountryName != "" {
			return r.CountryName
		}
	}
	return ""
}

func scorePipelineCountry(code, name string, recs []worldbank.CountryIndicatorRecord, target int) ProcessedCountryScore {
	gdp, gdpY, gdpOK := pickValue(recs, codeGDP, target)
	infl, inflY, inflOK := pickValue(recs, codeInflation, target)
	trade, tradeY, tradeOK := pickValue(recs, codePipelineTrade, target)
	manuf, manY, manOK := pickValue(recs, codeManuf, target)
	fuel, fuelY, fuelOK := pickValue(recs, codeFuelImports, target)
	res, resY, resOK := pickValue(recs, codeReserves, target)

	inflScore := inflationStressScore(infl)
	tradeScore := tradeExposureScore(trade)
	fuelScore := fuelImportDependenceScore(fuel)
	resRisk := reservesRiskScore(res)
	bufRisk := economicBufferRiskScore(gdp)

	w := defaultPipelineWeights()
	exposureNum, exposureDen := 0.0, 0.0
	addExposure := func(weight, score float64, ok bool) {
		if ok {
			exposureNum += weight * score
			exposureDen += weight
		}
	}
	addExposure(w.inflation, inflScore, inflOK)
	addExposure(w.trade, tradeScore, tradeOK)
	addExposure(w.fuel, fuelScore, fuelOK)
	addExposure(w.reserves, resRisk, resOK)
	addExposure(w.buffer, bufRisk, gdpOK)

	macroExposure := 0.0
	if exposureDen > 0 {
		macroExposure = exposureNum / exposureDen
	}

	resilience := blendAvailable(
		[]bool{gdpOK, resOK},
		[]float64{economicResilienceFromGDP(gdp), economicResilienceFromReserves(res)},
	)

	importDep := blendAvailable(
		[]bool{tradeOK, fuelOK},
		[]float64{tradeScore, fuelScore},
	)

	comps := []ProcessedComponent{
		makeProcessedComponent("inflation_stress", "inflation stress", inflScore, inflY, inflOK),
		makeProcessedComponent("trade_exposure", "trade exposure", tradeScore, tradeY, tradeOK),
		makeProcessedComponent("fuel_import_dependence", "fuel import dependence", fuelScore, fuelY, fuelOK),
		makeProcessedComponent("low_reserves", "low reserves", resRisk, resY, resOK),
		makeProcessedComponent("economic_buffer_risk", "low economic buffer", bufRisk, gdpY, gdpOK),
	}
	if manOK {
		comps = append(comps, makeProcessedComponent("manufacturing_dependency", "manufacturing dependency",
			manufacturingDependencyScore(manuf), manY, true))
	}

	year := target
	if year <= 0 {
		for _, c := range comps {
			if c.Available && c.YearUsed > year {
				year = c.YearUsed
			}
		}
	}

	return ProcessedCountryScore{
		CountryCode:             code,
		CountryName:             name,
		Year:                    year,
		MacroExposureScore:      round1(macroExposure),
		EconomicResilienceScore: round1(resilience),
		ImportDependencyScore:   round1(importDep),
		Components:              comps,
		MissingIndicators:       missingPipelineIndicators(recs, target),
		Source:                  ProcessedSourceName,
	}
}

func missingPipelineIndicators(recs []worldbank.CountryIndicatorRecord, target int) []string {
	missing := make([]string, 0, len(pipelineIndicatorCodes))
	for _, code := range pipelineIndicatorCodes {
		_, _, ok := pickValue(recs, code, target)
		if !ok {
			missing = append(missing, code)
		}
	}
	return missing
}

func makeProcessedComponent(key, name string, score float64, year int, available bool) ProcessedComponent {
	c := ProcessedComponent{Key: key, Name: name, YearUsed: year, Available: available}
	if available {
		c.Score = round1(score)
	}
	return c
}

func fuelImportDependenceScore(fuelPct float64) float64 {
	return scaleLinear(fuelPct, fuelLo, fuelHi)
}

func reservesRiskScore(reservesUSD float64) float64 {
	if reservesUSD <= 0 {
		return 100
	}
	buffer := scaleLinear(math.Log10(reservesUSD), reservesLogLo, reservesLogHi)
	return 100 - buffer
}

func economicResilienceFromGDP(gdpUSD float64) float64 {
	if gdpUSD <= 0 {
		return 0
	}
	return scaleLinear(math.Log10(gdpUSD), gdpLogLo, gdpLogHi)
}

func economicResilienceFromReserves(reservesUSD float64) float64 {
	if reservesUSD <= 0 {
		return 0
	}
	return scaleLinear(math.Log10(reservesUSD), reservesLogLo, reservesLogHi)
}

func blendAvailable(avail []bool, scores []float64) float64 {
	num, den := 0.0, 0.0
	for i := range avail {
		if avail[i] {
			num += scores[i]
			den++
		}
	}
	if den == 0 {
		return 0
	}
	return num / den
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
