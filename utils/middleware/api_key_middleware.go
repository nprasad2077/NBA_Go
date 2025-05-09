package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/keyauth"
	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils/security"
	"gorm.io/gorm"
)

// APIKeyAuth returns a Fiber Handler that validates X‑API‑Key
func APIKeyAuth(db *gorm.DB) fiber.Handler {
	return keyauth.New(keyauth.Config{
		KeyLookup: "header:X-API-Key",
		Validator: func(c *fiber.Ctx, rawKey string) (bool, error) {
			var rec models.APIKey
			hash := security.HashKey(rawKey)

			err := db.
				Where("hash = ? AND revoked = FALSE", hash).
				First(&rec).Error
			if err != nil {
				return false, keyauth.ErrMissingOrMalformedAPIKey
			}
			if subtle.ConstantTimeCompare(rec.Hash, hash) != 1 {
				return false, keyauth.ErrMissingOrMalformedAPIKey
			}
			// put the key ID on the context for rate‑limiting or logging later
			c.Locals("apiKeyID", rec.ID)
			return true, nil
		},
	})
}