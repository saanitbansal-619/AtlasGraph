package worldbank

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// normalizeRecords converts raw API observations into normalised records.
// Observations whose year cannot be parsed are skipped (the API occasionally
// emits aggregate rows with non-numeric dates); a null value is preserved.
func normalizeRecords(points []apiPoint, ind Indicator, fetchedAt time.Time) []CountryIndicatorRecord {
	out := make([]CountryIndicatorRecord, 0, len(points))
	for _, p := range points {
		year, err := strconv.Atoi(strings.TrimSpace(p.Date))
		if err != nil {
			continue
		}
		code := p.CountryISO3
		if code == "" {
			code = p.Country.ID
		}
		out = append(out, CountryIndicatorRecord{
			CountryCode:   strings.ToUpper(code),
			CountryName:   p.Country.Value,
			IndicatorCode: ind.Code,
			IndicatorName: ind.Name,
			Year:          year,
			Value:         p.Value,
			Source:        SourceName,
			FetchedAt:     fetchedAt,
		})
	}
	return out
}

// Save writes an IndicatorFile to dir/OutputFileName, creating dir if needed,
// and returns the full path written.
func Save(dir string, file IndicatorFile) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", dir, err)
	}
	path := filepath.Join(dir, OutputFileName)
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding records: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("writing %q: %w", path, err)
	}
	return path, nil
}

// Load reads the IndicatorFile from dir/OutputFileName.
func Load(dir string) (IndicatorFile, error) {
	path := filepath.Join(dir, OutputFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return IndicatorFile{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var file IndicatorFile
	if err := json.Unmarshal(b, &file); err != nil {
		return IndicatorFile{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return file, nil
}

// SortRecords orders records deterministically (country, indicator, year) for
// stable output files and diffs.
func SortRecords(records []CountryIndicatorRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.CountryCode != b.CountryCode {
			return a.CountryCode < b.CountryCode
		}
		if a.IndicatorCode != b.IndicatorCode {
			return a.IndicatorCode < b.IndicatorCode
		}
		return a.Year < b.Year
	})
}
