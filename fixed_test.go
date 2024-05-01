package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestFigure1Fixed(t *testing.T) {
	t.Run("Figure 1. A blocking bug caused by channel.", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		halfDelay := 250 * time.Millisecond
		delay := 2 * halfDelay

		fn := func() int { time.Sleep(delay); return 42 }
		exited := atomic.Bool{}

		ch := make(chan int, 1)
		go func() {
			result := fn()
			ch <- result
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
}

func TestFigure5Fixed(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("Figure 5. A blocking bug caused by WaitGroup.", func(t *testing.T) {
		shouldNotTimeout(t, func() {
			input := "abcd"
			var group sync.WaitGroup
			group.Add(len(input))
			for range input {
				go func() {
					defer group.Done()
				}()
			}
			group.Wait()
		})
	})
}

func TestFigure6Fixed(t *testing.T) {
	t.Run("Figure 6. A blocking bug caused by context", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		ctx := context.Background()
		timeout := 1500 * time.Millisecond
		hctx, hcancel := context.WithCancel(ctx)

		go func() {
			<-hctx.Done() // blocks
			fmt.Println("this should not have happened")
		}()

		if timeout > 0 {
			hctx, hcancel = context.WithTimeout(ctx, timeout)
		}

		hcancel()

		assert.NotNil(t, hctx)

		<-hctx.Done()
	})
}

func TestFigure7Fixed(t *testing.T) {
	t.Run("Figure 7. A blocking bug caused by wrong usage of channel with lock.", func(t *testing.T) {
		shouldNotTimeoutDelay(t, time.Second, func() {
			m := sync.Mutex{}
			ch := make(chan int)

			wg := sync.WaitGroup{}
			wg.Add(1)

			goroutine1 := func() {
				time.Sleep(10 * time.Millisecond)
				m.Lock()
				select {
				case ch <- 42:
					break
				default:
					break
				}
				m.Unlock()
				fmt.Println("sent")
				wg.Done()
			}
			goroutine2 := func() {
				time.Sleep(10 * time.Millisecond)
				for {
					m.Lock()   // blocks
					m.Unlock() // as in the original
					<-ch
					fmt.Println("received")
				}
			}
			go goroutine1()
			go goroutine2()

			wg.Wait()
		})
	})
}
