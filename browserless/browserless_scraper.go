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
	// Create a remote chromedp context with a 30-second timeout.
	ctx, cancel, err := NewRemoteContext("", 30*time.Second)
	if err != nil {
		return err
	}
	defer cancel()

	// Build the list of chromedp tasks.
	tasks := []chromedp.Action{
		// Navigate to the target URL.
		chromedp.Navigate(url),
	}
	// If a waitSelector is provided, wait until that element is visible.
	if waitSelector != "" {
		tasks = append(tasks, chromedp.WaitVisible(waitSelector, chromedp.ByQuery))
	}
	// Optionally wait for a fixed duration (useful if additional time is needed for JS to load).
	if sleepDuration > 0 {
		tasks = append(tasks, chromedp.Sleep(sleepDuration))
	}
	// Finally, evaluate the provided JavaScript snippet.
	tasks = append(tasks, chromedp.Evaluate(js, result))

	// Run all tasks sequentially.
	return chromedp.Run(ctx, tasks...)
}
