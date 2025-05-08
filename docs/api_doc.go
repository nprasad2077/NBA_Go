//go:build docs
// +build docs

// Package docs  Only contains Swagger annotations that are **global**.
// Run `swag init --parseDependency --parseInternal` after editing.
package docs

// ------------------------------------------------------------
// General API meta (kept here so CI can inject version/build).
// ------------------------------------------------------------

// @title          NBA_Go API
// @version        1.0
// @description    Stats service with API‑key auth
// @BasePath       /
// @schemes        http https

// ------------------------------------------------------------
// 🔐 SECURITY – this block is what you asked for
// ------------------------------------------------------------
//
// @securityDefinitions.apikey ApiKeyAuth
// @in          header
// @name        X-API-Key
//
// (every protected endpoint still needs `@Security ApiKeyAuth`)
// ------------------------------------------------------------