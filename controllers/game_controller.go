package controllers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils"
)

// DTOs (Data Transfer Objects) for API responses
// These structs define the exact JSON output, ensuring team names are abbreviated.

type LineScoreDTO struct {
	Team  string `json:"team"` // Abbreviation
	Q1    int    `json:"q1"`
	Q2    int    `json:"q2"`
	Q3    int    `json:"q3"`
	Q4    int    `json:"q4"`
	OT1   int    `json:"ot1"`
	OT2   int    `json:"ot2"`
	OT3   int    `json:"ot3"`
	Total int    `json:"total"`
}

type PlayerGameBasicStatDTO struct {
	PlayerID      string  `json:"playerId"`
	PlayerName    string  `json:"playerName"`
	Team          string  `json:"team"` // Abbreviation
	Status        string  `json:"status"`
	MP            string  `json:"mp"`
	FG            int     `json:"fg"`
	FGA           int     `json:"fga"`
	FGPercent     float64 `json:"fgPercent"`
	ThreeP        int     `json:"threeP"`
	ThreePA       int     `json:"threePa"`
	ThreePPercent float64 `json:"threePPercent"`
	FT            int     `json:"ft"`
	FTA           int     `json:"fta"`
	FTPercent     float64 `json:"ftPercent"`
	ORB           int     `json:"orb"`
	DRB           int     `json:"drb"`
	TRB           int     `json:"trb"`
	AST           int     `json:"ast"`
	STL           int     `json:"stl"`
	BLK           int     `json:"blk"`
	TOV           int     `json:"tov"`
	PF            int     `json:"pf"`
	PTS           int     `json:"pts"`
	GmSc          float64 `json:"gmSc"`
	PlusMinus     int     `json:"plusMinus"`
}

type PlayerGameAdvStatDTO struct {
	PlayerID   string  `json:"playerId"`
	PlayerName string  `json:"playerName"`
	Team       string  `json:"team"` // Abbreviation
	MP         string  `json:"mp"`
	TSPercent  float64 `json:"tsPercent"`
	EFGPercent float64 `json:"efgPercent"`
	ThreePAr   float64 `json:"threePAr"`
	FTr        float64 `json:"fTr"`
	ORBPercent float64 `json:"orbPercent"`
	DRBPercent float64 `json:"drbPercent"`
	TRBPercent float64 `json:"trbPercent"`
	ASTPercent float64 `json:"astPercent"`
	STLPercent float64 `json:"stlPercent"`
	BLKPercent float64 `json:"blkPercent"`
	TOVPercent float64 `json:"tovPercent"`
	USGPercent float64 `json:"usgPercent"`
	ORtg       int     `json:"oRtg"`
	DRtg       int     `json:"dRtg"`
	BPM        float64 `json:"bpm"`
}

type TeamGameBasicStatDTO struct {
	Team          string  `json:"team"` // Abbreviation
	FG            int     `json:"fg"`
	FGA           int     `json:"fga"`
	FGPercent     float64 `json:"fgPercent"`
	ThreeP        int     `json:"threeP"`
	ThreePA       int     `json:"threePa"`
	ThreePPercent float64 `json:"threePPercent"`
	FT            int     `json:"ft"`
	FTA           int     `json:"fta"`
	FTPercent     float64 `json:"ftPercent"`
	ORB           int     `json:"orb"`
	DRB           int     `json:"drb"`
	TRB           int     `json:"trb"`
	AST           int     `json:"ast"`
	STL           int     `json:"stl"`
	BLK           int     `json:"blk"`
	TOV           int     `json:"tov"`
	PF            int     `json:"pf"`
	PTS           int     `json:"pts"`
}

type TeamGameAdvStatDTO struct {
	Team       string  `json:"team"` // Abbreviation
	ORtg       float64 `json:"oRtg"`
	DRtg       float64 `json:"dRtg"`
	TSPercent  float64 `json:"tsPercent"`
	EFGPercent float64 `json:"efgPercent"`
	ThreePAr   float64 `json:"threePAr"`
	FTr        float64 `json:"fTr"`
	ORBPercent float64 `json:"orbPercent"`
	DRBPercent float64 `json:"drbPercent"`
	TRBPercent float64 `json:"trbPercent"`
	ASTPercent float64 `json:"astPercent"`
	STLPercent float64 `json:"stlPercent"`
	BLKPercent float64 `json:"blkPercent"`
	TOVPercent float64 `json:"tovPercent"`
}

// GameResponseDTO is the top-level object for a single game in the response.
type GameResponseDTO struct {
	ID                   uint                       `json:"id"`
	GameID               string                     `json:"gameId"`
	Date                 time.Time                  `json:"date"`
	IsPlayoff            bool                       `json:"isPlayoff"`
	StartTimeET          string                     `json:"startTimeET"`
	Arena                string                     `json:"arena"`
	VisitorTeam          string                     `json:"visitorTeam"`
	VisitorPTS           int                        `json:"visitorPts"`
	HomeTeam             string                     `json:"homeTeam"`
	HomePTS              int                        `json:"homePts"`
	GameDuration         string                     `json:"gameDuration"`
	BoxScoreURL          string                     `json:"boxScoreUrl"`
	LineScores           []LineScoreDTO             `json:"lineScores,omitempty"`
	PlayerGameBasicStats []PlayerGameBasicStatDTO   `json:"playerGameBasicStats,omitempty"`
	PlayerGameAdvStats   []PlayerGameAdvStatDTO     `json:"playerGameAdvStats,omitempty"`
	TeamGameBasicStats   []TeamGameBasicStatDTO     `json:"teamGameBasicStats,omitempty"`
	TeamGameAdvStats     []TeamGameAdvStatDTO       `json:"teamGameAdvStats,omitempty"`
}

// GamesResponse is the final paginated structure returned by the API.
type GamesResponse struct {
	Data       []GameResponseDTO `json:"data"`
	Pagination struct {
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Pages    int64 `json:"pages"`
	} `json:"pagination"`
}

// gameSortMap translates API sort parameters to database column names.
var gameSortMap = map[string]string{
	"date":         "date",
	"visitorPts":   "visitor_pts",
	"homePts":      "home_pts",
	"gameDuration": "game_duration",
}

// GetGames godoc
// @Summary      Get game data
// @Description  Returns a paginated list of games with optional filters and included associations.
// @Tags         Games
// @Produce      json
// @Param        date         query  string  false  "Filter by date (YYYY-MM-DD)"
// @Param        team         query  string  false  "Filter by team abbreviation (e.g., LAL)"
// @Param        gameId       query  string  false  "Filter by a specific Game ID"
// @Param        isPlayoff    query  bool    false  "Filter for playoff games"
// @Param        page         query  int     false  "Page number" default(1)
// @Param        pageSize     query  int     false  "Page size" default(20)
// @Param        sortBy       query  string  false  "Field to sort by (e.g., date)" default(date)
// @Param        ascending    query  bool    false  "Sort ascending" default(false)
// @Param        include      query  string  false  "Comma-separated list of associations to include (e.g., lineScores,playerGameBasicStats,teamGameAdvStats)"
// @Success      200          {object} controllers.GamesResponse
// @Failure      400          {object} map[string]string
// @Failure      500          {object} map[string]string
// @Router       /api/games [get]
func GetGames(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// --- Build Query ---
		query := db.Model(&models.Game{})

		// --- Filtering ---
		if gameId := c.Query("gameId"); gameId != "" {
			query = query.Where("game_id = ?", gameId)
		}
		if dateStr := c.Query("date"); dateStr != "" {
			parsedDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid date format. Use YYYY-MM-DD."})
			}
			query = query.Where("date = ?", parsedDate)
		}
		if teamAbbr := c.Query("team"); teamAbbr != "" {
			// Use the helper function to get the full name for the DB query
			fullName := utils.GetFullName(strings.ToUpper(teamAbbr))
			if fullName != teamAbbr { // Check if a match was found
				query = query.Where("visitor_team = ? OR home_team = ?", fullName, fullName)
			}
		}
		if c.Query("isPlayoff") != "" {
			query = query.Where("is_playoff = ?", c.QueryBool("isPlayoff"))
		}

		// --- Preloading Associations ---
		includeParam := c.Query("include")
		if includeParam != "" {
			associations := strings.Split(includeParam, ",")
			for _, assoc := range associations {
				// Preload the original model association based on the JSON field name
				// GORM is smart enough to handle the mapping from struct field to relation
				switch strings.TrimSpace(assoc) {
				case "lineScores":
					query = query.Preload("LineScores")
				case "playerGameBasicStats":
					query = query.Preload("PlayerGameBasicStats")
				case "playerGameAdvStats":
					query = query.Preload("PlayerGameAdvStats")
				case "teamGameBasicStats":
					query = query.Preload("TeamGameBasicStats")
				case "teamGameAdvStats":
					query = query.Preload("TeamGameAdvStats")
				}
			}
		}

		// --- Sorting ---
		sortByParam := c.Query("sortBy", "date")
		sortBy, ok := gameSortMap[sortByParam]
		if !ok {
			sortBy = "date"
		}
		order := sortBy + " DESC"
		if c.QueryBool("ascending", false) {
			order = sortBy + " ASC"
		}
		query = query.Order(order)

		// --- Pagination & Data Fetching ---
		var total int64
		query.Count(&total)

		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("pageSize", 20)
		offset := (page - 1) * pageSize

		var games []models.Game
		if err := query.Limit(pageSize).Offset(offset).Find(&games).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// --- Transformation from Model to DTO ---
		gameDTOs := make([]GameResponseDTO, len(games))
		for i, game := range games {
			gameDTOs[i] = toGameResponseDTO(game)
		}

		// --- Build Final Response ---
		resp := GamesResponse{Data: gameDTOs}
		resp.Pagination.Total = total
		resp.Pagination.Page = page
		resp.Pagination.PageSize = pageSize
		resp.Pagination.Pages = (total + int64(pageSize) - 1) / int64(pageSize)

		return c.JSON(resp)
	}
}

// toGameResponseDTO converts a Game model to its corresponding DTO, including nested associations.
func toGameResponseDTO(game models.Game) GameResponseDTO {
	dto := GameResponseDTO{
		ID:           game.ID,
		GameID:       game.GameID,
		Date:         game.Date,
		IsPlayoff:    game.IsPlayoff,
		StartTimeET:  game.StartTimeET,
		Arena:        game.Arena,
		VisitorTeam:  utils.GetAbbreviation(game.VisitorTeam),
		VisitorPTS:   game.VisitorPTS,
		HomeTeam:     utils.GetAbbreviation(game.HomeTeam),
		HomePTS:      game.HomePTS,
		GameDuration: game.GameDuration,
		BoxScoreURL:  game.BoxScoreURL,
	}

	// Transform nested slices if they were loaded
	if len(game.LineScores) > 0 {
		dto.LineScores = make([]LineScoreDTO, len(game.LineScores))
		for i, ls := range game.LineScores {
			dto.LineScores[i] = LineScoreDTO{
				Team:  utils.GetAbbreviation(ls.Team),
				Q1:    ls.Q1, Q2: ls.Q2, Q3: ls.Q3, Q4: ls.Q4,
				OT1:   ls.OT1, OT2: ls.OT2, OT3: ls.OT3,
				Total: ls.Total,
			}
		}
	}

	if len(game.PlayerGameBasicStats) > 0 {
		dto.PlayerGameBasicStats = make([]PlayerGameBasicStatDTO, len(game.PlayerGameBasicStats))
		for i, pbs := range game.PlayerGameBasicStats {
			dto.PlayerGameBasicStats[i] = PlayerGameBasicStatDTO{
				PlayerID: pbs.PlayerID, PlayerName: pbs.PlayerName, Team: utils.GetAbbreviation(pbs.Team),
				Status: pbs.Status, MP: pbs.MP, FG: pbs.FG, FGA: pbs.FGA, FGPercent: pbs.FGPercent,
				ThreeP: pbs.ThreeP, ThreePA: pbs.ThreePA, ThreePPercent: pbs.ThreePPercent,
				FT: pbs.FT, FTA: pbs.FTA, FTPercent: pbs.FTPercent, ORB: pbs.ORB, DRB: pbs.DRB,
				TRB: pbs.TRB, AST: pbs.AST, STL: pbs.STL, BLK: pbs.BLK, TOV: pbs.TOV, PF: pbs.PF,
				PTS: pbs.PTS, GmSc: pbs.GmSc, PlusMinus: pbs.PlusMinus,
			}
		}
	}

	if len(game.PlayerGameAdvStats) > 0 {
		dto.PlayerGameAdvStats = make([]PlayerGameAdvStatDTO, len(game.PlayerGameAdvStats))
		for i, pas := range game.PlayerGameAdvStats {
			dto.PlayerGameAdvStats[i] = PlayerGameAdvStatDTO{
				PlayerID: pas.PlayerID, PlayerName: pas.PlayerName, Team: utils.GetAbbreviation(pas.Team),
				MP: pas.MP, TSPercent: pas.TSPercent, EFGPercent: pas.EFGPercent, ThreePAr: pas.ThreePAr,
				FTr: pas.FTr, ORBPercent: pas.ORBPercent, DRBPercent: pas.DRBPercent, TRBPercent: pas.TRBPercent,
				ASTPercent: pas.ASTPercent, STLPercent: pas.STLPercent, BLKPercent: pas.BLKPercent,
				TOVPercent: pas.TOVPercent, USGPercent: pas.USGPercent, ORtg: pas.ORtg, DRtg: pas.DRtg, BPM: pas.BPM,
			}
		}
	}

	if len(game.TeamGameBasicStats) > 0 {
		dto.TeamGameBasicStats = make([]TeamGameBasicStatDTO, len(game.TeamGameBasicStats))
		for i, tbs := range game.TeamGameBasicStats {
			dto.TeamGameBasicStats[i] = TeamGameBasicStatDTO{
				Team: utils.GetAbbreviation(tbs.Team), FG: tbs.FG, FGA: tbs.FGA, FGPercent: tbs.FGPercent,
				ThreeP: tbs.ThreeP, ThreePA: tbs.ThreePA, ThreePPercent: tbs.ThreePPercent,
				FT: tbs.FT, FTA: tbs.FTA, FTPercent: tbs.FTPercent, ORB: tbs.ORB, DRB: tbs.DRB,
				TRB: tbs.TRB, AST: tbs.AST, STL: tbs.STL, BLK: tbs.BLK, TOV: tbs.TOV, PF: tbs.PF, PTS: tbs.PTS,
			}
		}
	}

	if len(game.TeamGameAdvStats) > 0 {
		dto.TeamGameAdvStats = make([]TeamGameAdvStatDTO, len(game.TeamGameAdvStats))
		for i, tas := range game.TeamGameAdvStats {
			dto.TeamGameAdvStats[i] = TeamGameAdvStatDTO{
				Team: utils.GetAbbreviation(tas.Team), ORtg: tas.ORtg, DRtg: tas.DRtg, TSPercent: tas.TSPercent,
				EFGPercent: tas.EFGPercent, ThreePAr: tas.ThreePAr, FTr: tas.FTr, ORBPercent: tas.ORBPercent,
				DRBPercent: tas.DRBPercent, TRBPercent: tas.TRBPercent, ASTPercent: tas.ASTPercent,
				STLPercent: tas.STLPercent, BLKPercent: tas.BLKPercent, TOVPercent: tas.TOVPercent,
			}
		}
	}

	return dto
}