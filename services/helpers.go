package services

import (
	"log"
	"strconv"
	"sync"

	"gorm.io/gorm/schema"
)

// mustAtoi parses s into an int, or returns 0 on error.
// This function remains unchanged for general use.
func mustAtoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// mustAtoiWithSign is the new function to handle strings that might
// have a "+" or "-" sign, like the 'plus_minus' stat.
func mustAtoiWithSign(s string) int {
	// strconv.Atoi already handles signs correctly. This function
	// provides a clear, semantic name for its specific purpose.
	i, _ := strconv.Atoi(s)
	return i
}

// mustParseFloat parses s into a float64, or returns 0.0 on error.
func mustParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// getModelColumns uses reflection to discover model columns for dynamic upserts.
func getModelColumns(instance interface{}) []string {
	s, err := schema.Parse(instance, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		log.Printf("Failed to parse GORM schema: %v", err)
		return []string{}
	}

	columns := make([]string, 0, len(s.Fields))
	for _, field := range s.Fields {
		if field.PrimaryKey {
			continue // Skip primary key columns
		}
		columns = append(columns, field.DBName)
	}
	return columns
}