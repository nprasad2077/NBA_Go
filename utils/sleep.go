// File: utils/sleep.go
package utils

import (
    "log"
    "math/rand"
    "time"
)

func init() {
    // Seed the RNG once when the package is initialized
    rand.Seed(time.Now().UnixNano())
}

// SleepWithJitter sleeps for the given base duration plus or minus 25% random jitter.
// It also logs the actual sleep duration so you can verify it fired correctly.
func SleepWithJitter(base time.Duration) {
    half := base / 2
    // Random value in [0, half)
    randOffset := time.Duration(rand.Int63n(int64(half)))
    // Shift to center jitter around zero: [-half/2, +half/2)
    delta := randOffset - half/2
    actual := base + delta
    log.Printf("⏱️  Sleeping for %v (base=%v, jitter=%v)", actual, base, delta)
    time.Sleep(actual)
}
