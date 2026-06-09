package plugin

// classificationField identifies which classification column a source applies
// to. Values match the vendor_meta JSON keys' prefixes.
const (
	fieldSector   = "sector"
	fieldIndustry = "industry"
)

// classification source values stored in vendor_meta.
const (
	sourceYfinance = "yfinance"
	sourceUser     = "user"
)

// setClassificationSource records the origin (yfinance|user) of one
// classification field inside the vendor_meta map, leaving every other key
// untouched. A nil map is treated as empty. The returned map is the same
// instance when non-nil (mutated in place) so callers can marshal it directly.
func setClassificationSource(meta map[string]any, field, source string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	meta[field+"_source"] = source
	return meta
}
