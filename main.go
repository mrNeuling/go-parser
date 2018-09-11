package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const CACHE_DIR = "/go/src/parser/cache"
const DOMAIN = "https://irr.ru"

var useCache bool
var adsLimit uint

type Location struct {
	Lat, Lng float64
}

type Announcement struct {
	date time.Time
	title, content string
	location Location
}

func (a Announcement) String() string {
	return fmt.Sprintf("Date: %v\nTitle: %v\nLocation: %v\n", a.date.Format(time.RFC822), a.title, a.location)
}

func init() {
	flag.BoolVar(&useCache, "useCache", true, "Set false to do not use cache")
	flag.UintVar(&adsLimit, "limit", 50, "Maximum ads number")

	if _, err := os.Stat(CACHE_DIR); os.IsNotExist(err) {
		os.MkdirAll(CACHE_DIR, os.ModeDir)
	}
}

func main() {
	flag.Parse()

	var processedCount uint
	currentUrl := "/real-estate/rent/"
	for processedCount < adsLimit {
		content, err := getPage(DOMAIN + currentUrl, useCache)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		doc, err := goquery.NewDocumentFromReader(content)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		doc.Find(".listing .listing__item").EachWithBreak(func (index int, item *goquery.Selection) bool {
			if validateAdItem(item) {
				processAdItem(item, useCache)
				processedCount++
			}
			return processedCount < adsLimit
		})

		if processedCount < adsLimit {
			nextLinkWrapper := doc.Find(".pagination .pagination__pagesItem.pagination__pagesItem_active").Next()
			if nextLinkWrapper == nil {
				fmt.Fprintln(os.Stderr, "Cannot find next link")
				return
			}
			var exist bool
			currentUrl, exist = nextLinkWrapper.Find(".pagination__pagesLink").Attr("href")
			if !exist {
				fmt.Fprintln(os.Stderr, "Cannot find next page url")
				return
			}
		}
	}
}

func getPage(uri string, useCache bool) (io.ReadCloser, error) {
	if !useCache {
		fmt.Fprintln(os.Stderr, "No cache")
		response, err := http.Get(uri)
		if nil != err {
			return nil, err
		}
		return response.Body, nil
	}

	filename := CACHE_DIR + "/" + url.PathEscape(uri)
	if _, err := os.Stat(filename); nil == err {
		file, err := os.Open(filename)
		if nil != err {
			return nil, err
		}
		readCloser := io.ReadCloser(file)
		return readCloser, nil
	}

	response, err := http.Get(uri)
	if nil != err {
		return nil, err
	}

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0755)
	if nil != err {
		fmt.Fprintln(os.Stderr, "Cannot create file " + filename)
	} else {
		defer file.Close()
		io.Copy(file, response.Body)
	}

	return response.Body, nil
}

func validateAdItem(item *goquery.Selection) bool {
	return item.Find(".listing__itemTitle").Length() > 0
}

func processAdItem(item *goquery.Selection, useCache bool) {
	uri, exist := item.Find(".listing__itemTitle").Attr("href")
	if !exist {
		fmt.Fprintln(os.Stderr, "Cannot find announcement url")
		return
	}
	page, err := getPage(uri, useCache)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot download a page", err)
		return
	}
	doc, err := goquery.NewDocumentFromReader(page)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	title := trim(doc.Find(".productPage__title").Text())
	//content := trim(doc.Find(".productPage__description, .productPage__infoColumns").Text())
	address := trim(doc.Find(".productPage__infoBlock .productPage__infoTextBold").Text())
	loc, err := addressToLocation(address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	dateString := trim(doc.Find(".productPage__mainInfo .productPage__createDate").Text())
	date, err := createTimeFromString(dateString)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	announcement := Announcement{title:title/*, content: content*/, location: loc, date: date}
	fmt.Fprintln(os.Stderr, doc.Find(".productPage__title").Length(), announcement)
}

func trim(s string) string {
	return strings.Trim(s, "\n\r\t ")
}

func addressToLocation(address string) (Location, error) {
	// TODO - use Google Geocoder API
	return Location{Lat:55,Lng:35}, nil
}

func createTimeFromString(date string) (time.Time, error) {
	normalizedDate, err := normalizeDate(date)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse("_2 Jan 06 15:04 -0700", normalizedDate)
}

func normalizeDate(date string) (string, error) {
	russianMonthToEnglish := map[string]string{
		"января":"Jan",
		"февраля":"Feb",
		"марта":"Mar",
		"апреля":"Apr",
		"мая":"May",
		"июня":"Jun",
		"июля":"Jul",
		"августа":"Aug",
		"сентября":"Sep",
		"октября":"Oct",
		"ноября":"Nov",
		"декабря":"Dec",
	}
	content := []byte(date)
	todayPattern := regexp.MustCompile(`^\s*сегодня,\s+(?P<time>(?:[01][0-9]|2[0-3])\:(?:[0-5][0-9]))\s*$`)
	if todayPattern.Match(content) {
		timeIndexes := todayPattern.FindSubmatchIndex(content)
		return time.Now().Format("02 Jan 06 ") + string(content[timeIndexes[2]:timeIndexes[3]]) + " +0400", nil
	}

	datePattern := regexp.MustCompile(`^\s*(?P<day>(?:0?[1-9]|[1-2][1-9]|3[01]))\s+(?P<month>\p{Cyrillic}+)(?:\s+(?P<year>\d\d(?:\d\d)?))?\s*$`)
	dateIndexes := datePattern.FindSubmatchIndex(content)

	if len(dateIndexes) == 0 {
		return "", errors.New("Cannot normalize date string " + date)
	}
	dayIndexes := dateIndexes[2:4]
	monthIndexes := dateIndexes[4:6]

	currentTime := time.Now()
	year := strconv.Itoa(currentTime.Year())
	if len(dateIndexes) > 7 {
		yearIndexes := dateIndexes[6:8]
		if yearIndexes[0] != -1 && yearIndexes[1] != -1 {
			year = string(content[yearIndexes[0]:yearIndexes[1]])
		}
	}
	day := string(content[dayIndexes[0]:dayIndexes[1]])
	month := string(content[monthIndexes[0]:monthIndexes[1]])

	if _, ok := russianMonthToEnglish[month]; ok == false {
		return "", errors.New("Unknown month")
	}

	return day + " " + russianMonthToEnglish[month] + " " + year + " 00:00 +0400", nil
}