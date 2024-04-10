package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"tengrinews/internal/helpers"
	"tengrinews/internal/models"
	"time"
)

func FetchAllArticles(result *models.Result) error {
	file, err := os.Open("sample.json")
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	decodingDone := make(chan struct{})

	go func() {
		defer close(decodingDone)
		err := json.NewDecoder(file).Decode(&result)
		if err != nil {
			panic(err)
		}
	}()

	select {
	case <-decodingDone:
		fmt.Println("Decoding done")
		return nil
	case <-time.After(5 * time.Second):
		fmt.Println("Decoding timeout")
		return fmt.Errorf("decoding timeout")
	}
}

func FetchByID(result *models.Article, id string) error {
	file, err := os.Open("sample.json")
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	decodingDone := make(chan struct{})

	go func() {
		defer close(decodingDone)
		var posts models.Result
		err := json.NewDecoder(file).Decode(&posts)
		if err != nil {
			panic(err)
		}

		for _, post := range posts.Posts {
			if post.ID == id {
				*result = post
				helpers.CutImageData(post.Content)
				return
			}
		}
	}()

	select {
	case <-decodingDone:
		fmt.Println("Decoding done")
		return nil
	case <-time.After(5 * time.Second):
		fmt.Println("Decoding timeout")
		return fmt.Errorf("decoding timeout")
	}

}

func FetchDataByID(result *models.Article, id string) error {
	apiURL := fmt.Sprintf("%s?apikey=%s&id=%s", helpers.APIURL, helpers.APIKey, id)
	fmt.Println(apiURL)
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("error fetching data for ID %s: %s", id, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching data for ID %s: received status code %d", id, resp.StatusCode)
	}

	var idResult models.Article
	if err := json.NewDecoder(resp.Body).Decode(&idResult); err != nil {
		return fmt.Errorf("error decoding response for ID %s: %s", id, err.Error())
	}

	if len(idResult.ID) == 0 {
		return fmt.Errorf("no post found for ID %s", id)
	}

	*result = idResult

	return nil
}

func FetchDataByCategory(result *models.Result, cat string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errMutex sync.Mutex
	var err error
	var resp *http.Response
	wg.Add(1)
	go func(cat string) {
		defer wg.Done()

		apiURL := fmt.Sprintf("%s?apikey=%s&category=%s&%s", helpers.APIURL, helpers.APIKey, cat, helpers.APILang)
		resp, err = http.Get(apiURL)
		fmt.Println(apiURL)
		if err != nil {
			errMutex.Lock()
			defer errMutex.Unlock()
			err = fmt.Errorf("error fetching data for category %s: %s", cat, err.Error())
			return
		}
		defer resp.Body.Close()

		var categoryResult models.Result
		if err = json.NewDecoder(resp.Body).Decode(&categoryResult); err != nil {
			errMutex.Lock()
			defer errMutex.Unlock()
			err = fmt.Errorf("error decoding response for category %s: %s", cat, err.Error())
			return
		}

		mu.Lock()
		defer mu.Unlock()
		result.Posts = append(result.Posts, categoryResult.Posts...)
	}(cat)

	wg.Wait()

	errMutex.Lock()
	defer errMutex.Unlock()
	return err
}

func FetchDataBySearch(result *models.Result, query string) error {
	apiURL := fmt.Sprintf("%s?apikey=%s&q=%s", helpers.APIURL, helpers.APIKey, url.QueryEscape(query))
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("error fetching data for query %s: %s", query, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching data for query %s: received status code %d", query, resp.StatusCode)
	}

	var searchResult models.Result
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return fmt.Errorf("error decoding response for query %s: %s", query, err.Error())
	}

	*result = searchResult

	return nil
}
