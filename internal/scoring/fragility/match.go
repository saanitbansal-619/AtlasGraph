package fragility

import (
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/scoring/commodities"
	"github.com/atlasgraph/atlas/internal/scoring/events"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

func normalizeCommodityCode(code string) string {
	return normalizeCommodityKey(code)
}

func guessCode(name string, macroByKey map[string]macro.CountryScore, eventByKey map[string]events.CountryScore) string {
	ref := countryRef{name: name}
	for _, s := range macroByKey {
		if countryRefsMatch(ref, countryRef{code: s.CountryCode, name: s.CountryName}) {
			return s.CountryCode
		}
	}
	for _, s := range eventByKey {
		if countryRefsMatch(ref, countryRef{code: s.CountryCode, name: s.CountryName}) {
			return s.CountryCode
		}
	}
	if code, ok := isoCodeForName(name); ok {
		return code
	}
	return ""
}

func guessCommodityCode(name string, stress map[string]commodities.CommodityScore) string {
	for _, s := range stress {
		if commodityRefsMatch(commodityRef{name: name}, commodityRef{code: s.CommodityCode, name: s.CommodityName}) {
			return commodityKey(s.CommodityCode, s.CommodityName)
		}
	}
	return commodityKey("", name)
}

func isoCodeForName(name string) (string, bool) {
	// Walk the GDELT country map to bridge graph display names to ISO3 codes.
	for _, code := range knownCountryCodes() {
		if n, ok := gdelt.CountryName(code); ok && strings.EqualFold(n, name) {
			return code, true
		}
	}
	return "", false
}

func knownCountryCodes() []string {
	return []string{
		"TWN", "CHN", "USA", "DEU", "JPN", "KOR", "SAU", "COD", "GBR", "FRA",
		"IND", "BRA", "AUS", "CAN", "MEX", "ITA", "ESP", "NLD", "SGP", "MYS",
	}
}

func lookupMacro(ref countryRef, byKey map[string]macro.CountryScore) (float64, bool) {
	a, aok := lookupMacroByCode(ref, byKey)
	b, bok := lookupMacroByName(ref, byKey)
	return pickScore(sc(a, aok), sc(b, bok))
}

func lookupMacroByCode(ref countryRef, byKey map[string]macro.CountryScore) (float64, bool) {
	code := canonicalCountryCode(firstNonEmpty(ref.code, countryKey(ref.code, ref.name)))
	if code == "" {
		return 0, false
	}
	if s, ok := byKey[code]; ok {
		return s.Score, true
	}
	return 0, false
}

func lookupMacroByName(ref countryRef, byKey map[string]macro.CountryScore) (float64, bool) {
	for _, s := range byKey {
		if countryRefsMatch(ref, countryRef{code: s.CountryCode, name: s.CountryName}) {
			return s.Score, true
		}
	}
	return 0, false
}

func lookupEvent(ref countryRef, byKey map[string]events.CountryScore) (float64, bool) {
	a, aok := lookupEventByCode(ref, byKey)
	b, bok := lookupEventByName(ref, byKey)
	return pickScore(sc(a, aok), sc(b, bok))
}

func lookupEventByCode(ref countryRef, byKey map[string]events.CountryScore) (float64, bool) {
	code := canonicalCountryCode(firstNonEmpty(ref.code, countryKey(ref.code, ref.name)))
	if code == "" {
		return 0, false
	}
	if s, ok := byKey[code]; ok {
		return s.Score, true
	}
	return 0, false
}

func lookupEventByName(ref countryRef, byKey map[string]events.CountryScore) (float64, bool) {
	for _, s := range byKey {
		if countryRefsMatch(ref, countryRef{code: s.CountryCode, name: s.CountryName}) {
			return s.Score, true
		}
	}
	return 0, false
}

func lookupTradeConcentration(ref countryRef, byKey map[string]float64) (float64, bool) {
	code := canonicalCountryCode(firstNonEmpty(ref.code, countryKey(ref.code, ref.name)))
	if code != "" {
		if v, ok := byKey[code]; ok {
			return v, true
		}
	}
	for k, v := range byKey {
		if countryRefsMatch(ref, countryRef{code: k, name: importerDisplayName(k)}) {
			return v, true
		}
	}
	return 0, false
}

func lookupShockExposure(ref countryRef, byKey map[string]float64) float64 {
	best := 0.0
	for k, v := range byKey {
		if countryRefsMatch(ref, countryRef{name: k}) && v > best {
			best = v
		}
	}
	return best
}

func lookupCommodityStress(ref commodityRef, byKey map[string]commodities.CommodityScore) (float64, bool) {
	a, aok := lookupCommodityStressByKey(ref, byKey)
	b, bok := lookupCommodityStressByName(ref, byKey)
	return pickScore(sc(a, aok), sc(b, bok))
}

func lookupCommodityStressByKey(ref commodityRef, byKey map[string]commodities.CommodityScore) (float64, bool) {
	key := commodityKey(ref.code, ref.name)
	if key == "" {
		return 0, false
	}
	if s, ok := byKey[key]; ok {
		return s.Score, true
	}
	return 0, false
}

func lookupCommodityStressByName(ref commodityRef, byKey map[string]commodities.CommodityScore) (float64, bool) {
	for _, s := range byKey {
		if commodityRefsMatch(ref, commodityRef{code: s.CommodityCode, name: s.CommodityName}) {
			return s.Score, true
		}
	}
	return 0, false
}

func lookupSupplierConcentration(ref commodityRef, byKey map[string]float64) (float64, bool) {
	key := commodityKey(ref.code, ref.name)
	if key != "" {
		if v, ok := byKey[key]; ok {
			return v, true
		}
	}
	for k, v := range byKey {
		if commodityRefsMatch(ref, commodityRef{code: k, name: k}) {
			return v, true
		}
	}
	return 0, false
}

func lookupEventExposure(ref commodityRef, byKey map[string]float64) (float64, bool) {
	key := commodityKey(ref.code, ref.name)
	if key != "" {
		if v, ok := byKey[key]; ok {
			return v, true
		}
	}
	for k, v := range byKey {
		if commodityRefsMatch(ref, commodityRef{code: k, name: k}) {
			return v, true
		}
	}
	return 0, false
}

func lookupCentrality(ref commodityRef, byName map[string]float64) (float64, bool) {
	best := 0.0
	found := false
	for k, v := range byName {
		if commodityRefsMatch(ref, commodityRef{name: k}) {
			if !found || v > best {
				best, found = v, true
			}
		}
	}
	return best, found
}

type scoreCandidate struct {
	score float64
	ok    bool
}

func pickScore(candidates ...scoreCandidate) (float64, bool) {
	var score float64
	var found bool
	for _, c := range candidates {
		if !c.ok {
			continue
		}
		if !found || (score == 0 && c.score > 0) || c.score > score {
			score, found = c.score, true
		}
	}
	return score, found
}

func sc(score float64, ok bool) scoreCandidate {
	return scoreCandidate{score: score, ok: ok}
}

func importerDisplayName(code string) string {
	if n, ok := gdelt.CountryName(code); ok {
		return n
	}
	return code
}

func namesMatch(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// tradeConcentrationByImporter returns the average supplier HHI (scaled to
// 0..100) across every commodity an importer purchases.
func tradeConcentrationByImporter(file *trade.TradeFile) map[string]float64 {
	out := map[string]float64{}
	if file == nil {
		return out
	}
	importers := map[string]map[string]struct{}{}
	for _, r := range file.Records {
		code := strings.ToUpper(strings.TrimSpace(r.ImporterCode))
		if code == "" {
			continue
		}
		com := strings.TrimSpace(r.CommodityName)
		if com == "" {
			com = strings.TrimSpace(r.CommodityCode)
		}
		if importers[code] == nil {
			importers[code] = map[string]struct{}{}
		}
		importers[code][com] = struct{}{}
	}
	for code, commodities := range importers {
		var sum float64
		var n int
		for com := range commodities {
			con := trade.BuildConcentration(*file, code, com)
			if !con.HasData {
				continue
			}
			sum += hhiToScore(con.HHI)
			n++
		}
		if n > 0 {
			out[code] = sum / float64(n)
		}
	}
	return out
}

// supplierConcentrationByCommodity returns the average importer-side HHI
// (scaled to 0..100) for each commodity across all importing countries.
func supplierConcentrationByCommodity(file *trade.TradeFile) map[string]float64 {
	out := map[string]float64{}
	if file == nil {
		return out
	}
	commodities := map[string]map[string]struct{}{}
	for _, r := range file.Records {
		com := strings.TrimSpace(r.CommodityName)
		if com == "" {
			com = strings.TrimSpace(r.CommodityCode)
		}
		code := strings.ToUpper(strings.TrimSpace(r.ImporterCode))
		if com == "" || code == "" {
			continue
		}
		key := commodityKey("", com)
		if commodities[key] == nil {
			commodities[key] = map[string]struct{}{}
		}
		commodities[key][code] = struct{}{}
	}
	for key, importers := range commodities {
		var sum float64
		var n int
		for code := range importers {
			comName := commodityDisplayName(*file, key)
			con := trade.BuildConcentration(*file, code, comName)
			if !con.HasData {
				continue
			}
			sum += hhiToScore(con.HHI)
			n++
		}
		if n > 0 {
			out[key] = sum / float64(n)
		}
	}
	return out
}

func commodityDisplayName(file trade.TradeFile, key string) string {
	for _, r := range file.Records {
		if commodityKey(r.CommodityCode, r.CommodityName) == key || commodityKey("", r.CommodityName) == key {
			if r.CommodityName != "" {
				return r.CommodityName
			}
			return r.CommodityCode
		}
	}
	return key
}

// eventExposureByCommodity averages event-risk scores of exporter countries
// linked to each commodity in the trade panel.
func eventExposureByCommodity(file *trade.TradeFile, eventByKey map[string]events.CountryScore) map[string]float64 {
	out := map[string]float64{}
	if file == nil || len(eventByKey) == 0 {
		return out
	}
	exporters := map[string]map[string]struct{}{}
	for _, r := range file.Records {
		com := strings.TrimSpace(r.CommodityName)
		if com == "" {
			com = strings.TrimSpace(r.CommodityCode)
		}
		code := strings.ToUpper(strings.TrimSpace(r.ExporterCode))
		if com == "" || code == "" {
			continue
		}
		key := commodityKey("", com)
		if exporters[key] == nil {
			exporters[key] = map[string]struct{}{}
		}
		exporters[key][code] = struct{}{}
	}
	for key, codes := range exporters {
		var sum float64
		var n int
		for code := range codes {
			if s, ok := eventByKey[code]; ok {
				sum += s.Score
				n++
			}
		}
		if n > 0 {
			out[key] = sum / float64(n)
		}
	}
	return out
}

// hhiToScore maps a Herfindahl index in [0,1] onto a 0..100 concentration score.
func hhiToScore(hhi float64) float64 {
	if hhi < 0 {
		return 0
	}
	if hhi > 1 {
		return 100
	}
	return hhi * 100
}
