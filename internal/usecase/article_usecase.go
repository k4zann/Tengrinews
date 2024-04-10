// usecase/article_usecase.go
package usecase

import (
	"errors"
	"fmt"
	"tengrinews/internal/domain"
	"tengrinews/internal/models"

	"go.mongodb.org/mongo-driver/mongo"
)

type ArticleUsecase struct {
	ArticleRepo domain.ArticleRepository
	Client      *mongo.Client
}

func NewArticleUsecase(repo domain.ArticleRepository, client *mongo.Client) *ArticleUsecase {
	return &ArticleUsecase{
		ArticleRepo: repo,
		Client:      client,
	}
}

func (uc *ArticleUsecase) GetAllArticles(result *models.Result) ([]models.Article, error) {
	return uc.ArticleRepo.GetAll(result)
}

func (uc *ArticleUsecase) GetArticleByID(result *models.Article, id string) (*models.Article, error) {
	return uc.ArticleRepo.GetByID(result, id)
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

	collection := uc.Client.Database("news").Collection("articles")
	fmt.Println(collection)
	return collection, nil
}
