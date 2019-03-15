package limitter

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"sync"
	"time"
)

type Rate struct {
	Freq uint64
	Per  time.Duration // (int) * time.Second
}

type Phase int

const (
	SlowStart Phase = iota
	FastRecovery
)

func RunWithRate(ctx context.Context, rate *Rate, f func()) {
	cnt := uint64(0)
	interval := uint64(rate.Per.Nanoseconds() / int64(rate.Freq))
	began := time.Now()

	worker := func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				now, next := time.Now(), began.Add(time.Duration(cnt*interval))
				time.Sleep(next.Sub(now))
				f()
				cnt++
			}
		}
	}

	// measure actual rate
	wg := &sync.WaitGroup{}
	workers := uint64(0)
	ticker := time.NewTicker(1 * time.Second)
	cancelers := make([]context.CancelFunc, 0)
	p := SlowStart

	file, err := os.Create("./result.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	_ = w.Write([]string{"rps", "rrps"})

L:
	for {
		select {
		case <-ctx.Done():
			break L
		case <-ticker.C:
			rps := float64(cnt) / float64(time.Since(began)) * float64(time.Second)
			rrps := float64(rate.Freq) * float64(rate.Per) / float64(time.Second)
			log.Printf("rps: %v (cnt: %d, elapsed: %v, workers: %d, numGoroutine: %d)", rps, cnt, time.Since(began), workers, runtime.NumGoroutine())
			log.Printf("rrps: %v", rrps)

			err := w.Write([]string{fmt.Sprintf("%f", rps), fmt.Sprintf("%f", rrps)})
			if err != nil {
				log.Print(err)
			}
			w.Flush()

			if p == SlowStart && rps < rrps*0.9 {
				// workerが足りてない場合は補充する
				// スロースタート
				delta := int(workers)
				workers = uint64(math.Max(1, float64(workers*2)))
				wg.Add(delta)
				for i := 0; i < delta; i++ {
					go func() {
						defer wg.Done()
						_ctx, cancel := context.WithCancel(ctx)
						cancelers = append(cancelers, cancel)
						worker(_ctx)
					}()
				}
			} else if p == FastRecovery && rps < rrps*0.9 {
				// 半減させたあと足りない場合は１ずつ補充
				workers++
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ctx, cancel := context.WithCancel(ctx)
					cancelers = append(cancelers, cancel)
					worker(_ctx)
				}()
			} else if rps >= rrps*1.1 {
				// 半減させる
				delta := workers / 2
				workers -= delta

				log.Printf("cancel %d goroutines from %d goroutines", delta, runtime.NumGoroutine())
				for i := 0; i < int(delta); i++ {
					log.Printf("slice length: %d, goroutines: %d", len(cancelers), runtime.NumGoroutine())
					cancelers[0]()
					cancelers = cancelers[1:]
				}
				p = FastRecovery
			}
		}
	}
	wg.Wait()
}
