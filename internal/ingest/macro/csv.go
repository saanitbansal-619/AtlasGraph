package macroingest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

// LoadRaw reads macro indicators from a raw directory. It prefers
// worldbank_indicators.json when present, otherwise merges all *.csv files.
func LoadRaw(dir string) (worldbank.IndicatorFile, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return worldbank.IndicatorFile{}, fmt.Errorf("raw directory is required")
	}

	jsonPath := filepath.Join(dir, worldbank.OutputFileName)
	if _, err := os.Stat(jsonPath); err == nil {
		return worldbank.Load(dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return worldbank.IndicatorFile{}, fmt.Errorf("reading raw directory %q: %w", dir, err)
	}

	var records []worldbank.CountryIndicatorRecord
	var countries []string
	seenCountry := map[string]bool{}
	for _, ent := range entries {
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(ent.Name()), ".csv") {
			continue
		}
		part, codes, err := loadCSVFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			return worldbank.IndicatorFile{}, err
		}
		records = append(records, part...)
		for _, c := range codes {
			if !seenCountry[c] {
				seenCountry[c] = true
				countries = append(countries, c)
			}
		}
	}
	if len(records) == 0 {
		return worldbank.IndicatorFile{}, fmt.Errorf("no macro indicators found in %q (expected %s or CSV files)", dir, worldbank.OutputFileName)
	}
	worldbank.SortRecords(records)
	return worldbank.IndicatorFile{
		Source:    "World Bank (CSV)",
		FetchedAt: time.Now().UTC(),
		Countries: countries,
		Records:   records,
	}, nil
}

func loadCSVFile(path string) ([]worldbank.CountryIndicatorRecord, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("reading %q: %w", path, err)
	}
	if len(rows) < 2 {
		return nil, nil, fmt.Errorf("%s: no data rows", path)
	}

	header := map[string]int{}
	for i, h := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(h))] = i
	}
	idx := func(names ...string) (int, bool) {
		for _, n := range names {
			if i, ok := header[n]; ok {
				return i, true
			}
		}
		return 0, false
	}

	codeIdx, ok := idx("country_code", "countryiso3code", "iso3")
	if !ok {
		return nil, nil, fmt.Errorf("%s: missing country_code column", path)
	}
	indIdx, ok := idx("indicator_code", "indicator")
	if !ok {
		return nil, nil, fmt.Errorf("%s: missing indicator_code column", path)
	}
	yearIdx, ok := idx("year", "date")
	if !ok {
		return nil, nil, fmt.Errorf("%s: missing year column", path)
	}
	valIdx, ok := idx("value")
	if !ok {
		return nil, nil, fmt.Errorf("%s: missing value column", path)
	}
	nameIdx, _ := idx("country_name", "country")

	now := time.Now().UTC()
	var records []worldbank.CountryIndicatorRecord
	var countries []string
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}
		get := func(i int) string {
			if i < 0 || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		code := strings.ToUpper(get(codeIdx))
		ind := get(indIdx)
		if code == "" || ind == "" {
			continue
		}
		year, err := strconv.Atoi(get(yearIdx))
		if err != nil {
			continue
		}
		var valPtr *float64
		if raw := get(valIdx); raw != "" && !strings.EqualFold(raw, "null") {
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				continue
			}
			valPtr = &v
		}
		name := get(nameIdx)
		records = append(records, worldbank.CountryIndicatorRecord{
			CountryCode:   code,
			CountryName:   name,
			IndicatorCode: ind,
			IndicatorName: worldbank.IndicatorName(ind),
			Year:          year,
			Value:         valPtr,
			Source:        "World Bank (CSV)",
			FetchedAt:     now,
		})
		if !seen[code] {
			seen[code] = true
			countries = append(countries, code)
		}
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("%s: no valid indicator rows", path)
	}
	return records, countries, nil
}
