package service

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var errYahooCooldown = errors.New("Yahoo HTTP 429: requests temporarily paused")

type yahooRequests struct {
	once         sync.Once
	slots        chan struct{}
	mu           sync.Mutex
	next, resume time.Time
	backoff      time.Duration
}

type yahooBody struct {
	io.ReadCloser
	release func()
}

func (b *yahooBody) Close() error {
	defer b.release()
	return b.ReadCloser.Close()
}

func retryDelay(header string, now time.Time, fallback time.Duration) time.Duration {
	if sec, err := strconv.ParseUint(header, 10, 32); err == nil {
		return time.Duration(sec) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return fallback
}

func (s *YahooService) yahooRequest(req *http.Request) (*http.Response, error) {
	g := &s.requests
	g.once.Do(func() { g.slots = make(chan struct{}, 2) })
	select {
	case g.slots <- struct{}{}:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	release := sync.OnceFunc(func() { <-g.slots })
	for {
		g.mu.Lock()
		now := time.Now()
		if now.Before(g.resume) {
			g.mu.Unlock()
			release()
			return nil, errYahooCooldown
		}
		wait := g.next.Sub(now)
		if wait <= 0 {
			g.next = now.Add(time.Second)
			g.mu.Unlock()
			break
		}
		g.mu.Unlock()
		select {
		case <-time.After(wait):
		case <-req.Context().Done():
			release()
			return nil, req.Context().Err()
		}
	}
	client := s.marketClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		release()
		return nil, err
	}
	g.mu.Lock()
	now := time.Now()
	if resp.StatusCode == http.StatusTooManyRequests {
		g.backoff = min(15*time.Minute, max(time.Minute, 2*g.backoff))
		until := now.Add(retryDelay(resp.Header.Get("Retry-After"), now, g.backoff))
		if until.After(g.resume) {
			g.resume = until
		}
	} else if resp.StatusCode == http.StatusOK && !now.Before(g.resume) {
		g.backoff = 0
	}
	g.mu.Unlock()
	resp.Body = &yahooBody{resp.Body, release}
	return resp, nil
}
