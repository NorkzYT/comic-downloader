//go:build dumphtml
// +build dumphtml

package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// newRemoteContext creates a Chromedp context that connects to a remote Browserless instance.
// It uses the DEVTOOLS_WS_URL environment variable if available, otherwise it falls back
// to a default endpoint.
func newRemoteContext() (context.Context, context.CancelFunc, error) {
	devtoolsWsURL := os.Getenv("DEVTOOLS_WS_URL")
	if devtoolsWsURL == "" {
		// Change this default to your Browserless endpoint and token.
		devtoolsWsURL = "ws://localhost:8454?token=6R0W53R135510"
	}
	log.Printf("Connecting to Browserless at: %s\n", devtoolsWsURL)
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), devtoolsWsURL, chromedp.NoModifyURL)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	return ctx, func() {
		cancelCtx()
		cancelAlloc()
	}, nil
}

func main() {
	// Usage:
	// Chapter mode: go run -tags=dumphtml dumphtml.go <chapter_url> <output_file.html>
	// Series mode:  go run -tags=dumphtml dumphtml.go <series_url> <chapter_number>
	if len(os.Args) < 3 {
		log.Fatalf("Usage:\nChapter mode: %s <chapter_url> <output_file.html>\nSeries mode: %s <series_url> <chapter_number>", os.Args[0], os.Args[0])
	}
	urlArg := os.Args[1]

	// Create a new remote Chromedp context using Browserless.
	ctx, cancel, err := newRemoteContext()
	if err != nil {
		log.Fatalf("Error creating remote context: %v", err)
	}
	defer cancel()

	var chapterURL string
	var outputFile string

	// Determine if the provided URL is already a chapter URL.
	if strings.Contains(urlArg, "/chapter/") {
		// Chapter mode: use the provided URL and second argument as output file.
		chapterURL = urlArg
		outputFile = os.Args[2]
	} else {
		// Series mode: second argument is the chapter number.
		chapterNumber := os.Args[2]
		// Generate an output filename automatically.
		outputFile = "chapter_" + chapterNumber + ".html"

		// Navigate to the series page.
		var seriesHTML string
		err = chromedp.Run(ctx,
			chromedp.Navigate(urlArg),
			chromedp.WaitVisible("body", chromedp.ByQuery),
			// Wait extra seconds to ensure the chapter list loads.
			chromedp.Sleep(5*time.Second),
			chromedp.OuterHTML("html", &seriesHTML, chromedp.ByQuery),
		)
		if err != nil {
			log.Fatalf("Failed to fetch series page: %v", err)
		}

		// Use JavaScript to search for the chapter link by its number.
		// This snippet looks for an <a> element inside any container with class "overflow-y-auto"
		// whose text contains "Chapter <chapterNumber>".
		var chapterLink string
		js := `(function(){
			var links = document.querySelectorAll("div.overflow-y-auto a");
			for(var i = 0; i < links.length; i++){
				if(links[i].textContent.indexOf("Chapter ` + chapterNumber + `") !== -1){
					return links[i].getAttribute("href");
				}
			}
			return "";
		})();`
		err = chromedp.Run(ctx,
			chromedp.Evaluate(js, &chapterLink),
		)
		if err != nil {
			log.Fatalf("Failed to evaluate chapter link: %v", err)
		}
		if chapterLink == "" {
			log.Fatalf("Chapter %s not found on the series page.", chapterNumber)
		}

		// Fix the relative URL.
		if !strings.HasPrefix(chapterLink, "http") {
			// Ensure there is a leading slash.
			if chapterLink[0] != '/' {
				chapterLink = "/" + chapterLink
			}
			parsed, err := url.Parse(urlArg)
			if err != nil {
				log.Fatalf("Failed to parse series URL: %v", err)
			}
			// Special handling for asuracomic.net:
			if strings.Contains(parsed.Host, "asuracomic.net") {
				chapterLink = parsed.Scheme + "://" + parsed.Host + "/series" + chapterLink
			} else {
				chapterLink = parsed.Scheme + "://" + parsed.Host + chapterLink
			}
		}
		chapterURL = chapterLink
		log.Printf("Found chapter URL: %s\n", chapterURL)
	}

	// Now navigate to the chapter page.
	var chapterHTML string
	err = chromedp.Run(ctx,
		chromedp.Navigate(chapterURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second),
		chromedp.OuterHTML("html", &chapterHTML, chromedp.ByQuery),
	)
	if err != nil {
		log.Fatalf("Failed to fetch chapter page: %v", err)
	}

	// Save the chapter HTML.
	err = ioutil.WriteFile(outputFile, []byte(chapterHTML), 0644)
	if err != nil {
		log.Fatalf("Failed to write chapter HTML to file: %v", err)
	}
	log.Printf("Chapter HTML saved to %s\n", outputFile)

	// Now, scrape and download images from the chapter page.
	err = scrapeAndDownloadImages(ctx)
	if err != nil {
		log.Fatalf("Failed to scrape and download images: %v", err)
	}
}

// scrapeAndDownloadImages queries the current page for all images within
// <div class="w-full mx-auto center"> and downloads them.
func scrapeAndDownloadImages(ctx context.Context) error {
	var imageSrcs []string
	// Use JavaScript to extract all the src attributes from <img> elements inside the target div.
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`Array.from(document.querySelectorAll("div.w-full.mx-auto.center img")).map(img => img.src)`, &imageSrcs),
	)
	if err != nil {
		return err
	}
	if len(imageSrcs) == 0 {
		log.Println("No images found on the chapter page.")
		return nil
	}
	log.Printf("Found %d images. Starting download...", len(imageSrcs))
	for i, src := range imageSrcs {
		log.Printf("Downloading image %d: %s\n", i+1, src)
		err := downloadImage(src, i+1)
		if err != nil {
			log.Printf("Error downloading image %d: %v\n", i+1, err)
		}
	}
	return nil
}

// downloadImage downloads an image from the given URL and saves it to disk
// with a filename based on its index and file extension.
func downloadImage(urlStr string, index int) error {
	resp, err := http.Get(urlStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// Parse the URL to extract the file extension.
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	ext := path.Ext(parsedURL.Path)
	if ext == "" {
		ext = ".jpg" // default extension if none found
	}
	filename := "image_" + strconv.Itoa(index) + ext
	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}
	log.Printf("Image %d saved as %s\n", index, filename)
	return nil
}
