package main

import (
	"context"
	"time"

	"github.com/RY-2718/goroutine_compressor/limitter"
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
