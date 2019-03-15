package limitter

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type Rate struct {
	Freq uint64
	Per  time.Duration
}

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
			log.Printf("rps: %v (cnt: %d, elapsed: %v, workers: %d)", rps, cnt, time.Since(began), workers)
			log.Printf("rrps: %v", rrps)

			err := w.Write([]string{fmt.Sprintf("%f", rps), fmt.Sprintf("%f", rrps)})
			if err != nil {
				log.Print(err)
			}
			w.Flush()

			if workers < 1 || rps < rrps*0.9 {
				// workerが足りてない場合は補充する
				workers++
				wg.Add(1)
				go func() {
					defer wg.Done()
					worker(ctx)
				}()
			}
		}
	}
	wg.Wait()
}
