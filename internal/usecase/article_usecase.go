// usecase/article_usecase.go
package usecase

import (
	"tengrinews/internal/domain"
	"tengrinews/internal/models"
)

type ArticleUsecase struct {
	ArticleRepo domain.ArticleRepository
}

func NewArticleUsecase(repo domain.ArticleRepository) *ArticleUsecase {
	return &ArticleUsecase{
		ArticleRepo: repo,
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
