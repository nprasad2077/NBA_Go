package controllers

import (
    "github.com/gofiber/fiber/v2"
    "gorm.io/gorm"
    "github.com/nprasad2077/NBA_Go/services"
    "github.com/nprasad2077/NBA_Go/models"
)

func FetchPlayerStats(db *gorm.DB) fiber.Handler {
    return func(c *fiber.Ctx) error {
        season := c.QueryInt("season", 2025)

        // Call service and assign err here
        err := services.FetchAndStorePlayerStats(db, season)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        return c.JSON(fiber.Map{"message": "Player stats fetched and saved."})
    }
}


func GetAllPlayerStats(db *gorm.DB) fiber.Handler {
    return func(c *fiber.Ctx) error {
        var stats []models.PlayerStat

        if err := db.Find(&stats).Error; err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        return c.JSON(stats)
    }
}