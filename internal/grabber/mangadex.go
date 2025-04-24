package grabber

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/time/rate"

	"github.com/NorkzYT/comic-downloader/internal/http"
	"github.com/NorkzYT/comic-downloader/internal/logger"
)

// atHomeLimiter restricts requests to Mangadex’s /at-home endpoint to 39 per minute.
// See https://api.mangadex.org/docs/2-limitations#endpoint-specific-rate-limits
var atHomeLimiter = rate.NewLimiter(rate.Every(time.Minute/39), 1)

// Mangadex is a grabber for mangadex.org.
type Mangadex struct {
	*Grabber
	title string
}

// MangadexChapter represents a MangaDex Chapter.
type MangadexChapter struct {
	Chapter
	Id string
}

// Test checks if the site is MangaDex.
func (m *Mangadex) Test() (bool, error) {
	logger.Debug("Mangadex.Test: Checking if URL contains 'mangadex.org': %s", m.URL)
	re := regexp.MustCompile(`mangadex\.org`)
	return re.MatchString(m.URL), nil
}

// FetchTitle returns the title of the manga (cached after the first call).
func (m *Mangadex) FetchTitle() (string, error) {
	logger.Debug("Mangadex.FetchTitle: Starting for URL: %s", m.URL)
	if m.title != "" {
		logger.Debug("Mangadex.FetchTitle: Returning cached title: %s", m.title)
		return m.title, nil
	}

	id := getUuid(m.URL)

	rbody, err := http.Get(http.RequestParams{
		URL:     "https://api.mangadex.org/manga/" + id,
		Referer: m.BaseUrl(),
	})
	if err != nil {
		logger.Error("Mangadex.FetchTitle: Error fetching manga data: %v", err)
		return "", err
	}
	defer rbody.Close()

	var body mangadexManga
	if err = json.NewDecoder(rbody).Decode(&body); err != nil {
		logger.Error("Mangadex.FetchTitle: Error decoding JSON: %v", err)
		return "", err
	}

	// If user requested a specific language, try the alt titles first.
	if m.Settings.Language != "" {
		if trans := body.Data.Attributes.AltTitles.GetTitleByLang(m.Settings.Language); trans != "" {
			m.title = trans
			logger.Debug("Mangadex.FetchTitle: Found translated title: %s", m.title)
			return m.title, nil
		}
	}

	// Fallback to English
	m.title = body.Data.Attributes.Title["en"]
	logger.Debug("Mangadex.FetchTitle: Using English title: %s", m.title)
	return m.title, nil
}

// FetchChapters returns all chapters in ascending order.
func (m Mangadex) FetchChapters() (chapters Filterables, errs []error) {
	logger.Debug("Mangadex.FetchChapters: Fetching chapters for URL: %s", m.URL)
	id := getUuid(m.URL)

	const pageSize = 500
	var fetchBatch func(offset int)
	fetchBatch = func(offset int) {
		uri := fmt.Sprintf("https://api.mangadex.org/manga/%s/feed", id)
		params := url.Values{}
		params.Add("limit", strconv.Itoa(pageSize))
		params.Add("order[volume]", "asc")
		params.Add("order[chapter]", "asc")
		params.Add("offset", strconv.Itoa(offset))
		if m.Settings.Language != "" {
			params.Add("translatedLanguage[]", m.Settings.Language)
		}
		uri = uri + "?" + params.Encode()
		logger.Debug("Mangadex.FetchChapters: GET %s", uri)

		rbody, err := http.Get(http.RequestParams{URL: uri})
		if err != nil {
			logger.Error("Mangadex.FetchChapters: Error fetching chapters: %v", err)
			errs = append(errs, err)
			return
		}
		defer rbody.Close()

		var body mangadexFeed
		if err = json.NewDecoder(rbody).Decode(&body); err != nil {
			logger.Error("Mangadex.FetchChapters: Error decoding JSON: %v", err)
			errs = append(errs, err)
			return
		}

		for _, c := range body.Data {
			num, _ := strconv.ParseFloat(c.Attributes.Chapter, 64)
			chapters = append(chapters, &MangadexChapter{
				Chapter: Chapter{
					Number:     num,
					Title:      c.Attributes.Title,
					Language:   c.Attributes.TranslatedLanguage,
					PagesCount: c.Attributes.Pages,
				},
				Id: c.Id,
			})
			logger.Debug("Mangadex.FetchChapters: Added chapter: %s", c.Attributes.Title)
		}

		if len(body.Data) == pageSize {
			fetchBatch(offset + pageSize)
		}
	}

	fetchBatch(0)
	logger.Debug("Mangadex.FetchChapters: Total chapters fetched: %d", len(chapters))
	return
}

// FetchChapter fetches a chapter’s pages, respecting the 39/minute rate limit.
func (m Mangadex) FetchChapter(f Filterable) (*Chapter, error) {
	logger.Debug("Mangadex.FetchChapter: Waiting for rate limiter")
	if err := atHomeLimiter.Wait(context.Background()); err != nil {
		logger.Error("Mangadex.FetchChapter: Rate limiter error: %v", err)
		return nil, err
	}

	chap := f.(*MangadexChapter)
	logger.Debug("Mangadex.FetchChapter: Fetching pages for chapter %v (ID=%s)", chap.Number, chap.Id)

	rbody, err := http.Get(http.RequestParams{
		URL: "https://api.mangadex.org/at-home/server/" + chap.Id,
	})
	if err != nil {
		logger.Error("Mangadex.FetchChapter: Error fetching chapter page: %v", err)
		return nil, err
	}
	defer rbody.Close()

	var body mangadexPagesFeed
	if err = json.NewDecoder(rbody).Decode(&body); err != nil {
		logger.Error("Mangadex.FetchChapter: Error decoding pages JSON: %v", err)
		return nil, err
	}

	pageCount := len(body.Chapter.Data)
	chapter := &Chapter{
		Title:      fmt.Sprintf("Chapter %04d %s", int64(chap.Number), chap.Title),
		Number:     chap.Number,
		PagesCount: int64(pageCount),
		Language:   chap.Language,
	}

	for i, p := range body.Chapter.Data {
		url := body.BaseUrl + path.Join("/data", body.Chapter.Hash, p)
		logger.Debug("Mangadex.FetchChapter: Adding page %d URL=%s", i+1, url)
		chapter.Pages = append(chapter.Pages, Page{
			Number: int64(i + 1),
			URL:    url,
		})
	}

	return chapter, nil
}

// --- JSON helper types ---

type mangadexManga struct {
	Data struct {
		Attributes struct {
			Title     map[string]string
			AltTitles altTitles
		}
	}
}

type altTitles []map[string]string

func (a altTitles) GetTitleByLang(lang string) string {
	for _, t := range a {
		if v, ok := t[lang]; ok {
			return v
		}
	}
	return ""
}

type mangadexFeed struct {
	Data []struct {
		Id         string
		Attributes struct {
			Volume             string
			Chapter            string
			Title              string
			TranslatedLanguage string
			Pages              int64
		}
	}
}

type mangadexPagesFeed struct {
	BaseUrl string
	Chapter struct {
		Hash string
		Data []string
	}
}
