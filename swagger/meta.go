// Package main holds the global Swagger annotations.
// Run “swag init --parseDependency --parseInternal” after editing.
package main

// -----------------------------------------------------------------------------
// General API information
// -----------------------------------------------------------------------------

// @title       NBA_Go API
// @version     1.0
// @description Stats service with API‑key auth
// @schemes     http https
// @BasePath    /

//
// -----------------------------------------------------------------------------
// 🔐 SECURITY
// -----------------------------------------------------------------------------
// @securityDefinitions.apikey ApiKeyAuth
// @in   header
// @name X-API-Key
//
// (each protected handler still needs `@Security ApiKeyAuth`)
// -----------------------------------------------------------------------------