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

func TestFigure1(t *testing.T) {
	t.Run("Figure 1. A blocking bug caused by channel.", func(t *testing.T) {
		defer goleak.VerifyNone(t)

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
}

func TestFigure5(t *testing.T) {
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

func TestFigure6(t *testing.T) {
	t.Run("Figure 6. A blocking bug caused by context", func(t *testing.T) {
		goleak.VerifyNone(t)

		ctx := context.Background()
		timeout := 1500 * time.Millisecond
		hctx, hcancel := context.WithCancel(ctx)
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			<-hctx.Done() // blocks
			fmt.Println("this is new behavior, it seems")
			wg.Done()
		}()

		if timeout > 0 {
			hctx, hcancel = context.WithTimeout(ctx, timeout)
		}

		hcancel()

		assert.NotNil(t, hctx)

		<-hctx.Done()
		fmt.Println("cancelled")
		wg.Wait()
	})
}

func TestFigure7(t *testing.T) {
	t.Run("Figure 7. A blocking bug caused by wrong usage of channel with lock.", func(t *testing.T) {
		shouldNotTimeoutDelay(t, time.Second, func() {
			m := sync.Mutex{}
			ch := make(chan int)

			wg := sync.WaitGroup{}
			wg.Add(1)

			goroutine1 := func() {
				time.Sleep(10 * time.Millisecond)
				m.Lock()
				ch <- 42 //blocks
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

func TestFigure8(t *testing.T) {
	t.Run("Figure 8. A data race caused by anonymous function.", func(t *testing.T) {
		versionOf := func(i int) string { return fmt.Sprintf("v1.%d", i) }
		start := 17
		finish := 21
		count := finish - start + 1
		results := make(chan string, count)

		for i := start; i <= finish; i++ { // write
			go func() { /* Create a new goroutine */
				// loop variable i captured by func
				results <- versionOf(i) // read
			}()
		}

		versions := []string{}
		for v := range results {
			versions = append(versions, v)
			if len(versions) == count {
				break
			}
		}
		close(results)

		for i := start; i <= finish; i++ {
			assert.Contains(t, versions, versionOf(i))
		}
	})
}

func shouldNotTimeoutDelay(t *testing.T, delay time.Duration, f func()) {
	timeout := time.After(delay)

	done := make(chan bool, 1)

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

func shouldNotTimeout(t *testing.T, f func()) {
	shouldNotTimeoutDelay(t, 100*time.Millisecond, f)
}
