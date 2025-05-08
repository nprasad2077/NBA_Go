// swagger/swagger_doc.go
package main

// -----------------------------------------------------------------------------
// Global Swagger metadata
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
// (each protected handler should still add `@Security ApiKeyAuth`)
// -----------------------------------------------------------------------------