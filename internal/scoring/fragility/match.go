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
	nameKey := strings.ToLower(strings.TrimSpace(trade.NormalizeImporterQuery(ref.name)))
	if nameKey != "" {
		if v, ok := byKey[nameKey]; ok {
			return v, true
		}
	}
	for k, v := range byKey {
		if countryRefsMatch(ref, countryRef{code: k, name: importerDisplayName(k)}) {
			return v, true
		}
		if namesMatch(ref.name, k) || namesMatch(trade.NormalizeImporterQuery(ref.name), trade.NormalizeImporterQuery(k)) {
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
func tradeConcentrationByImporter(file *trade.TradeFile, deps *trade.DependencyFile) map[string]float64 {
	if deps != nil && len(deps.Dependencies) > 0 {
		return tradeConcentrationFromDeps(deps)
	}
	out := map[string]float64{}
	if file == nil {
		return out
	}
	importers := map[string]map[string]struct{}{}
	for _, r := range file.Records {
		key := importerKey(r.ImporterCode, r.ImporterName)
		if key == "" {
			continue
		}
		com := strings.TrimSpace(r.CommodityName)
		if com == "" {
			com = strings.TrimSpace(r.CommodityCode)
		}
		if com == "" {
			continue
		}
		if importers[key] == nil {
			importers[key] = map[string]struct{}{}
		}
		importers[key][com] = struct{}{}
	}
	for key, commodities := range importers {
		var sum float64
		var n int
		query := concentrationQuery(key)
		for com := range commodities {
			con := trade.BuildConcentration(*file, query, com)
			if !con.HasData {
				continue
			}
			sum += hhiToScore(con.HHI)
			n++
		}
		if n > 0 {
			out[key] = sum / float64(n)
			storeImporterAliases(out, key, out[key])
		}
	}
	return out
}

func tradeConcentrationFromDeps(deps *trade.DependencyFile) map[string]float64 {
	out := map[string]float64{}
	if deps == nil || len(deps.Dependencies) == 0 {
		return out
	}

	// importer -> commodity -> exporter -> trade value (or share fallback)
	byImporter := map[string]map[string]map[string]float64{}
	importerNames := map[string]string{}

	for _, d := range deps.Dependencies {
		ikey := importerKey(trade.CountryCodeForName(d.Importer), d.Importer)
		if ikey == "" {
			continue
		}
		com := strings.TrimSpace(d.Commodity)
		if com == "" {
			continue
		}
		exporter := strings.TrimSpace(d.Exporter)
		if exporter == "" {
			continue
		}
		ekey := exporterGroupKey(trade.CountryCodeForName(exporter), exporter)
		val := d.TradeValueUSD
		if val <= 0 {
			val = d.Share
		}
		if val <= 0 {
			continue
		}
		if byImporter[ikey] == nil {
			byImporter[ikey] = map[string]map[string]float64{}
		}
		if byImporter[ikey][com] == nil {
			byImporter[ikey][com] = map[string]float64{}
		}
		byImporter[ikey][com][ekey] += val
		importerNames[ikey] = d.Importer
	}

	for ikey, commodities := range byImporter {
		var weightedSum, totalWeight float64
		for _, exporters := range commodities {
			var total float64
			for _, v := range exporters {
				total += v
			}
			if total <= 0 {
				continue
			}
			var hhi float64
			for _, v := range exporters {
				s := v / total
				hhi += s * s
			}
			score := hhiToScore(hhi)
			weightedSum += score * total
			totalWeight += total
		}
		if totalWeight <= 0 {
			continue
		}
		avg := weightedSum / totalWeight
		if avg > 100 {
			avg = 100
		}
		out[ikey] = avg
		storeImporterAliases(out, ikey, avg)
		if name := strings.TrimSpace(importerNames[ikey]); name != "" {
			out[strings.ToLower(name)] = avg
			out[strings.ToLower(trade.NormalizeImporterQuery(name))] = avg
			out[strings.ToLower(trade.NormalizeCountryName(name))] = avg
		}
	}
	return out
}

// exporterGroupKey groups exporters by ISO code when known, else normalized name.
func exporterGroupKey(code, name string) string {
	if c := strings.ToUpper(strings.TrimSpace(code)); c != "" {
		return "c:" + c
	}
	n := strings.ToLower(strings.TrimSpace(trade.NormalizeCountryName(name)))
	if n == "" {
		n = strings.ToLower(strings.TrimSpace(name))
	}
	return "n:" + n
}

// supplierConcentrationByCommodity returns the average importer-side HHI
// (scaled to 0..100) for each commodity across all importing countries.
func supplierConcentrationByCommodity(file *trade.TradeFile, deps *trade.DependencyFile) map[string]float64 {
	if deps != nil && len(deps.Dependencies) > 0 {
		return supplierConcentrationFromDeps(deps)
	}
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
		ikey := importerKey(r.ImporterCode, r.ImporterName)
		if com == "" || ikey == "" {
			continue
		}
		key := commodityKey("", com)
		if commodities[key] == nil {
			commodities[key] = map[string]struct{}{}
		}
		commodities[key][ikey] = struct{}{}
	}
	for key, importers := range commodities {
		var sum float64
		var n int
		comName := commodityDisplayName(*file, key)
		for ikey := range importers {
			con := trade.BuildConcentration(*file, concentrationQuery(ikey), comName)
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

func supplierConcentrationFromDeps(deps *trade.DependencyFile) map[string]float64 {
	out := map[string]float64{}
	commodities := map[string]map[string]struct{}{}
	for _, d := range deps.Dependencies {
		com := strings.TrimSpace(d.Commodity)
		ikey := importerKey(trade.CountryCodeForName(d.Importer), d.Importer)
		if com == "" || ikey == "" {
			continue
		}
		key := commodityKey("", com)
		if commodities[key] == nil {
			commodities[key] = map[string]struct{}{}
		}
		commodities[key][ikey] = struct{}{}
	}
	for key, importers := range commodities {
		var sum float64
		var n int
		comName := key
		for _, d := range deps.Dependencies {
			if commodityKey("", d.Commodity) == key && d.Commodity != "" {
				comName = d.Commodity
				break
			}
		}
		for ikey := range importers {
			con := trade.BuildConcentrationResolved(trade.ResolvedTrade{DependencyFile: deps}, concentrationQuery(ikey), comName)
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

func importerKey(code, name string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code != "" {
		return code
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if resolved := trade.CountryCodeForName(name); resolved != "" {
		return resolved
	}
	return strings.ToLower(trade.NormalizeImporterQuery(name))
}

func concentrationQuery(key string) string {
	key = strings.TrimSpace(key)
	if len(key) == 3 && key == strings.ToUpper(key) {
		return key
	}
	return trade.NormalizeImporterQuery(key)
}

func storeImporterAliases(out map[string]float64, key string, score float64) {
	out[key] = score
	if len(key) == 3 && key == strings.ToUpper(key) {
		if n, ok := gdelt.CountryName(key); ok {
			out[strings.ToLower(n)] = score
			out[strings.ToLower(trade.NormalizeImporterQuery(n))] = score
		}
		return
	}
	if code := trade.CountryCodeForName(key); code != "" {
		out[code] = score
	}
}

func tradeConcentrationMeta(file *trade.TradeFile, deps *trade.DependencyFile) (source, note string) {
	importers := map[string]struct{}{}
	real := false
	if deps != nil && len(deps.Dependencies) > 0 {
		if strings.EqualFold(deps.Source, trade.ComtradeRealSourceName) || strings.Contains(strings.ToLower(deps.Source), "comtrade") {
			real = true
		}
		for _, d := range deps.Dependencies {
			key := importerKey(trade.CountryCodeForName(d.Importer), d.Importer)
			if key != "" {
				importers[key] = struct{}{}
			}
		}
	} else if file != nil {
		if strings.Contains(strings.ToLower(file.Source), "comtrade") {
			real = true
		}
		for _, r := range file.Records {
			key := importerKey(r.ImporterCode, r.ImporterName)
			if key != "" {
				importers[key] = struct{}{}
			}
		}
	}
	if len(importers) == 0 {
		return "", ""
	}
	if real {
		source = trade.ComtradeRealSourceName
	} else {
		source = "demo trade"
	}
	onlyUSA := true
	for k := range importers {
		if k != "USA" && !strings.EqualFold(trade.NormalizeImporterQuery(k), "United States") {
			onlyUSA = false
			break
		}
	}
	if onlyUSA {
		note = "US import-based concentration"
	} else if real {
		note = "importer-side concentration from UN Comtrade reporters"
	}
	return source, note
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
		if code == "" {
			code = trade.CountryCodeForName(r.ExporterName)
		}
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
