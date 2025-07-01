package controllers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/services"
	"gorm.io/gorm"
)

// FetchPlayerShotChartAPI godoc
// @Summary     Fetch a player's shot-chart from NBA-API and store in DB
// @Tags        PlayerShotChart
// @Param       playerId query string true "Player ID (e.g. hardeja01)"
// @Accept      json
// @Produce     json
// @Success     200      {object} map[string]string
// @Failure     400,500  {object} map[string]string
// @Router      /api/playershotchart/fetch [get]
// func FetchPlayerShotChartAPI(db *gorm.DB) fiber.Handler {
//     return func(c *fiber.Ctx) error {
//         pid := c.Query("playerId")
//         if pid == "" {
//             return c.Status(400).JSON(fiber.Map{"error": "playerId query parameter is required"})
//         }
//         if err := services.FetchAndStoreShotChartForPlayer(db, pid); err != nil {
//             return c.Status(500).JSON(fiber.Map{"error": err.Error()})
//         }
//         return c.JSON(fiber.Map{"message": "Shot chart for " + pid + " fetched and saved."})
//     }
// }

// ScrapePlayerShotChart godoc
// @ignore
// @Summary     Scrape a player's shot-chart from BR website
// @Description Scrapes seasons [startSeason…endSeason] for the given playerId
//
//	(playerName is auto-detected).
//
// @Tags        PlayerShotChart
// @Accept      json
// @Produce     json
// @Param       playerId    query  string true  "Player ID (e.g. derozde01)"
// @Param       startSeason query  int    true  "Start season (e.g. 2024)"
// @Param       endSeason   query  int    true  "End season (e.g. 2021)"
// @Success     200         {object} map[string]string
// @Failure     400,500     {object} map[string]string
// //@Router      /api/playershotchart/scrape [get]
func ScrapePlayerShotChart(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		pid := c.Query("playerId")
		start := c.QueryInt("startSeason", 0)
		end := c.QueryInt("endSeason", 0)
		if pid == "" || start == 0 || end == 0 {
			return c.Status(400).JSON(fiber.Map{
				"error": "playerId, startSeason and endSeason are required",
			})
		}
		if start < end {
			return c.Status(400).JSON(fiber.Map{
				"error": "startSeason must be >= endSeason",
			})
		}
		if err := services.FetchAndStoreShotChartScrapedForPlayer(db, pid, start, end); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "Shot chart scraped and saved for " + pid})
	}
}


// GetPlayerShotChart godoc
// @Summary      Get shot-chart data
// @Description  Returns a paginated list of shot-chart points, optionally filtered by various parameters.
// @Tags         PlayerShotChart
// @x-order 3
// @Group Player-Specific
// @Accept       json
// @Produce      json
// @Param        page      query  int     false  "Page number for pagination (defaults to 1)"
// @Param        playerId  query  string  false  "Player ID (e.g., hardeja01)"
// @Param        season    query  int     false  "Season (e.g., 2023)"
// @Param        date      query  string  false  "Game date (e.g., Oct 17, 2018)"
// @Param        qtr       query  string  false  "Quarter (e.g., 1st Qtr)"
// @Param        result    query  boolean false  "Shot result (true for made, false for missed)"
// @Param        shot_type query  string  false  "Shot type (e.g., 2-pointer)"
// @Param        opponent  query  string  false  "Opponent team (e.g., NOP)"
// @Success      200       {array}   models.PlayerShotChart
// @Failure      500       {object}  map[string]string
// @Router       /api/playershotchart [get]
func GetPlayerShotChart(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var shots []models.PlayerShotChart

		query := db.Model(&models.PlayerShotChart{})

		// --- Filtering Logic (unchanged) ---
		if pid := c.Query("playerId"); pid != "" {
			query = query.Where("player_id = ?", pid)
		}
		if s := c.QueryInt("season"); s != 0 {
			query = query.Where("season = ?", s)
		}
		if date := c.Query("date"); date != "" {
			query = query.Where("date = ?", date)
		}
		if qtr := c.Query("qtr"); qtr != "" {
			query = query.Where("qtr = ?", qtr)
		}
		if c.Query("result") != "" {
			query = query.Where("result = ?", c.QueryBool("result"))
		}
		if shotType := c.Query("shot_type"); shotType != "" {
			query = query.Where("shot_type = ?", shotType)
		}
		if opponent := c.Query("opponent"); opponent != "" {
			query = query.Where("opponent = ?", opponent)
		}

		// --- Pagination Logic ---
		// Set a fixed limit of 50 results per page.
		limit := 50

		// Get the page number from the query, defaulting to 1.
		page := c.QueryInt("page", 1)

		// Calculate the offset based on the page number and limit.
		offset := (page - 1) * limit

		// Apply Limit and Offset to the GORM query.
		query = query.Limit(limit).Offset(offset)


		if err := query.Find(&shots).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(shots)
	}
}
