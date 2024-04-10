package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/MangoDream1/go-scraper"
	"github.com/caarlos0/env/v10"
)

type config struct {
	HtmlDir               *string `env:"HTML_DIR"`
	StartUrl              string  `env:"START_URL" envDefault:"en.wikipedia.org/wiki/United_Kingdom"`
	MaxConcurrentRequests int8    `env:"MAX_CONCURRENT_REQUESTS" envDefault:"-1"`
	AllowedHrefRegex      string  `env:"ALLOWED_HREF_REGEX" envDefault:"en.wikipedia.org/wiki"`
	BlockedHrefRegex      *string `env:"BLOCKED_HREF_REGEX"`
}

func main() {
	c := config{}
	if err := env.Parse(&c); err != nil {
		panic(err)
	}

	if c.HtmlDir == nil {
		panic("HTML_DIR is required")
	}

	var blockHrefRegex *regexp.Regexp
	if c.BlockedHrefRegex != nil {
		blockHrefRegex = regexp.MustCompile(*c.BlockedHrefRegex)
	}

	tmpBytes := []byte("tmp")

	s := scraper.NewScraper(scraper.Options{
		AllowedHrefRegex:      regexp.MustCompile(c.AllowedHrefRegex),
		BlockedHrefRegex:      blockHrefRegex,
		AlreadyDownloaded:     c.doesHtmlExist,
		HasDownloaded:         func(href string) { c.save(href, strings.NewReader(string(tmpBytes))) },
		MaxConcurrentRequests: c.MaxConcurrentRequests,
		StartUrl:              c.StartUrl,
	})

	parseFile := func(path string) {
		f, err := readFile(path)
		if err != nil {
			fmt.Printf("An error has occurred while trying to read file with name: %v \n", path)
			return
		}
		defer f.Close()

		bodyW := new(bytes.Buffer)
		bodyT := io.TeeReader(f, bodyW)

		bytesT, err := io.ReadAll(bodyT)
		if err != nil {
			fmt.Printf("An error has occurred while trying read file with name: %v \n", path)
			return
		}

		skip := len(bytesT) == 0 || bytes.Equal(bytesT, tmpBytes)
		if skip {
			removeFile(path)
			fmt.Printf("File with name: %v was temporary and is now removed\n", path)
			return
		}

		parentHref := pathToUrl(path, *c.HtmlDir)

		o := make(chan string)
		err = scraper.ParseHtml(parentHref, bodyW, o)

		for href := range o {
			s.AddHref(href)
		}

		if err != nil {
			fmt.Printf("An error has occurred while trying to parse file with name: %v \n", path)
			return
		}
	}

	wg := sync.WaitGroup{}

	pathc := make(chan string)
	wg.Add(1)
	go func() {
		readNestedDir(*c.HtmlDir, pathc)
		wg.Done()
	}()

	o := make(chan scraper.Html)

	wg.Add(1)
	go func() {
		s.Start(o)
		wg.Done()
	}()

	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	for {
		select {
		case <-done:
			return
		case path := <-pathc:
			wg.Add(1)
			go func() {
				parseFile(path)
				wg.Done()
			}()
		case html := <-o:
			wg.Add(1)
			go func() {
				c.save(html.Href, html.Body)
				wg.Done()
			}()
		}
	}
}

func (c *config) save(href string, blob io.Reader) {
	fileName := transformUrlIntoFilename(href)
	path := filepath.Join(*c.HtmlDir, fileName)

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
	path := filepath.Join(*c.HtmlDir, fileName)
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

func pathToUrl(path string, storageDir string) (url string) {
	storage := storageDir + "/"
	url = strings.Replace(path, storage, "", 1) + "/"
	url = strings.Replace(url, "https:/", "https://", 1)

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

func readFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func removeFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}
	return nil
}

func readNestedDir(dirPath string, outputc chan string) error {
	if !doesFileExist(dirPath) {
		return fmt.Errorf("dirpath %v does not exist", dirPath)
	}

	done := make(chan bool)
	errc := make(chan error)
	count := 1

	var inner func(dirPath string)
	inner = func(dirPath string) {
		defer func() { done <- true }()

		fs, err := os.ReadDir(dirPath)
		if err != nil {
			errc <- err
			return
		}

		for _, f := range fs {
			path := filepath.Join(dirPath, f.Name())
			if f.IsDir() {
				count++
				go inner(path)
			} else {
				outputc <- path
			}
		}
	}

	go inner(dirPath)

	for {
		select {
		case err := <-errc:
			return err
		case <-done:
			count--
		default:
			if count == 0 {
				return nil
			}
		}
	}
}
