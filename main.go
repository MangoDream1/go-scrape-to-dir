package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MangoDream1/go-scraper"
	"github.com/caarlos0/env/v10"
)

type config struct {
	HtmlDir               string `env:"HTML_DIR"`
	StartUrl              string `env:"START_URL" envDefault:"en.wikipedia.org/wiki/United_Kingdom"`
	MaxConcurrentRequests int8   `env:"MAX_CONCURRENT_REQUESTS" envDefault:"-1"`
	AllowedHrefRegex      string `env:"ALLOWED_HREF_REGEX" envDefault:"en.wikipedia.org/wiki"`
}

func main() {
	c := config{}
	if err := env.Parse(&c); err != nil {
		panic(err)
	}

	s := scraper.Scraper{
		AllowedHrefRegex:      regexp.MustCompile(c.AllowedHrefRegex),
		AlreadyDownloaded:     c.doesHtmlExist,
		HasDownloaded:         func(href string) { c.save(href, strings.NewReader("tmp")) },
		MaxConcurrentRequests: c.MaxConcurrentRequests,
		StartUrl:              c.StartUrl,
	}

	o := make(chan scraper.Html)
	go s.Start(o)

	done := make(chan bool)

	for {
		select {
		case <-done:
			continue
		case html := <-o:
			go func() {
				c.save(html.Href, html.Body)
				done <- true
			}()
		}
	}
}

func (c *config) save(href string, blob io.Reader) {
	fileName := transformUrlIntoFilename(href)
	path := filepath.Join(c.HtmlDir, fileName)

	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("An error has occurred while trying to store an file with name: %v \n", path)
			panic(err)
		}
	}()

	fmt.Printf("Writing file to %v\n", path)

	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		panic(err)
	}

	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = io.Copy(f, blob)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Successfully written file to %v\n", path)
}

func (c *config) doesHtmlExist(href string) bool {
	fileName := transformUrlIntoFilename(href)
	path := filepath.Join(c.HtmlDir, fileName)
	return doesFileExist(path)
}

func transformUrlIntoFilename(href string) (fileName string) {
	fileName = href
	if fileName[len(fileName)-1] == '/' {
		fileName = fileName[0 : len(fileName)-1]
	}
	fileName = strings.Replace(fileName, "https://", "", 1)
	fileName = strings.Replace(fileName, "http://", "", 1)

	fileName = addExtension(fileName, "html")
	return
}

func addExtension(id string, extension string) string {
	return fmt.Sprintf("%s.%s", id, extension)
}

func doesFileExist(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
