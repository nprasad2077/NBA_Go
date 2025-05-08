package models

import (
	"time"

	"gorm.io/gorm"
)

// APIKey is stored **hashed** (SHA‑256) and may be revoked at any time.
type APIKey struct {
	ID        uint           `gorm:"primaryKey"`
	Hash      []byte         `gorm:"uniqueIndex"`
	Label     string         // e.g. “mobile‑app”, “data‑partner‑X”
	Revoked   bool
	CreatedAt time.Time
	RevokedAt gorm.DeletedAt `gorm:"index"`
}