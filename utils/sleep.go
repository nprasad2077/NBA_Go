// File: utils/sleep.go
package utils

import (
    "math/rand"
    "time"
)

func init() {
    // Seed the RNG once when the package is initialized
    rand.Seed(time.Now().UnixNano())
}

// SleepWithJitter sleeps for the given base duration plus or minus 25% random jitter.
// This helps to avoid perfectly uniform request intervals.
func SleepWithJitter(base time.Duration) {
    half := base / 2
    // Random value in [0, half)
    randOffset := time.Duration(rand.Int63n(int64(half)))
    // Shift to center jitter around zero: [-half/2, +half/2)
    delta := randOffset - half/2
    time.Sleep(base + delta)
}