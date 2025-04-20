package grabber

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NorkzYT/comic-downloader/internal/browserless"
	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/PuerkitoBio/goquery"
)

// Mangafire implements support for mangafire.to via browserless/chromedp.
type Mangafire struct {
	*Grabber
	title string
}

// Test checks if the URL belongs to mangafire.to.
func (m *Mangafire) Test() (bool, error) {
	logger.Debug("Mangafire.Test: Checking URL: %s", m.URL)
	return strings.Contains(m.URL, "mangafire.to"), nil
}

// UsesBrowser indicates that Mangafire requires a headless browser (for chapter pages).
func (m *Mangafire) UsesBrowser() bool {
	return true
}

// FetchTitle extracts the series title from <h1 itemprop="name"> without a browser.
func (m *Mangafire) FetchTitle() (string, error) {
	logger.Info("Mangafire.FetchTitle: Starting title fetch for %s", m.URL)

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(m.URL)
	if err != nil {
		logger.Error("Mangafire.FetchTitle: HTTP GET failed: %v", err)
		return "", fmt.Errorf("error fetching page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Mangafire.FetchTitle: unexpected status code %d", resp.StatusCode)
		return "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logger.Error("Mangafire.FetchTitle: parsing HTML failed: %v", err)
		return "", fmt.Errorf("error parsing HTML: %w", err)
	}

	title := strings.TrimSpace(
		doc.Find("div.info h1[itemprop='name']").First().Text(),
	)
	if title == "" {
		logger.Info("Mangafire.FetchTitle: could not find <h1[itemprop='name']>, falling back to <title>")
		title = strings.TrimSpace(doc.Find("title").Text())
		if title == "" {
			logger.Error("Mangafire.FetchTitle: both selectors returned empty")
			return "", fmt.Errorf("could not extract title")
		}
	}

	logger.Info("Mangafire.FetchTitle: Retrieved title '%s'", title)
	m.title = title
	return title, nil
}

// FetchChapters retrieves the list of chapters by scraping the static HTML.
func (m *Mangafire) FetchChapters() (Filterables, []error) {
	logger.Info("Mangafire.FetchChapters: Fetching chapters for %s", m.URL)

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(m.URL)
	if err != nil {
		logger.Error("Mangafire.FetchChapters: HTTP GET failed: %v", err)
		return nil, []error{fmt.Errorf("error fetching page: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Mangafire.FetchChapters: unexpected status code %d", resp.StatusCode)
		return nil, []error{fmt.Errorf("unexpected status code %d", resp.StatusCode)}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logger.Error("Mangafire.FetchChapters: parsing HTML failed: %v", err)
		return nil, []error{fmt.Errorf("error parsing HTML: %w", err)}
	}

	selector := "section.m-list div.tab-content[data-name='chapter'] ul.scroll-sm li.item"
	items := doc.Find(selector)
	if items.Length() == 0 {
		logger.Warn("Mangafire.FetchChapters: no chapters found with selector %s", selector)
	}

	var out Filterables
	items.Each(func(_ int, s *goquery.Selection) {
		a := s.Find("a")
		href, ok := a.Attr("href")
		if !ok {
			return
		}
		// make absolute if needed
		link := href
		if !strings.HasPrefix(href, "http") {
			link = m.BaseUrl() + href
		}

		// Chapter title
		title := strings.TrimSpace(a.Find("span").First().Text())

		// Chapter number
		numAttr, _ := s.Attr("data-number")
		number, _ := strconv.ParseFloat(numAttr, 64)

		out = append(out, &MangafireChapter{
			Chapter: Chapter{Title: title, Number: number},
			URL:     link,
		})
	})

	logger.Info("Mangafire.FetchChapters: Found %d chapters", len(out))
	return out, nil
}

// MangafireChapter represents a chapter on mangafire.to.
type MangafireChapter struct {
	Chapter
	URL string
}

// FetchChapter downloads one chapter using browserless to list image URLs.
func (m *Mangafire) FetchChapter(f Filterable) (*Chapter, error) {
	mc := f.(*MangafireChapter)
	logger.Info("Mangafire.FetchChapter: Downloading chapter %s", mc.Title)

	const js = `(function(){
		var imgs = document.querySelectorAll("div.pages.longstrip img");
		var srcs = [];
		imgs.forEach(function(img){
			if (img.src) srcs.push(img.src);
		});
		return JSON.stringify(srcs);
	})();`

	var raw string
	if err := browserless.RunJS(mc.URL, "div.pages.longstrip", 5*time.Second, js, &raw); err != nil {
		logger.Error("Mangafire.FetchChapter: JS image extraction failed: %v", err)
		return nil, fmt.Errorf("error fetching chapter pages: %w", err)
	}

	var imgs []string
	if err := json.Unmarshal([]byte(raw), &imgs); err != nil {
		logger.Error("Mangafire.FetchChapter: JSON unmarshal images failed: %v", err)
		return nil, fmt.Errorf("invalid image list: %w", err)
	}

	chap := &Chapter{Title: mc.Title, Number: mc.Number, PagesCount: int64(len(imgs)), Language: "en"}
	for i, src := range imgs {
		chap.Pages = append(chap.Pages, Page{Number: int64(i + 1), URL: src})
	}
	logger.Info("Mangafire.FetchChapter: Prepared %d pages", len(imgs))
	return chap, nil
}

// BaseUrl returns the site origin.
func (m *Mangafire) BaseUrl() string {
	u, _ := url.Parse(m.URL)
	return u.Scheme + "://" + u.Host
}
