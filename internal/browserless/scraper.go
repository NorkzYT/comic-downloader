package browserless

import (
	"context"
	"fmt"
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
	// Load environment variables from .env file.
	if err := godotenv.Load(); err != nil {
		logger.Info("No .env file found, continuing with environment variables")
	}
}

// NewRemoteContext creates a new chromedp context by connecting to a remote Browserless instance.
// It requires that BROWSERLESS_TOKEN is set in your .env file.
// When DOCKER is set to "true", it connects to the docker container endpoint;
// otherwise, it connects to the host specified by BROWSERLESS_HOST_IP (default "localhost").
// This makes it flexible to work with hosts on different machines.
func NewRemoteContext(devtoolsWsURL string, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	// If no URL is provided, build one from the environment.
	if devtoolsWsURL == "" {
		token := os.Getenv("BROWSERLESS_TOKEN")
		if token == "" {
			logger.Error("BROWSERLESS_TOKEN must be set in .env")
			return nil, nil, fmt.Errorf("BROWSERLESS_TOKEN must be set in .env")
		}
		if os.Getenv("DOCKER") == "true" {
			// Use Docker container endpoint.
			devtoolsWsURL = fmt.Sprintf("ws://comic-downloader-browserless:3000?token=%s", token)
		} else {
			// Retrieve host IP from environment variable, default to "localhost".
			host := os.Getenv("BROWSERLESS_HOST_IP")
			if host == "" {
				host = "localhost"
			}
			devtoolsWsURL = fmt.Sprintf("ws://%s:8454?token=%s", host, token)
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
