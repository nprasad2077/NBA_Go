package services

import "strconv"

// mustAtoi parses s into an int, or returns 0 on error.
func mustAtoi(s string) int {
    i, _ := strconv.Atoi(s)
    return i
}

// mustParseFloat parses s into a float64, or returns 0.0 on error.
func mustParseFloat(s string) float64 {
    f, _ := strconv.ParseFloat(s, 64)
    return f
}


