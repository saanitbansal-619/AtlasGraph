package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

func TestIngestMacroFromCSV(t *testing.T) {
	rawDir := t.TempDir()
	csv := `country_code,country_name,indicator_code,year,value
USA,United States,NY.GDP.MKTP.CD,2023,27000000000000
USA,United States,FP.CPI.TOTL.ZG,2023,3.2
USA,United States,NE.TRD.GNFS.ZS,2023,25.5
USA,United States,NV.IND.MANF.ZS,2023,11.0
USA,United States,TM.VAL.FUEL.ZS.UN,2023,12.0
USA,United States,FI.RES.TOTL.CD,2023,700000000000
DEU,Germany,NY.GDP.MKTP.CD,2023,4500000000000
DEU,Germany,FP.CPI.TOTL.ZG,2023,5.9
DEU,Germany,NE.TRD.GNFS.ZS,2023,88.0
DEU,Germany,TM.VAL.FUEL.ZS.UN,2023,18.0
DEU,Germany,FI.RES.TOTL.CD,2023,300000000000
`
	if err := os.WriteFile(filepath.Join(rawDir, "macro.csv"), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()

	out, errOut, code := run("ingest", "macro", "--source", "csv", "--raw", rawDir, "--out", outDir)
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	if !strings.Contains(out, "macro_scores.json") {
		t.Errorf("output missing scores path:\n%s", out)
	}

	loaded, err := macro.LoadProcessed(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Scores) < 2 {
		t.Fatalf("scores = %d, want >= 2", len(loaded.Scores))
	}
}
