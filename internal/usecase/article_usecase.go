// usecase/article_usecase.go
package usecase

import (
	"errors"
	"tengrinews/internal/domain"
	"tengrinews/internal/models"

	"go.mongodb.org/mongo-driver/mongo"
)

type ArticleUsecase struct {
	ArticleRepo domain.ArticleRepository
	Client      *mongo.Client
	Collection  *mongo.Collection
}

func NewArticleUsecase(repo domain.ArticleRepository, client *mongo.Client, collection *mongo.Collection) *ArticleUsecase {
	return &ArticleUsecase{
		ArticleRepo: repo,
		Client:      client,
		Collection:  collection,
	}
}

func (uc *ArticleUsecase) GetAllArticles(result *models.Result) ([]models.Article, error) {
	return uc.ArticleRepo.GetAll(result)
}

func (uc *ArticleUsecase) GetArticleByID(id string) (*models.Post, error) {
	collection, err := uc.GetCollection()
	if err != nil {
		return nil, err
	}

	article := &models.Post{}
	err = uc.ArticleRepo.GetByID(article, id, collection)
	if err != nil {
		return nil, err
	}

	return article, nil
}

func (uc *ArticleUsecase) GetArticlesByCategory(result *models.Result, category string) ([]models.Article, error) {
	return uc.ArticleRepo.GetByCategory(result, category)
}

func (uc *ArticleUsecase) GetArticlesBySearch(result *models.Result, search string) ([]models.Article, error) {
	return uc.ArticleRepo.GetBySearch(result, search)
}

func (uc *ArticleUsecase) GetCollection() (*mongo.Collection, error) {
	if uc.Client == nil {
		return nil, errors.New("MongoDB client is not initialized")
	}

	return uc.Collection, nil
}
