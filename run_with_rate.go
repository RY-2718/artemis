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

			if p == SlowStart && rate.rps < rate.targetRps*0.9 {
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
			} else if p == FastRecovery && rate.rps < rate.targetRps*0.9 {
				// Add workers gently when fast recovery
				// TODO: 補充するペースを調整？
				rate.numWorker++
				wg.Add(1)
				go func() {
					defer wg.Done()
					worker(quit)
				}()
			} else if rate.rps >= rate.targetRps*1.1 {
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
