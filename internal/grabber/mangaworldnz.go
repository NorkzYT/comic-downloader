package grabber

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/NorkzYT/comic-downloader/internal/http"
	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/PuerkitoBio/goquery"
)

// MangaWorldNZ is a grabber implementation for www.mangaworld.nz.
type MangaWorldNZ struct {
	*Grabber
	title string
}

// MangaWorldChapter represents a single chapter on MangaWorldNZ.
type MangaWorldChapter struct {
	Chapter
	URL string
}

// Test checks if the URL belongs to mangaworld.nz.
func (m *MangaWorldNZ) Test() (bool, error) {
	return strings.Contains(m.URL, "mangaworld.nz"), nil
}

// UsesBrowser indicates that no headless browser is required.
func (m *MangaWorldNZ) UsesBrowser() bool {
	return false
}

// FetchTitle retrieves (and caches) the manga title from the <h1 class="name bigger"> element.
func (m *MangaWorldNZ) FetchTitle() (string, error) {
	if m.title != "" {
		return m.title, nil
	}

	resp, err := http.Get(http.RequestParams{URL: m.URL})
	if err != nil {
		logger.Error("MangaWorldNZ.FetchTitle: error fetching URL %s: %v", m.URL, err)
		return "", err
	}
	defer resp.Close()

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		logger.Error("MangaWorldNZ.FetchTitle: error parsing HTML: %v", err)
		return "", err
	}

	// Selector matches: <h1 class="name bigger">Pick Me Up!</h1>
	m.title = strings.TrimSpace(doc.Find("h1.name.bigger").Text())
	logger.Debug("MangaWorldNZ.FetchTitle → %s", m.title)
	return m.title, nil
}

// FetchChapters parses all chapters under either the flat list or the per-volume lists.
func (m *MangaWorldNZ) FetchChapters() (Filterables, []error) {
	resp, err := http.Get(http.RequestParams{URL: m.URL})
	if err != nil {
		logger.Error("MangaWorldNZ.FetchChapters: error fetching URL %s: %v", m.URL, err)
		return nil, []error{err}
	}
	defer resp.Close()

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		logger.Error("MangaWorldNZ.FetchChapters: error parsing HTML: %v", err)
		return nil, []error{err}
	}

	// Try the flat list first:
	sel := doc.Find("div.chapters-wrapper div.chapter")
	// If none there (e.g. this series uses volumes), fall back to the volume sections:
	if sel.Length() == 0 {
		sel = doc.Find("div.volume-chapters div.chapter")
	}

	var chapters Filterables
	sel.Each(func(_ int, s *goquery.Selection) {
		a := s.Find("a.chap")
		href, ok := a.Attr("href")
		if !ok || href == "" {
			return
		}

		// Ensure ?style=list is on every URL
		if u, err := url.Parse(href); err == nil {
			q := u.Query()
			q.Set("style", "list")
			u.RawQuery = q.Encode()
			href = u.String()
		}

		// Full title comes from the title attribute
		fullTitle, _ := a.Attr("title")

		// Chapter number from the <span class="d-inline-block">Capitolo XX</span>
		span := strings.TrimSpace(a.Find("span.d-inline-block").Text())
		numStr := strings.TrimPrefix(span, "Capitolo ")
		number, _ := strconv.ParseFloat(strings.TrimSpace(numStr), 64)

		chapters = append(chapters, &MangaWorldChapter{
			Chapter: Chapter{Title: fullTitle, Number: number},
			URL:     href,
		})
		logger.Debug("MangaWorldNZ.FetchChapters: found chapter %q (#%.1f)", fullTitle, number)
	})

	return chapters, nil
}

// FetchChapter downloads all page image URLs for a given chapter.
func (m *MangaWorldNZ) FetchChapter(f Filterable) (*Chapter, error) {
	mwc, ok := f.(*MangaWorldChapter)
	if !ok {
		return nil, fmt.Errorf("MangaWorldNZ.FetchChapter: invalid chapter type")
	}

	resp, err := http.Get(http.RequestParams{URL: mwc.URL})
	if err != nil {
		logger.Error("MangaWorldNZ.FetchChapter: error fetching URL %s: %v", mwc.URL, err)
		return nil, err
	}
	defer resp.Close()

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		logger.Error("MangaWorldNZ.FetchChapter: error parsing HTML: %v", err)
		return nil, err
	}

	chapter := &Chapter{
		Title:    mwc.Title,
		Number:   mwc.Number,
		Language: "it", // content is Italian
	}

	// Images under: <div id="page"><img class="page-image" src="..."></div>
	doc.Find("div#page img.page-image").Each(func(i int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok && src != "" {
			chapter.Pages = append(chapter.Pages, Page{
				Number: int64(i + 1),
				URL:    src,
			})
		}
	})

	chapter.PagesCount = int64(len(chapter.Pages))
	logger.Debug("MangaWorldNZ.FetchChapter: fetched %d pages", chapter.PagesCount)
	return chapter, nil
}

// BaseUrl returns the site’s scheme+host (e.g. "https://www.mangaworld.nz").
func (m *MangaWorldNZ) BaseUrl() string {
	u, _ := url.Parse(m.URL)
	return u.Scheme + "://" + u.Host
}
