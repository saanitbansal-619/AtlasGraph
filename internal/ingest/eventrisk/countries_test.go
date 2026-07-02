package eventrisk

import (
	"strings"
	"testing"
)

func TestNormalizeCountryAliases(t *testing.T) {
	cases := []struct {
		in       string
		want     string
		wantISO3 string
	}{
		{"USA", "United States", "USA"},
		{"United States of America", "United States", "USA"},
		{"South Korea", "Korea, Rep.", "KOR"},
		{"Korea, Rep.", "Korea, Rep.", "KOR"},
		{"Russian Federation", "Russia", "RUS"},
		{"People's Republic of China", "China", "CHN"},
		{"Islamic Republic of Iran", "Iran", "IRN"},
		{"UAE", "United Arab Emirates", "ARE"},
		{"DRC", "Congo, Dem. Rep.", "COD"},
		{"Ukraine", "Ukraine", "UKR"},
	}
	for _, tc := range cases {
		got, iso, ok := NormalizeCountry(tc.in)
		if !ok {
			t.Fatalf("NormalizeCountry(%q) not recognized", tc.in)
		}
		if got != tc.want {
			t.Errorf("NormalizeCountry(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if iso != tc.wantISO3 {
			t.Errorf("ISO3 for %q = %q, want %q", tc.in, iso, tc.wantISO3)
		}
	}
}

func TestNormalizeCountryUnknown(t *testing.T) {
	if _, _, ok := NormalizeCountry("Unknownistan"); ok {
		t.Fatal("expected unknown country to be rejected")
	}
}

func TestWarnUnknownCountry(t *testing.T) {
	msg := WarnUnknownCountry("Atlantis")
	if !strings.Contains(msg, "Atlantis") {
		t.Fatalf("warning = %q", msg)
	}
}
