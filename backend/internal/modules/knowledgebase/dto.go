package knowledgebase

import (
	"github.com/tsdlamongan/whcms/backend/internal/domain"
)

// Categories

// CategoryInput creates a knowledgebase category.
type CategoryInput struct {
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Slug        string `json:"slug" validate:"omitempty,min=2,max=100"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
	Hidden      bool   `json:"hidden"`
}

// CategoryUpdateInput patches a category; nil fields are left unchanged.
type CategoryUpdateInput struct {
	Name        *string `json:"name" validate:"omitempty,min=2,max=100"`
	Slug        *string `json:"slug" validate:"omitempty,min=2,max=100"`
	Description *string `json:"description"`
	Sort        *int    `json:"sort"`
	Hidden      *bool   `json:"hidden"`
}

// Articles

// ArticleInput creates a knowledgebase article.
type ArticleInput struct {
	CategoryID int64  `json:"category_id" validate:"required,min=1"`
	Title      string `json:"title" validate:"required,min=2,max=200"`
	Slug       string `json:"slug" validate:"omitempty,min=2,max=200"`
	Body       string `json:"body"`
	Published  bool   `json:"published"`
	Sort       int    `json:"sort"`
}

// ArticleUpdateInput patches an article; nil fields are left unchanged.
type ArticleUpdateInput struct {
	CategoryID *int64  `json:"category_id" validate:"omitempty,min=1"`
	Title      *string `json:"title" validate:"omitempty,min=2,max=200"`
	Slug       *string `json:"slug" validate:"omitempty,min=2,max=200"`
	Body       *string `json:"body"`
	Published  *bool   `json:"published"`
	Sort       *int    `json:"sort"`
}

// Public payloads

// CategoryWithArticles is the payload of GET /kb/categories/:slug: the category
// plus its published articles.
type CategoryWithArticles struct {
	domain.KBCategory
	Articles []domain.KBArticle `json:"articles"`
}
