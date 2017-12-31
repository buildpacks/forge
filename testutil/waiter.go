package testutil

import "sync"

func Wait(delta int) func() {
	wg := sync.WaitGroup{}
	wg.Add(delta)
	return func() {
		wg.Done()
		wg.Wait()
	}
}
