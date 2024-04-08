package domain

import (
	"fmt"
	"tengrinews/internal/models"
)

type ArticleRepository interface {
	GetAll() ([]models.Article, error)
	GetByID(id int) (*models.Article, error)
	GetByCategory(category string) (*models.Article, error)
	GetBySearch(search string) (*models.Article, error)
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

func (r *MockArticleRepository) GetByCategory(category string) ([]models.Article, error) {
	// for _, article := range r.Articles {
	// 	if article.Category == category {
	// 		return &article, nil
	// 	}
	// }
	// return nil, fmt.Errorf("article not found")
	return r.Articles, nil
}

func (r *MockArticleRepository) GetBySearch(search string) ([]models.Article, error) {
	// articles := []models.Article{}
	// for i, article := range r.Articles {
	// 	if article.Title == search {
	// 		articles = append(articles, article)
	// 	}
	// 	if i == len(r.Articles) {
	// 		return articles, nil
	// 	}
	// }
	// return nil, fmt.Errorf("article not found")
	return r.Articles, nil
}
