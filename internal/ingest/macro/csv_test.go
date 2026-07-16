package macroingest_test

import (
	"os"
	"path/filepath"
	"testing"

	macroingest "github.com/atlasgraph/atlas/internal/ingest/macro"
)

func TestLoadRawCSV(t *testing.T) {
	dir := t.TempDir()
	csv := `country_code,country_name,indicator_code,year,value
USA,United States,NY.GDP.MKTP.CD,2023,27000000000000
USA,United States,FP.CPI.TOTL.ZG,2023,3.2
USA,United States,NE.TRD.GNFS.ZS,2023,25.5
USA,United States,TM.VAL.FUEL.ZS.UN,2023,12.0
USA,United States,FI.RES.TOTL.CD,2023,700000000000
`
	if err := os.WriteFile(filepath.Join(dir, "usa.csv"), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := macroingest.LoadRaw(dir)
	if err != nil {
		t.Fatalf("LoadRaw: %v", err)
	}
	if len(file.Records) != 5 {
		t.Fatalf("records = %d, want 5", len(file.Records))
	}
}

func TestLoadRawMissingDir(t *testing.T) {
	_, err := macroingest.LoadRaw(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error for empty/missing raw dir")
	}
}
