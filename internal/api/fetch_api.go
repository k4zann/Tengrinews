package api

import (
	"encoding/json"
	"os"
	"tengrinews/internal/models"
)

func FetchData(result *models.Result) {
	file, err := os.Open("sample.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&result)
	if err != nil {
		panic(err)
	}

}

func FetchDataByID(result *models.Result) {
	file, err := os.Open("sample.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&result)
	if err != nil {
		panic(err)
	}
}

func FetchDataByCategory(result *models.Result) {
	file, err := os.Open("sample.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&result)
	if err != nil {
		panic(err)
	}
}

func FetchDataBySearch(result *models.Result) {
	file, err := os.Open("sample.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&result)
	if err != nil {
		panic(err)
	}
}
