package controllers

import (
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils/security"
	"gorm.io/gorm"
)

// very small middleware: require header X‑Admin‑Secret == $ADMIN_SECRET
func adminGuard() fiber.Handler {
	secret := os.Getenv("ADMIN_SECRET")
	return func(c *fiber.Ctx) error {
		if c.Get("X-Admin-Secret") != secret {
			return c.SendStatus(http.StatusUnauthorized)
		}
		return c.Next()
	}
}

func RegisterKeyAdminRoutes(app *fiber.App, db *gorm.DB) {
	admin := app.Group("/admin/keys", adminGuard())

	admin.Get("/", func(c *fiber.Ctx) error {
		var keys []models.APIKey
		db.Find(&keys)
		return c.JSON(keys)
	})

	admin.Post("/", func(c *fiber.Ctx) error {
		var body struct{ Label string }
		if err := c.BodyParser(&body); err != nil {
			return err
		}

		raw, err := security.GenerateRawKey()
		if err != nil {
			return err
		}

		key := models.APIKey{
			Hash:  security.HashKey(raw),
			Label: body.Label,
		}
		db.Create(&key)

		// return ONLY the raw key once
		return c.JSON(fiber.Map{
			"apiKey": raw,
			"id":     key.ID,
		})
	})

	admin.Post("/:id/revoke", func(c *fiber.Ctx) error {
		id := c.Params("id")
		db.Model(&models.APIKey{}).Where("id = ?", id).Update("revoked", true)
		return c.JSON(fiber.Map{"revoked": id})
	})
}