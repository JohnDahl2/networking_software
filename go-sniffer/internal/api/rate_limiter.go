package api

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct{
	maxToken float64
	currentToken float64
	rateLimit float64
	lastUpdate time.Time
	mu sync.Mutex
}

func NewRateLimit(maxToken float64, rateLimit float64) *rateLimiter{
	return &rateLimiter{
		maxToken: maxToken,
		currentToken: maxToken,
		rateLimit: rateLimit,
		lastUpdate: time.Now(),
	}
}

func (r *rateLimiter) Allow() bool{
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentToken += (time.Since(r.lastUpdate).Seconds() * r.rateLimit)
	r.lastUpdate = time.Now()
	if r.currentToken > r.maxToken{
		r.currentToken = r.maxToken
	}
	if r.currentToken < 1{
		return false
	}
	r.currentToken --
	return true

}

func (r *rateLimiter) Middleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            if !r.Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
                return 
            }
            next.ServeHTTP(w, req)
        })
    }
}
