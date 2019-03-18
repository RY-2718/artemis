/*
The MIT License (MIT)

Copyright (c) 2019 RY-2718.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package artemis

import (
	"context"
	"math"
	"sync"
	"time"
)

type Rate struct {
	Freq      uint64
	Per       time.Duration // (int) * time.Second
	ErrorRate float64
	rps       float64
	targetRps float64
	numWorker uint64
}

type Report struct {
	RPS       float64
	TargetRPS float64
	NumWorker uint64
}

type Phase int

const (
	SlowStart Phase = iota
	FastRecovery
)

func (r *Rate) Report() Report {
	return Report{
		RPS:       r.rps,
		TargetRPS: r.targetRps,
		NumWorker: r.numWorker,
	}
}

func RunWithRate(ctx context.Context, rate *Rate, f func()) {
	cnt := uint64(0)
	interval := uint64(rate.Per.Nanoseconds() / int64(rate.Freq))
	began := time.Now()

	worker := func(quit <-chan bool) {
		for {
			select {
			case <-quit:
				return
			default:
				now, next := time.Now(), began.Add(time.Duration(cnt*interval))
				time.Sleep(next.Sub(now))
				f()
				cnt++
			}
		}
	}

	wg := &sync.WaitGroup{}
	ticker := time.NewTicker(1 * time.Second)
	p := SlowStart

	quit := make(chan bool, 1000)

L:
	for {
		select {
		case <-ctx.Done():
			for i := 0; i < int(rate.numWorker); i++ {
				quit <- true
			}
			break L
		case <-ticker.C:
			// measure actual rate
			rate.rps = float64(cnt) / float64(time.Since(began)) * float64(time.Second)
			rate.targetRps = float64(rate.Freq) * float64(rate.Per) / float64(time.Second)

			if p == SlowStart && rate.rps < rate.targetRps*(1.0-rate.ErrorRate) {
				// Add workers exponentially when slow start
				delta := int(rate.numWorker)
				rate.numWorker = uint64(math.Max(1, float64(rate.numWorker*2)))
				wg.Add(delta)
				for i := 0; i < delta; i++ {
					go func() {
						defer wg.Done()
						worker(quit)
					}()
				}
			} else if p == FastRecovery && rate.rps < rate.targetRps*(1.0-rate.ErrorRate) {
				// Add workers gently when fast recovery
				delta := int(float64(rate.numWorker) * 0.1)
				rate.numWorker += uint64(delta)
				wg.Add(delta)
				for i := 0; i < delta; i++ {
					go func() {
						defer wg.Done()
						worker(quit)
					}()
				}
			} else if rate.rps >= rate.targetRps*(1.0+rate.ErrorRate) {
				// Reduce workers by half
				delta := rate.numWorker / 2
				rate.numWorker -= delta

				for i := 0; i < int(delta); i++ {
					quit <- true
				}
				p = FastRecovery
			}
		}
	}
	wg.Wait()
}
