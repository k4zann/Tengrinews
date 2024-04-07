package domain

import (
	"fmt"
	"tengrinews/internal/models"
)

type ArticleRepository interface {
	GetAll() ([]models.Article, error)
	GetByID(id int) (*models.Article, error)
}

type MockArticleRepository struct {
	Articles []models.Article
}

func (r *MockArticleRepository) GetAll() ([]models.Article, error) {
	return r.Articles, nil
}

func (r *MockArticleRepository) GetByID(id int) (*models.Article, error) {
	for _, article := range r.Articles {
		if article.ID == id {
			return &article, nil
		}
	}
	return nil, fmt.Errorf("article not found")
}

