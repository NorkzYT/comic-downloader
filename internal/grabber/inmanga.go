package grabber

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"github.com/NorkzYT/comic-downloader/internal/http"
	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/PuerkitoBio/goquery"
)

// Inmanga is a grabber for inmanga.com
type Inmanga struct {
	*Grabber
	title string
}

// InmangaChapter is a chapter representation from InManga
type InmangaChapter struct {
	Chapter
	Id string
}

// Test checks if the site is Inmanga
func (i *Inmanga) Test() (bool, error) {
	logger.Debug("Inmanga.Test: Checking if URL contains 'inmanga.com': %s", i.URL)
	re := regexp.MustCompile(`inmanga\.com`)
	return re.MatchString(i.URL), nil
}

// FetchTitle fetches the manga title
func (i *Inmanga) FetchTitle() (string, error) {
	logger.Debug("Inmanga.FetchTitle: Starting for URL: %s", i.URL)
	if i.title != "" {
		logger.Debug("Inmanga.FetchTitle: Returning cached title: %s", i.title)
		return i.title, nil
	}

	body, err := http.Get(http.RequestParams{
		URL: i.URL,
	})
	if err != nil {
		logger.Error("Inmanga.FetchTitle: Error fetching URL %s: %v", i.URL, err)
		return "", err
	}
	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		logger.Error("Inmanga.FetchTitle: Error parsing document from URL %s: %v", i.URL, err)
		return "", err
	}

	i.title = doc.Find("h1").Text()
	logger.Debug("Inmanga.FetchTitle: Fetched title: %s", i.title)
	return i.title, nil
}

// FetchChapters returns the chapters of the manga
func (i Inmanga) FetchChapters() (Filterables, []error) {
	logger.Debug("Inmanga.FetchChapters: Fetching chapters for URL: %s", i.URL)
	id := getUuid(i.URL)

	// Retrieve chapters JSON list.
	body, err := http.GetText(http.RequestParams{
		URL: "https://inmanga.com/chapter/getall?mangaIdentification=" + id,
	})
	if err != nil {
		logger.Error("Inmanga.FetchChapters: Error fetching chapters JSON: %v", err)
		return nil, []error{err}
	}

	raw := struct {
		Data string
	}{}

	if err = json.Unmarshal([]byte(body), &raw); err != nil {
		logger.Error("Inmanga.FetchChapters: Error unmarshaling raw JSON: %v", err)
		return nil, []error{err}
	}

	feed := inmangaChapterFeed{}
	if err = json.Unmarshal([]byte(raw.Data), &feed); err != nil {
		logger.Error("Inmanga.FetchChapters: Error unmarshaling chapters feed: %v", err)
		return nil, []error{err}
	}

	chapters := make(Filterables, 0, len(feed.Result))
	for _, c := range feed.Result {
		chapters = append(chapters, newInmangaChapter(c))
	}
	logger.Debug("Inmanga.FetchChapters: Parsed %d chapters", len(feed.Result))
	return chapters, nil
}

// FetchChapter fetches the chapter with its pages
func (i Inmanga) FetchChapter(chap Filterable) (*Chapter, error) {
	ichap := chap.(*InmangaChapter)
	logger.Debug("Inmanga.FetchChapter: Fetching chapter with ID: %s", ichap.Id)
	body, err := http.Get(http.RequestParams{
		URL: "https://inmanga.com/chapter/chapterIndexControls?identification=" + ichap.Id,
	})
	if err != nil {
		logger.Error("Inmanga.FetchChapter: Error fetching chapter page: %v", err)
		return nil, err
	}
	defer body.Close()
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		logger.Error("Inmanga.FetchChapter: Error parsing chapter document: %v", err)
		return nil, err
	}

	chapter := &Chapter{
		Title:      chap.GetTitle(),
		Number:     chap.GetNumber(),
		PagesCount: int64(ichap.PagesCount),
		// Inmanga only hosts Spanish mangas.
		Language: "es",
	}

	// Get pages from select (skip duplicate).
	doc.Find("select.PageListClass").First().Children().Each(func(i int, s *goquery.Selection) {
		num, _ := strconv.ParseInt(s.Text(), 10, 64)
		pageURL := "https://pack-yak.intomanga.com/images/manga/ms/chapter/ch/page/p/" + s.AttrOr("value", "")
		logger.Debug("Inmanga.FetchChapter: Adding page %d with URL: %s", num, pageURL)
		chapter.Pages = append(chapter.Pages, Page{
			Number: num,
			URL:    pageURL,
		})
	})

	return chapter, nil
}

// newInmangaChapter creates an InmangaChapter from an InmangaChapterFeedResult.
func newInmangaChapter(c inmangaChapterFeedResult) *InmangaChapter {
	title := fmt.Sprintf("Capítulo %04d", int64(c.Number))
	logger.Debug("Inmanga.newInmangaChapter: Creating chapter %s with ID: %s", title, c.Id)
	return &InmangaChapter{
		Chapter: Chapter{
			Number:     c.Number,
			PagesCount: int64(c.PagesCount),
			Title:      title,
		},
		Id: c.Id,
	}
}

// inmangaChapterFeed is the JSON feed for the chapters list.
type inmangaChapterFeed struct {
	Result []inmangaChapterFeedResult
}

// inmangaChapterFeedResult is the JSON feed for a single chapter result.
type inmangaChapterFeedResult struct {
	Id         string `json:"identification"`
	Number     float64
	PagesCount float64
}
