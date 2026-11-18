package check

import (
	"fmt"
	"time"
)

// Eventually polls cond every interval and returns as soon as it reports true.
// It panics if cond is still false after timeout. cond is checked once
// immediately, and once more when the timeout expires. cond runs on the
// calling goroutine, and Eventually starts no goroutines. interval and timeout
// must be positive.
func Eventually(timeout, interval time.Duration, cond func() bool, msg ...string) {
	checkDurations("Eventually", timeout, interval)
	if cond() {
		return
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if cond() {
				return
			}
		case <-deadline.C:
			if cond() {
				return
			}
			panic(panicMsg(fmt.Sprintf("condition not met within %v", timeout), msg))
		}
	}
}

// Never polls cond every interval for the duration d and panics as soon as it
// reports true. It returns once d has passed without cond becoming true. cond
// is checked once immediately, and once more when d expires. cond runs on the
// calling goroutine, and Never starts no goroutines. d and interval must be
// positive.
func Never(d, interval time.Duration, cond func() bool, msg ...string) {
	checkDurations("Never", d, interval)
	fail := func() {
		panic(panicMsg(fmt.Sprintf("condition became true within %v", d), msg))
	}
	if cond() {
		fail()
	}
	deadline := time.NewTimer(d)
	defer deadline.Stop()
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if cond() {
				fail()
			}
		case <-deadline.C:
			if cond() {
				fail()
			}
			return
		}
	}
}

func checkDurations(fn string, d, interval time.Duration) {
	if d <= 0 || interval <= 0 {
		panic(fmt.Sprintf("%s: duration and interval must be positive, got %v and %v", fn, d, interval))
	}
}
