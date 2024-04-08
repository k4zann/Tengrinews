package domain

import (
	"fmt"
	"tengrinews/internal/api"
	"tengrinews/internal/models"
)

type ArticleRepository interface {
	GetAll(result *models.Result) ([]models.Article, error)
	GetByID(result *models.Result, id string) (*models.Article, error)
	GetByCategory(result *models.Result, category string) ([]models.Article, error)
	GetBySearch(result *models.Result, search string) ([]models.Article, error)
}

type MockArticleRepository struct {
	Articles []models.Article
}

func (r *MockArticleRepository) GetAll(result *models.Result) ([]models.Article, error) {
	if err := api.FetchAllArticles(result); err != nil {
		return nil, fmt.Errorf("error fetching all articles: %s", err.Error())
	}

	return result.Posts, nil
}

func (r *MockArticleRepository) GetByID(result *models.Result, id string) (*models.Article, error) {
	if err := api.FetchDataByID(result, id); err != nil {
		return nil, fmt.Errorf("error fetching article by ID %s: %s", id, err.Error())
	}

	if len(result.Posts) == 0 {
		return nil, fmt.Errorf("no article found for ID %s", id)
	}

	return &result.Posts[0], nil
}

func (r *MockArticleRepository) GetByCategory(result *models.Result, category string) ([]models.Article, error) {
	if err := api.FetchDataByCategory(result, category); err != nil {
		return nil, fmt.Errorf("error fetching articles by category %s: %s", category, err.Error())
	}

	return result.Posts, nil
}

func (r *MockArticleRepository) GetBySearch(result *models.Result, search string) ([]models.Article, error) {
	if err := api.FetchDataBySearch(result, search); err != nil {
		return nil, fmt.Errorf("error fetching articles by search query %s: %s", search, err.Error())
	}

	return result.Posts, nil
}
