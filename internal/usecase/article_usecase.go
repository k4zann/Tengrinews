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

func (uc *ArticleUsecase) GetAllArticles() ([]models.Article, error) {
	return uc.ArticleRepo.GetAll()
}

func (uc *ArticleUsecase) GetArticleByID(id int) (*models.Article, error) {
	return uc.ArticleRepo.GetByID(id)
}
