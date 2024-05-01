package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBroken(t *testing.T) {
	t.Run("Figure 1. A blocking bug caused by channel.", func(t *testing.T) {
		halfDelay := 250 * time.Millisecond
		delay := 2 * halfDelay

		fn := func() int { time.Sleep(delay); return 42 }
		exited := atomic.Bool{}

		ch := make(chan int)
		go func() {
			result := fn()
			ch <- result // block
			exited.Store(true)
		}()

		select {
		case result := <-ch:
			fmt.Println(result)
		case <-time.After(halfDelay):
			fmt.Println("timeout")
		}

		time.Sleep(delay)

		assert.Truef(t, exited.Load(), "the goroutine is now stuck and leaked")
	})

	t.Run("Figure 5. A blocking bug caused by WaitGroup.", func(t *testing.T) {
		shouldNotTimeout(t, func() {
			input := "abcd"
			var group sync.WaitGroup
			group.Add(len(input))
			for range input {
				go func() {
					defer group.Done()
				}()
				group.Wait()
			}
		})
	})
}

func shouldNotTimeout(t *testing.T, f func()) {
	timeout := time.After(100 * time.Millisecond)

	done := make(chan bool)

	go func() {
		f()
		done <- true
	}()

	select {
	case <-done:
		break
	case <-timeout:
		t.Fatalf("should not have timed out")
	}
}
