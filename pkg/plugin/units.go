package plugin

import "math"

// Minor-unit → major-unit normalization for Yahoo-reported currencies.
// Yahoo returns UK equities priced in pence ("GBp"), South African in
// cents ("ZAc"), Israeli in agorot ("ILa"). Downstream `prices` MVs do
// FX conversion using the payload `currency` key, so unconverted minor
// units inflate NAV ~100x. Mirrors services/ingestor-yfinance/main.py
// `_MINOR_UNIT_TO_MAJOR`.
var minorUnitToMajor = map[string]struct {
	Major   string
	Divisor float64
}{
	"GBp": {"GBP", 100.0},
	"GBX": {"GBP", 100.0},
	"ZAc": {"ZAR", 100.0},
	"ILa": {"ILS", 100.0},
}

// normalizeMinorUnits returns (majorCurrency, divisor). When currency is
// already a major unit (or unknown), divisor is 1.0 and the currency is
// returned verbatim (defaulting to "USD" when empty, matching Python).
func normalizeMinorUnits(currency string) (string, float64) {
	if v, ok := minorUnitToMajor[currency]; ok {
		return v.Major, v.Divisor
	}
	if currency == "" {
		return "USD", 1.0
	}
	return currency, 1.0
}

// classifyBarUnit picks between "minor" (divide by divisor) and "major"
// (publish raw, Yahoo already pre-converted) for one bar. Mirrors
// services/ingestor-yfinance/main.py::_classify_unit exactly.
//
// Both hypotheses are scored against the reference price (which is in
// the same unit Yahoo's metadata claims — typically minor when the
// metadata claims minor). The bar is treated as "major" only when the
// major hypothesis fits markedly better (log_dist tighter by at least
// log(2)) — otherwise default to minor so we always divide for legit
// minor-unit tickers.
//
// reference == 0 OR value == 0 OR divisor <= 1 → caller already knows
// the unit; this function returns "minor" so the caller's default
// (always-divide for the minor-unit table) wins.
func classifyBarUnit(value, reference, divisor float64) string {
	if reference <= 0 || value <= 0 || divisor <= 1.0 {
		return "minor"
	}
	logDistMinor := math.Abs(math.Log(value) - math.Log(reference))
	logDistMajor := math.Abs(math.Log(value*divisor) - math.Log(reference))
	if logDistMajor+math.Log(2) < logDistMinor {
		return "major"
	}
	return "minor"
}
