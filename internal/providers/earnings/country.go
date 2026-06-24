package earnings

import (
	"strings"

	"github.com/biter777/countries"
)

// specialCodes maps country names to special codes that aren't ISO country codes.
// These are used for economic regions like "Euro Area" that are commonly used in financial data.
var specialCodes = map[string]string{
	"Euro Area":          "EU",
	"European Union":     "EU",
	"Eurozone":           "EU",
	"Euro Zone":          "EU",
	"EU":                 "EU",
}

// nameToCountry maps common country names to their CountryCode.
// This is a static lookup table for reliable name-to-code conversion.
var nameToCountry = map[string]countries.CountryCode{
	// Direct country constants from the library
	"United States":        countries.US,
	"United Kingdom":       countries.GB,
	"Japan":                countries.JP,
	"Germany":              countries.DE,
	"France":               countries.FR,
	"Italy":                countries.IT,
	"Spain":                countries.ES,
	"Canada":               countries.CA,
	"Australia":            countries.AU,
	"Austria":              countries.AT,
	"Belgium":              countries.BE,
	"Brazil":               countries.BR,
	"China":                countries.CN,
	"Switzerland":          countries.CH,
	"Hong Kong":            countries.HK,
	"Hungary":              countries.HU,
	"Indonesia":            countries.ID,
	"India":                countries.IN,
	"Ireland":              countries.IE,
	"Israel":               countries.IL,
	"South Korea":          countries.KR,
	"Mexico":               countries.MX,
	"Malaysia":             countries.MY,
	"Netherlands":          countries.NL,
	"Norway":               countries.NO,
	"New Zealand":          countries.NZ,
	"Philippines":          countries.PH,
	"Poland":               countries.PL,
	"Portugal":             countries.PT,
	"Russia":               countries.RU,
	"Saudi Arabia":         countries.SA,
	"Sweden":               countries.SE,
	"Singapore":            countries.SG,
	"Thailand":             countries.TH,
	"Turkey":               countries.TR,
	"Taiwan":               countries.TW,
	"South Africa":         countries.ZA,
	"Greece":               countries.GR,
	"Finland":              countries.FI,
	"Czech Republic":       countries.CZ,
	"Denmark":              countries.DK,
	"Romania":              countries.RO,
	"Vietnam":              countries.VN,
	"Argentina":            countries.AR,
	"Chile":                countries.CL,
	"Colombia":             countries.CO,
	"Peru":                 countries.PE,
	"Ukraine":              countries.UA,
	"United Arab Emirates": countries.AE,
	"Bulgaria":             countries.BG,
	"Pakistan":             countries.PK,
	"Egypt":                countries.EG,
	"Nigeria":              countries.NG,
	"Czechia":              countries.CZ,
	"Norges":               countries.NO,
	"Sverige":              countries.SE,
	"Polska":               countries.PL,

	// Common variations and abbreviations
	"USA":                 countries.US,
	"US":                  countries.US,
	"U.S.":               countries.US,
	"U.S.A.":             countries.US,
	"America":             countries.US,
	"UK":                  countries.GB,
	"Great Britain":       countries.GB,
	"Britain":             countries.GB,
	"England":             countries.GB,
	"HK":                  countries.HK,
	"Swiss":               countries.CH,
	"Holland":             countries.NL,
	"Korea":               countries.KR,
	"Korea, Republic of":  countries.KR,
}

// CountryCode returns the ISO 3166 Alpha-2 country code for a given country name.
// It performs case-insensitive matching and handles common country name variations.
// Returns empty string if the country is not found.
// Note: For economic regions like "Euro Area", returns "EU" which is a common
// convention for the Eurozone in financial data.
func CountryCode(countryName string) string {
	if countryName == "" {
		return ""
	}

	// Normalize input
	normalized := strings.TrimSpace(countryName)

	// First check special codes (Euro Area, EU, etc.)
	if code, ok := specialCodes[normalized]; ok {
		return code
	}
	// Also check lowercase version for special codes
	lower := strings.ToLower(normalized)
	if code, ok := specialCodes[strings.Title(lower)]; ok {
		return code
	}

	// Try direct lookup (case-sensitive)
	if code, ok := nameToCountry[normalized]; ok {
		return code.Alpha2()
	}

	// Try with common variations
	mapped := mapCountryName(normalized)
	if mapped != "" {
		if code, ok := nameToCountry[mapped]; ok {
			return code.Alpha2()
		}
		// Check special codes for mapped name
		if code, ok := specialCodes[mapped]; ok {
			return code
		}
	}

	// Try cleaned name (remove parenthetical content)
	cleaned := cleanCountryName(normalized)
	if cleaned != normalized {
		if code, ok := nameToCountry[cleaned]; ok {
			return code.Alpha2()
		}
		// Also try mapping the cleaned name
		if mapped := mapCountryName(cleaned); mapped != "" {
			if code, ok := nameToCountry[mapped]; ok {
				return code.Alpha2()
			}
			// Check special codes for cleaned/mapped name
			if code, ok := specialCodes[mapped]; ok {
				return code
			}
		}
	}

	// Case-insensitive lookup as last resort
	for name, code := range nameToCountry {
		if strings.ToLower(name) == lower {
			return code.Alpha2()
		}
	}

	return ""
}

// mapCountryName handles common country name variations that don't have exact matches.
func mapCountryName(name string) string {
	lower := strings.ToLower(name)

	mappings := map[string]string{
		"united states":        "United States",
		"usa":                  "United States",
		"us":                   "United States",
		"u.s.":                 "United States",
		"u.s.a.":               "United States",
		"america":              "United States",
		"united kingdom":       "United Kingdom",
		"uk":                   "United Kingdom",
		"great britain":        "United Kingdom",
		"britain":              "United Kingdom",
		"england":              "United Kingdom",
		"china":                "China",
		"hong kong":            "Hong Kong",
		"hk":                   "Hong Kong",
		"japan":                "Japan",
		"germany":              "Germany",
		"france":               "France",
		"italy":                "Italy",
		"spain":                "Spain",
		"canada":               "Canada",
		"australia":            "Australia",
		"austria":              "Austria",
		"belgium":              "Belgium",
		"brazil":               "Brazil",
		"euro zone":            "Euro Area",
		"eurozone":             "Euro Area",
		"euro area":            "Euro Area",
		"eu":                   "Euro Area",
		"european union":       "European Union",
		"switzerland":          "Switzerland",
		"swiss":                "Switzerland",
		"netherlands":          "Netherlands",
		"holland":              "Netherlands",
		"new zealand":          "New Zealand",
		"russia":               "Russia",
		"russian federation":   "Russia",
		"south korea":          "South Korea",
		"korea":                "South Korea",
		"korea, republic of":  "South Korea",
		"singapore":            "Singapore",
		"india":                "India",
		"mexico":                "Mexico",
		"turkey":               "Turkey",
		"south africa":         "South Africa",
		"sweden":               "Sweden",
		"norway":               "Norway",
		"poland":               "Poland",
		"ireland":              "Ireland",
		"denmark":              "Denmark",
		"finland":              "Finland",
		"portugal":             "Portugal",
		"greece":               "Greece",
		"czech republic":       "Czech Republic",
		"czechia":              "Czech Republic",
		"hungary":              "Hungary",
		"romania":              "Romania",
		"bulgaria":             "Bulgaria",
		"indonesia":            "Indonesia",
		"malaysia":             "Malaysia",
		"thailand":             "Thailand",
		"philippines":          "Philippines",
		"vietnam":              "Vietnam",
		"taiwan":               "Taiwan",
		"taiwan, province of china": "Taiwan",
		"argentina":            "Argentina",
		"chile":                "Chile",
		"colombia":             "Colombia",
		"peru":                 "Peru",
		"venezuela":            "Venezuela",
		"saudi arabia":         "Saudi Arabia",
		"uae":                  "United Arab Emirates",
		"united arab emirates": "United Arab Emirates",
		"israel":               "Israel",
		"ukraine":              "Ukraine",
		"pakistan":             "Pakistan",
		"egypt":                "Egypt",
		"nigeria":              "Nigeria",
	}

	if mapped, ok := mappings[lower]; ok {
		return mapped
	}

	return ""
}

// cleanCountryName removes common suffixes and variations from country names.
func cleanCountryName(name string) string {
	// Remove parenthetical content
	if idx := strings.Index(name, "("); idx > 0 {
		name = strings.TrimSpace(name[:idx])
	}

	// Remove common suffixes
	suffixes := []string{
		" (core)",
		" (monthly)",
		" (annual)",
		" (yoy)",
		" (mom)",
	}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	return strings.TrimSpace(name)
}
