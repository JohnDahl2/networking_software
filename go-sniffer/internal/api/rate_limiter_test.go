package api

import (
	"testing"
	"time"
)


func TestAllow(t *testing.T){
	r := NewRateLimit(1.0, 2)
	b := r.Allow()
	if !b{
		t.Errorf("The basic allow should have happened here")
	}
	b = r.Allow()
	if b{
		t.Errorf("The basic allow should have not refilled so quickly happened here")
	}
	time.Sleep(2 * time.Second)
	b = r.Allow()
	if !b{
		t.Errorf("The basic allow should have happened here")
	}
}