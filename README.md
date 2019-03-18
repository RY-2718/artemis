# Artemis

Artemis is a library which dynamically resize the amount of workers (goroutines)
to run functions in parallel at your target rate.

## Usage

```go
package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/RY-2718/artemis"
)

func main() {
	// Define the function to run
	f := func() {
		time.Sleep(300 * time.Millisecond)
	}
	
	// Define the rate (frequency, error rate)
	rate := artemis.Rate{
		Freq:      100,
		Per:       time.Second, // Set `Per` param as time.Second now
		ErrorRate: 0.1,
	}

	ctx := context.Background()

	go artemis.RunWithRate(ctx, &rate, f)

    // If you want to report periodically, using Ticker is a solution
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			// rate.Report() returns a struct which have RPS (rate per second), TargetRPS
			r := rate.Report()
			fmt.Printf("%f, %f, %d\n", r.RPS, r.TargetRPS, runtime.NumGoroutine())
		}
	}
}
```
