package domain

import (
	"fmt"
	"tengrinews/internal/api"
	"tengrinews/internal/models"

	"go.mongodb.org/mongo-driver/mongo"
)

type ArticleRepository interface {
	GetAll(result *models.Result) ([]models.Article, error)
	GetByID(result *models.Post, id string, collection *mongo.Collection) error
	GetByCategory(result *models.Result, category string) ([]models.Article, error)
	GetBySearch(result *models.Result, search string) ([]models.Article, error)
}

type MockArticleRepository struct {
	Articles []models.Article
	Client   *mongo.Client
}

func (r *MockArticleRepository) GetAll(result *models.Result) ([]models.Article, error) {
	if err := api.FetchAllArticles(result); err != nil {
		return nil, fmt.Errorf("error fetching all articles: %s", err.Error())
	}

	return result.Posts, nil
}

func (r *MockArticleRepository) GetByID(result *models.Post, id string, collection *mongo.Collection) error {
	if err := api.FetchByIDMongo(result, id, collection); err != nil {
		return fmt.Errorf("error fetching article by ID %s: %s", id, err.Error())
	}

	if result == nil {
		return fmt.Errorf("no article found for ID %s", id)
	}

	return nil
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
