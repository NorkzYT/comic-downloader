// Package browserless provides generic functionality to perform
// JavaScript evaluations on remote pages via chromedp (Browserless).
package browserless

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

// NewRemoteContext creates a new chromedp context by connecting to a remote Browserless instance.
// If devtoolsWsURL is empty, a default endpoint is used.
// The timeout parameter defines how long the entire operation may take.
func NewRemoteContext(devtoolsWsURL string, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	// Use default endpoint if none provided.
	if devtoolsWsURL == "" {
		devtoolsWsURL = "ws://localhost:8454?token=6R0W53R135510"
	}
	// Create a parent context with a timeout.
	parentCtx, cancelParent := context.WithTimeout(context.Background(), timeout)
	// Create a remote allocator with the given devtools endpoint.
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(parentCtx, devtoolsWsURL, chromedp.NoModifyURL)
	// Create a new chromedp context.
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	// Combine cancel functions so that all resources are cleaned up.
	cancel := func() {
		cancelCtx()
		cancelAlloc()
		cancelParent()
	}

	return ctx, cancel, nil
}

// RunJS navigates to the given URL, optionally waits for a CSS selector to be visible,
// sleeps for the specified duration (if any), and then evaluates the provided JavaScript snippet.
// The result of the evaluation is stored in the 'result' pointer.
func RunJS(url string, waitSelector string, sleepDuration time.Duration, js string, result interface{}) error {
	ctx, cancel, err := NewRemoteContext("", 30*time.Second)
	if err != nil {
		return err
	}
	defer cancel()
	tasks := []chromedp.Action{
		chromedp.Navigate(url),
	}
	if waitSelector != "" {
		tasks = append(tasks, chromedp.WaitVisible(waitSelector, chromedp.ByQuery))
	}
	if sleepDuration > 0 {
		tasks = append(tasks, chromedp.Sleep(sleepDuration))
	}
	tasks = append(tasks, chromedp.Evaluate(js, result))
	return chromedp.Run(ctx, tasks...)
}

// FetchStringWithProgress wraps a RunJS call that returns a string.
// It starts a separate goroutine that calls progressCallback every 250ms until the JS call completes.
func FetchStringWithProgress(url, waitSelector, js string, timeout time.Duration, progressCallback func()) (string, error) {
	done := make(chan struct{})
	// Start ticker goroutine that calls the callback every 250ms.
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if progressCallback != nil {
					progressCallback()
				}
			case <-done:
				return
			}
		}
	}()

	var res string
	err := RunJS(url, waitSelector, timeout, js, &res)
	close(done)
	return res, err
}

// FetchStringSliceWithProgress wraps a RunJS call that returns a []string.
// It starts a separate goroutine that calls progressCallback every 250ms until the JS call completes.
func FetchStringSliceWithProgress(url, waitSelector, js string, timeout time.Duration, progressCallback func()) ([]string, error) {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if progressCallback != nil {
					progressCallback()
				}
			case <-done:
				return
			}
		}
	}()

	var res []string
	err := RunJS(url, waitSelector, timeout, js, &res)
	close(done)
	return res, err
}
