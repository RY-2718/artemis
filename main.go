package main

import (
	"context"
	"github.com/RY-2718/rate_limit/limitter"
	"time"
)

func main() {
	f := func() {
		time.Sleep(300 * time.Millisecond)
		return
	}
	rate := limitter.Rate{
		Freq: 100,
		Per:  time.Second,
	}

	ctx := context.Background()

	limitter.RunWithRate(ctx, &rate, f)
}
