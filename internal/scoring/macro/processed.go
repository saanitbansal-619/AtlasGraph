package macro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProcessedSourceName is recorded on processed macro score files.
const ProcessedSourceName = "World Bank"

// ProcessedOutputFileName is the canonical processed macro scores file.
const ProcessedOutputFileName = "macro_scores.json"

// ProcessedComponent is one scored macro input for a country.
type ProcessedComponent struct {
	Key       string  `json:"key"`
	Name      string  `json:"name"`
	Score     float64 `json:"score"`
	Available bool    `json:"available"`
	YearUsed  int     `json:"year_used,omitempty"`
}

// ProcessedCountryScore is the processed macro-risk record for one country.
type ProcessedCountryScore struct {
	CountryCode             string               `json:"country_code"`
	CountryName             string               `json:"country_name"`
	Year                    int                  `json:"year"`
	MacroExposureScore      float64              `json:"macro_exposure_score"`
	EconomicResilienceScore float64              `json:"economic_resilience_score"`
	ImportDependencyScore   float64              `json:"import_dependency_score"`
	Components              []ProcessedComponent `json:"components"`
	MissingIndicators       []string             `json:"missing_indicators"`
	Source                  string               `json:"source"`
}

// ProcessedScoreFile is the on-disk processed macro panel.
type ProcessedScoreFile struct {
	Source      string                  `json:"source"`
	GeneratedAt time.Time               `json:"generated_at"`
	StartYear   int                     `json:"start_year,omitempty"`
	EndYear     int                     `json:"end_year,omitempty"`
	Countries   []string                `json:"countries,omitempty"`
	Scores      []ProcessedCountryScore `json:"scores"`
}

// SaveProcessed writes macro_scores.json under dir.
func SaveProcessed(dir string, file ProcessedScoreFile) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", dir, err)
	}
	path := filepath.Join(dir, ProcessedOutputFileName)
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding macro scores: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("writing %q: %w", path, err)
	}
	return path, nil
}

// LoadProcessed reads macro_scores.json from dir.
func LoadProcessed(dir string) (ProcessedScoreFile, error) {
	path := filepath.Join(dir, ProcessedOutputFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return ProcessedScoreFile{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var file ProcessedScoreFile
	if err := json.Unmarshal(b, &file); err != nil {
		return ProcessedScoreFile{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return file, nil
}

// TryLoadProcessed returns processed scores when the file exists and is valid.
func TryLoadProcessed(dir string) (ProcessedScoreFile, bool) {
	if strings.TrimSpace(dir) == "" {
		return ProcessedScoreFile{}, false
	}
	file, err := LoadProcessed(dir)
	if err != nil || len(file.Scores) == 0 {
		return ProcessedScoreFile{}, false
	}
	return file, true
}

// ToCountryScores adapts processed scores for legacy macro.CountryScore consumers.
func (f ProcessedScoreFile) ToCountryScores() []CountryScore {
	out := make([]CountryScore, 0, len(f.Scores))
	for _, s := range f.Scores {
		comps := make([]Component, 0, len(s.Components))
		for _, c := range s.Components {
			comps = append(comps, Component{
				Key:       c.Key,
				Name:      c.Name,
				Score:     c.Score,
				YearUsed:  c.YearUsed,
				Available: c.Available,
			})
		}
		out = append(out, CountryScore{
			CountryCode: s.CountryCode,
			CountryName: s.CountryName,
			Year:        s.Year,
			Score:       s.MacroExposureScore,
			RiskLevel:   RiskLevel(s.MacroExposureScore),
			Components:  comps,
			TopDrivers:  topDrivers(comps, 2),
		})
	}
	return out
}
