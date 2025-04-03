package grabber

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"

	"github.com/NorkzYT/comic-downloader/internal/http"
	"github.com/NorkzYT/comic-downloader/internal/logger"
)

// Mangadex is a grabber for mangadex.org
type Mangadex struct {
	*Grabber
	title string
}

// MangadexChapter represents a MangaDex Chapter
type MangadexChapter struct {
	Chapter
	Id string
}

// Test checks if the site is MangaDex
func (m *Mangadex) Test() (bool, error) {
	logger.Debug("Mangadex.Test: Checking if URL contains 'mangadex.org': %s", m.URL)
	re := regexp.MustCompile(`mangadex\.org`)
	return re.MatchString(m.URL), nil
}

// FetchTitle returns the title of the manga
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

	body := mangadexManga{}
	if err = json.NewDecoder(rbody).Decode(&body); err != nil {
		logger.Error("Mangadex.FetchTitle: Error decoding JSON: %v", err)
		return "", err
	}

	if m.Settings.Language != "" {
		trans := body.Data.Attributes.AltTitles.GetTitleByLang(m.Settings.Language)
		if trans != "" {
			m.title = trans
			logger.Debug("Mangadex.FetchTitle: Found translated title: %s", m.title)
			return m.title, nil
		}
	}

	m.title = body.Data.Attributes.Title["en"]
	logger.Debug("Mangadex.FetchTitle: Using English title: %s", m.title)
	return m.title, nil
}

// FetchChapters returns the chapters of the manga
func (m Mangadex) FetchChapters() (chapters Filterables, errs []error) {
	logger.Debug("Mangadex.FetchChapters: Fetching chapters for URL: %s", m.URL)
	id := getUuid(m.URL)

	baseOffset := 500
	var fetchChaps func(int)
	fetchChaps = func(offset int) {
		uri := fmt.Sprintf("https://api.mangadex.org/manga/%s/feed", id)
		params := url.Values{}
		params.Add("limit", fmt.Sprint(baseOffset))
		params.Add("order[volume]", "asc")
		params.Add("order[chapter]", "asc")
		params.Add("offset", fmt.Sprint(offset))
		if m.Settings.Language != "" {
			params.Add("translatedLanguage[]", m.Settings.Language)
		}
		uri = fmt.Sprintf("%s?%s", uri, params.Encode())
		logger.Debug("Mangadex.FetchChapters: Fetching chapters with offset %d from URI: %s", offset, uri)

		rbody, err := http.Get(http.RequestParams{URL: uri})
		if err != nil {
			logger.Error("Mangadex.FetchChapters: Error fetching chapters: %v", err)
			errs = append(errs, err)
			return
		}
		defer rbody.Close()
		body := mangadexFeed{}
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

		if len(body.Data) > 0 {
			fetchChaps(offset + baseOffset)
		}
	}
	fetchChaps(0)
	logger.Debug("Mangadex.FetchChapters: Total chapters fetched: %d", len(chapters))
	return
}

// FetchChapter fetches a chapter and its pages.
func (m Mangadex) FetchChapter(f Filterable) (*Chapter, error) {
	logger.Debug("Mangadex.FetchChapter: Fetching chapter...")
	chap := f.(*MangadexChapter)
	rbody, err := http.Get(http.RequestParams{
		URL: "https://api.mangadex.org/at-home/server/" + chap.Id,
	})
	if err != nil {
		logger.Error("Mangadex.FetchChapter: Error fetching chapter page: %v", err)
		return nil, err
	}
	body := mangadexPagesFeed{}
	if err = json.NewDecoder(rbody).Decode(&body); err != nil {
		logger.Error("Mangadex.FetchChapter: Error decoding pages JSON: %v", err)
		return nil, err
	}
	pcount := len(body.Chapter.Data)
	chapter := &Chapter{
		Title:      fmt.Sprintf("Chapter %04d %s", int64(f.GetNumber()), chap.Title),
		Number:     f.GetNumber(),
		PagesCount: int64(pcount),
		Language:   chap.Language,
	}
	for i, p := range body.Chapter.Data {
		num := i + 1
		pageURL := body.BaseUrl + path.Join("/data", body.Chapter.Hash, p)
		logger.Debug("Mangadex.FetchChapter: Adding page %d with URL: %s", num, pageURL)
		chapter.Pages = append(chapter.Pages, Page{
			Number: int64(num),
			URL:    pageURL,
		})
	}
	return chapter, nil
}

// mangadexManga represents the Manga JSON object.
type mangadexManga struct {
	Id   string
	Data struct {
		Attributes struct {
			Title     map[string]string
			AltTitles altTitles
		}
	}
}

// altTitles is a slice of maps with the language as key and the title as value.
type altTitles []map[string]string

// GetTitleByLang returns the title in the given language (or empty if not found).
func (a altTitles) GetTitleByLang(lang string) string {
	for _, t := range a {
		if val, ok := t[lang]; ok {
			return val
		}
	}
	return ""
}

// mangadexFeed represents the JSON object returned by the feed endpoint.
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

// mangadexPagesFeed represents the JSON object returned by the pages endpoint.
type mangadexPagesFeed struct {
	BaseUrl string
	Chapter struct {
		Hash      string
		Data      []string
		DataSaver []string
	}
}
