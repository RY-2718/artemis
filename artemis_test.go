package artemis

import (
	"context"
	"testing"
	"time"
)

func TestRunner_RunWithRate(t *testing.T) {
	f := func() {
		time.Sleep(300 * time.Millisecond)
	}
	rate := Rate{
		Freq:      10,
		Per:       time.Second,
		ErrorRate: 0.1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runner := Runner{}
	runner.RunWithRate(ctx, &rate, f)

	<-ctx.Done()

	report := runner.Report()

	if report.RPS < report.TargetRPS*(1-rate.ErrorRate) || report.RPS > report.TargetRPS*(1+rate.ErrorRate) {
		t.Error("RPS does not converge on Target RPS")
		t.Error("RPS       :", report.RPS)
		t.Error("Target RPS:", report.TargetRPS)
	}
}
