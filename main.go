package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	//"github.com/PuerkitoBio/goquery"
)

const CACHE_DIR = "/go/src/parser/cache"

var useCache bool
var adsLimit int

func init() {
	flag.BoolVar(&useCache, "useCache", true, "Set false to do not use cache")
	flag.IntVar(&adsLimit, "limit", 50, "Maximum ads number")

	if _, err := os.Stat(CACHE_DIR); os.IsNotExist(err) {
		os.MkdirAll(CACHE_DIR, os.ModeDir)
	}
}

func main() {
	flag.Parse()

	t1 := time.Now()
	getPage("https://irr.ru/real-estate/rent/", useCache)
	fmt.Fprintln(os.Stderr, time.Now().Sub(t1))
	fmt.Fprintln(os.Stderr, adsLimit)
}

func getPage(uri string, useCache bool) (*io.ReadCloser, error) {
	if !useCache {
		fmt.Fprintln(os.Stderr, "No cache")
		response, err := http.Get(uri)
		if nil != err {
			return nil, err
		}
		return &response.Body, nil
	}

	filename := CACHE_DIR + "/" + url.PathEscape(uri)
	if _, err := os.Stat(filename); nil == err {
		file, err := os.Open(filename)
		if nil != err {
			return nil, err
		}
		readCloser := io.ReadCloser(file)
		return &readCloser, nil
	}

	response, err := http.Get(uri)
	if nil != err {
		return nil, err
	}
	defer response.Body.Close()

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0755)
	if nil != err {
		fmt.Fprintln(os.Stderr, "Cannot create file " + filename)
	} else {
		defer file.Close()
		io.Copy(file, response.Body)
	}

	return &response.Body, nil
}