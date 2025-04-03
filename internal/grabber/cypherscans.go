package grabber

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NorkzYT/comic-downloader/internal/browserless"
	"github.com/NorkzYT/comic-downloader/internal/http"
	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/PuerkitoBio/goquery"
)

// CypherScans implements the Site interface for cypheroscans.xyz.
type CypherScans struct {
	*Grabber
	title string
}

// Test checks if the URL belongs to cypheroscans.xyz.
func (c *CypherScans) Test() (bool, error) {
	logger.Debug("CypherScans.Test: Checking if URL contains 'cypheroscans.xyz': %s", c.URL)
	return strings.Contains(c.URL, "cypheroscans.xyz"), nil
}

func (a *CypherScans) UsesBrowser() bool {
	return true
}

// FetchTitle retrieves the comic title by parsing the HTML content.
func (c *CypherScans) FetchTitle() (string, error) {
	logger.Debug("CypherScans.FetchTitle: Fetching title from URL: %s", c.URL)
	if c.title != "" {
		logger.Debug("CypherScans.FetchTitle: Returning cached title: %s", c.title)
		return c.title, nil
	}

	// Get the main page HTML.
	body, err := http.Get(http.RequestParams{URL: c.URL})
	if err != nil {
		logger.Error("CypherScans.FetchTitle: Error fetching URL %s: %v", c.URL, err)
		return "", err
	}
	defer body.Close()

	// Parse the HTML document.
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		logger.Error("CypherScans.FetchTitle: Error parsing document: %v", err)
		return "", err
	}

	// Extract title from: <div id="titledesktop"><div id="titlemove"><h1 class="entry-title" ...>...</h1>
	c.title = strings.TrimSpace(doc.Find("div#titledesktop h1.entry-title").Text())
	logger.Debug("CypherScans.FetchTitle: Fetched title: %s", c.title)
	return c.title, nil
}

// CypherScansChapter represents a chapter from cypheroscans.xyz.
type CypherScansChapter struct {
	Chapter
	URL string
}

// newCypherScansChapter creates a new CypherScansChapter instance.
func newCypherScansChapter(num float64, title, url string) *CypherScansChapter {
	logger.Debug("newCypherScansChapter: Creating chapter %s with URL: %s", title, url)
	return &CypherScansChapter{
		Chapter: Chapter{
			Number: num,
			Title:  title,
		},
		URL: url,
	}
}

// FetchChapters retrieves the list of chapters by parsing the chapter list HTML.
func (c *CypherScans) FetchChapters() (Filterables, []error) {
	logger.Debug("CypherScans.FetchChapters: Fetching chapters from URL: %s", c.URL)
	body, err := http.Get(http.RequestParams{URL: c.URL})
	if err != nil {
		logger.Error("CypherScans.FetchChapters: Error fetching URL %s: %v", c.URL, err)
		return nil, []error{err}
	}
	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		logger.Error("CypherScans.FetchChapters: Error parsing document: %v", err)
		return nil, []error{err}
	}

	chapters := make(Filterables, 0)

	// The chapter list is inside <div class="eplister" id="chapterlist"><ul>...
	doc.Find("div.eplister#chapterlist ul li").Each(func(i int, s *goquery.Selection) {
		// Locate the anchor tag.
		link := s.Find("a")
		href, exists := link.Attr("href")
		if !exists || strings.TrimSpace(href) == "" {
			logger.Debug("CypherScans.FetchChapters: Skipping chapter without link")
			return
		}

		// Extract chapter number text from <span class="chapternum">Chapter 681</span>.
		chapText := strings.TrimSpace(link.Find("span.chapternum").Text())
		if chapText == "" {
			logger.Debug("CypherScans.FetchChapters: Skipping chapter with empty chapter text")
			return
		}

		// Remove the "Chapter" prefix and trim to get the numeric part.
		numberStr := strings.TrimSpace(strings.TrimPrefix(chapText, "Chapter"))
		num, err := strconv.ParseFloat(numberStr, 64)
		if err != nil {
			logger.Error("CypherScans.FetchChapters: Error parsing chapter number from '%s': %v", chapText, err)
			num = 0
		}

		// Use the chapter text as the chapter title.
		chapterTitle := chapText

		// Create and append a new chapter.
		chapter := newCypherScansChapter(num, chapterTitle, href)
		chapters = append(chapters, chapter)
	})
	logger.Debug("CypherScans.FetchChapters: Parsed %d chapters", len(chapters))
	return chapters, nil
}

// FetchChapter downloads the chapter page using a headless browser to render dynamic content
// and extracts all image URLs as pages.
func (c *CypherScans) FetchChapter(f Filterable) (*Chapter, error) {
	csc, ok := f.(*CypherScansChapter)
	if !ok {
		return nil, fmt.Errorf("CypherScans.FetchChapter: invalid chapter type")
	}
	logger.Debug("CypherScans.FetchChapter: Fetching chapter from URL: %s", csc.URL)

	// Use browserless.RunJS to get the fully rendered HTML of the chapter page.
	var renderedHTML string
	// Here we wait for the "div#readerarea" element to be visible, allowing lazy-loaded images to load.
	err := browserless.RunJS(csc.URL, "div#readerarea", 5*time.Second, "document.documentElement.outerHTML", &renderedHTML)
	if err != nil {
		logger.Error("CypherScans.FetchChapter: Error rendering chapter page: %v", err)
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(renderedHTML))
	if err != nil {
		logger.Error("CypherScans.FetchChapter: Error parsing rendered HTML: %v", err)
		return nil, err
	}

	chapter := &Chapter{
		Title:    csc.Title,
		Number:   csc.Number,
		Language: "en",
	}

	// Extract image URLs from <div id="readerarea"> and all <img class="ts-main-image">.
	doc.Find("div#readerarea img.ts-main-image").Each(func(i int, s *goquery.Selection) {
		src := strings.TrimSpace(s.AttrOr("src", ""))
		if src != "" {
			chapter.Pages = append(chapter.Pages, Page{
				Number: int64(i + 1),
				URL:    src,
			})
		}
	})

	chapter.PagesCount = int64(len(chapter.Pages))
	if chapter.PagesCount == 0 {
		return nil, fmt.Errorf("CypherScans.FetchChapter: no images found on chapter page")
	}
	logger.Debug("CypherScans.FetchChapter: Fetched %d pages", chapter.PagesCount)
	return chapter, nil
}

// BaseUrl returns the base URL of the website.
func (c *CypherScans) BaseUrl() string {
	u, err := url.Parse(c.URL)
	if err != nil {
		logger.Error("CypherScans.BaseUrl: Error parsing URL: %v", err)
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// GetFilenameTemplate returns the filename template from settings.
func (c *CypherScans) GetFilenameTemplate() string {
	return c.Settings.FilenameTemplate
}

// GetMaxConcurrency returns the maximum concurrency settings.
func (c *CypherScans) GetMaxConcurrency() MaxConcurrency {
	return c.Settings.MaxConcurrency
}

// GetPreferredLanguage returns the preferred language.
func (c *CypherScans) GetPreferredLanguage() string {
	return c.Settings.Language
}
