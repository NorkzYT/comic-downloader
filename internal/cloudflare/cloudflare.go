package cloudflare

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/joho/godotenv"
)

var FASTAPIBaseURL string

func init() {
	_ = godotenv.Load()
	FASTAPIBaseURL = os.Getenv("FASTAPI_BASE_URL")
	if FASTAPIBaseURL == "" {
		FASTAPIBaseURL = "http://localhost:6081"
	}
}

// TriggerCloudflare primes FastAPI’s /trigger endpoint for the given URL.
func TriggerCloudflare(seriesURL string, sleepMs int) error {
	endpoint := FASTAPIBaseURL + "/trigger"
	values := url.Values{
		"url":   {seriesURL},
		"js":    {""},
		"wait":  {""},
		"sleep": {fmt.Sprint(sleepMs)},
	}
	full := endpoint + "?" + values.Encode()
	logger.Debug("cloudflare.Trigger → %s", full)

	resp, err := http.Get(full)
	if err != nil {
		return fmt.Errorf("trigger GET failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("trigger non-200 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	// give FastAPI a moment to settle cookies
	time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	return nil
}

// SaveImage calls FastAPI’s /save_image to persist a single image URL.
func SaveImage(chapterURL, imageURL string) error {
	endpoint := FASTAPIBaseURL + "/save_image"
	values := url.Values{
		"chapter_url": {chapterURL},
		"image_url":   {imageURL},
	}
	full := endpoint + "?" + values.Encode()
	logger.Debug("cloudflare.SaveImage → %s", full)

	resp, err := http.Get(full)
	if err != nil {
		return fmt.Errorf("save_image GET failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("save_image non-200 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// GetSavedImages calls FastAPI’s /get_image (no filename) and returns the JSON list.
func GetSavedImages(chapterURL string) ([]string, error) {
	u, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("invalid chapter URL: %w", err)
	}
	folder := path.Base(u.Path)

	endpoint := FASTAPIBaseURL + "/get_image"
	values := url.Values{"chapter": {folder}}
	full := endpoint + "?" + values.Encode()
	logger.Debug("cloudflare.GetSavedImages → %s", full)

	resp, err := http.Get(full)
	if err != nil {
		return nil, fmt.Errorf("get_image GET failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get_image non-200 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var payload struct {
		Chapter string   `json:"chapter"`
		Images  []string `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid get_image JSON: %w", err)
	}

	// ————————————————
	// Sort payload.Images by the numeric page prefix
	sort.Slice(payload.Images, func(i, j int) bool {
		aParts := strings.SplitN(payload.Images[i], "-", 2)
		bParts := strings.SplitN(payload.Images[j], "-", 2)
		ai, _ := strconv.Atoi(aParts[0])
		bi, _ := strconv.Atoi(bParts[0])
		return ai < bi
	})
	// ————————————————

	return payload.Images, nil
}
