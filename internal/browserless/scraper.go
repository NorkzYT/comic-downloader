package browserless

import (
	"context"
	"os"
	"time"

	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/chromedp/chromedp"
	"github.com/joho/godotenv"
)

type BrowserlessUser interface {
	UsesBrowser() bool
}

func init() {
	if err := godotenv.Load(); err != nil {
		logger.Error("browserless: No .env file found or error loading .env: %v", err)
	}
}

// NewRemoteContext creates a new chromedp context by connecting to a remote Browserless instance.
func NewRemoteContext(devtoolsWsURL string, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if devtoolsWsURL == "" {
		if envURL := os.Getenv("BROWSERLESS_URL"); envURL != "" {
			devtoolsWsURL = envURL
		} else {
			devtoolsWsURL = "ws://localhost:8454?token=6R0W53R135510"
		}
	}
	parentCtx, cancelParent := context.WithTimeout(context.Background(), timeout)
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(parentCtx, devtoolsWsURL, chromedp.NoModifyURL)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	cancel := func() {
		cancelCtx()
		cancelAlloc()
		cancelParent()
	}

	logger.Debug("browserless.NewRemoteContext: Created new remote context with URL: %s", devtoolsWsURL)
	return ctx, cancel, nil
}

// RunJS navigates to the given URL, optionally waits for a CSS selector to be visible,
// sleeps for the specified duration (if any), and then evaluates the provided JavaScript snippet.
func RunJS(url string, waitSelector string, sleepDuration time.Duration, js string, result interface{}) error {
	ctx, cancel, err := NewRemoteContext("", 30*time.Second)
	if err != nil {
		logger.Error("browserless.RunJS: Error creating remote context: %v", err)
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
	logger.Debug("browserless.RunJS: Executing JS on URL: %s", url)
	return chromedp.Run(ctx, tasks...)
}

// FetchStringWithProgress wraps a RunJS call that returns a string.
func FetchStringWithProgress(url, waitSelector, js string, timeout time.Duration, progressCallback func()) (string, error) {
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

	var res string
	err := RunJS(url, waitSelector, timeout, js, &res)
	close(done)
	if err != nil {
		logger.Error("browserless.FetchStringWithProgress: Error: %v", err)
	} else {
		logger.Debug("browserless.FetchStringWithProgress: Successfully fetched string result.")
	}
	return res, err
}

// FetchStringSliceWithProgress wraps a RunJS call that returns a []string.
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
	if err != nil {
		logger.Error("browserless.FetchStringSliceWithProgress: Error: %v", err)
	} else {
		logger.Debug("browserless.FetchStringSliceWithProgress: Successfully fetched string slice result.")
	}
	return res, err
}
