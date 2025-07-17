// utils/teams.go
package utils

// teamNameToAbbr is un-exported (private to the package) and serves as the single source of truth.
var teamNameToAbbr = map[string]string{
	"Atlanta Hawks":         "ATL",
	"Boston Celtics":        "BOS",
	"Brooklyn Nets":         "BKN",
	"Charlotte Hornets":     "CHA",
	"Chicago Bulls":         "CHI",
	"Cleveland Cavaliers":   "CLE",
	"Dallas Mavericks":      "DAL",
	"Denver Nuggets":        "DEN",
	"Detroit Pistons":       "DET",
	"Golden State Warriors": "GSW",
	"Houston Rockets":       "HOU",
	"Indiana Pacers":        "IND",
	"Los Angeles Clippers":  "LAC",
	"Los Angeles Lakers":    "LAL",
	"Memphis Grizzlies":     "MEM",
	"Miami Heat":            "MIA",
	"Milwaukee Bucks":       "MIL",
	"Minnesota Timberwolves":"MIN",
	"New Orleans Pelicans":  "NOP",
	"New York Knicks":       "NYK",
	"Oklahoma City Thunder": "OKC",
	"Orlando Magic":         "ORL",
	"Philadelphia 76ers":    "PHI",
	"Phoenix Suns":          "PHX",
	"Portland Trail Blazers":"POR",
	"Sacramento Kings":      "SAC",
	"San Antonio Spurs":     "SAS",
	"Toronto Raptors":       "TOR",
	"Utah Jazz":             "UTA",
	"Washington Wizards":    "WAS",
}

// TeamAbbrToName is the exported reverse map.
var TeamAbbrToName map[string]string

// init() runs automatically when the package is first used.
// It creates the reverse map so it's ready for use.
func init() {
	TeamAbbrToName = make(map[string]string, len(teamNameToAbbr))
	for name, abbr := range teamNameToAbbr {
		TeamAbbrToName[abbr] = name
	}
}

// GetAbbreviation looks up a team's full name and returns its abbreviation.
func GetAbbreviation(fullName string) string {
	if abbr, ok := teamNameToAbbr[fullName]; ok {
		return abbr
	}
	return fullName // Fallback
}

// GetFullName looks up a team's abbreviation and returns its full name.
func GetFullName(abbr string) string {
	if name, ok := TeamAbbrToName[abbr]; ok {
		return name
	}
	return abbr // Fallback
}