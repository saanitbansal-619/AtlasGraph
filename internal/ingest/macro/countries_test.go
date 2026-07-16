package macroingest_test

import (
	"testing"

	macroingest "github.com/atlasgraph/atlas/internal/ingest/macro"
)

func TestAPICountryCodesExcludesTaiwan(t *testing.T) {
	api, skipped := macroingest.APICountryCodes([]string{"USA", "TWN", "CHN"})
	if len(skipped) != 1 || skipped["TWN"] != "Taiwan" {
		t.Fatalf("skipped = %#v", skipped)
	}
	for _, code := range api {
		if code == "TWN" {
			t.Fatal("TWN should not be sent to the World Bank API")
		}
	}
}
