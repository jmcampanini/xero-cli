package xero

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestBudgetLimitsConcurrentRequests(t *testing.T) {
	started := make(chan struct{}, 6)
	release := make(chan struct{})
	c := testClient(t, map[string]http.HandlerFunc{
		"/api/hold": func(w http.ResponseWriter, _ *http.Request) {
			started <- struct{}{}
			<-release
			_, _ = w.Write([]byte(`{}`))
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	for range 6 {
		workers.Go(func() {
			request, err := http.NewRequestWithContext(ctx, "GET", c.options.BaseURL+"hold", nil)
			if err != nil {
				t.Error(err)
				return
			}
			if _, _, _, err := c.send(ctx, request); err != nil {
				t.Error(err)
			}
		})
	}
	for range 5 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("five concurrent requests did not start")
		}
	}
	select {
	case <-started:
		t.Error("sixth request exceeded the concurrency limit")
	case <-time.After(30 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	workers.Wait()
}

func TestMinuteBudgetAndTokenExpiry(t *testing.T) {
	budget := NewBudget()
	if got := budget.limiter.Limit(); got != 1 {
		t.Errorf("rate = %v, want one request per second", got)
	}
	now := time.Now()
	if budget.limiter.Burst() != 5 || !budget.limiter.AllowN(now, 5) || budget.limiter.AllowN(now, 1) || !budget.limiter.AllowN(now.Add(time.Second), 1) {
		t.Error("minute budget did not allow a burst of five and then one per second")
	}
	c := testClient(t, nil)
	c.token = &oauth2.Token{AccessToken: "expired", Expiry: time.Now().Add(-time.Minute)}
	token, err := c.accessToken(context.Background(), false)
	if err != nil || token.AccessToken == "expired" {
		t.Errorf("expired token was not renewed: %v", err)
	}
	if token == nil || !token.Valid() {
		t.Fatal("renewed token is invalid")
	}
}
