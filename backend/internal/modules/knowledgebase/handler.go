package knowledgebase

import (
	"context"
	"strconv"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	transporthttp "github.com/tsdlamongan/whcms/backend/internal/transport/http"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
)

// Middlewares is the narrow middleware surface the handler needs; satisfied
// by *transporthttp.Middleware.
type Middlewares interface {
	RequireAuth() fiber.Handler
	RequireRole(roles ...string) fiber.Handler
	RequirePermission(module string) fiber.Handler
	RateLimit(prefix string, limit int, window time.Duration) fiber.Handler
}

var _ Middlewares = (*transporthttp.Middleware)(nil)

// KnowledgebaseService is the use-case surface consumed by the handler
// (implemented by *Service).
type KnowledgebaseService interface {
	// Public
	PublicCategories(ctx context.Context) ([]domain.KBCategory, error)
	PublicCategoryBySlug(ctx context.Context, slug string) (*CategoryWithArticles, error)
	PublicArticles(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error)
	PublicArticleBySlug(ctx context.Context, slug string) (*domain.KBArticle, error)
	// Categories (admin)
	ListCategories(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error)
	GetCategory(ctx context.Context, id int64) (*domain.KBCategory, error)
	CreateCategory(ctx context.Context, actorUserID int64, in CategoryInput) (*domain.KBCategory, error)
	UpdateCategory(ctx context.Context, actorUserID, id int64, in CategoryUpdateInput) (*domain.KBCategory, error)
	DeleteCategory(ctx context.Context, actorUserID, id int64) error
	// Articles (admin)
	ListArticles(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error)
	GetArticle(ctx context.Context, id int64) (*domain.KBArticle, error)
	CreateArticle(ctx context.Context, actorUserID int64, in ArticleInput) (*domain.KBArticle, error)
	UpdateArticle(ctx context.Context, actorUserID, id int64, in ArticleUpdateInput) (*domain.KBArticle, error)
	DeleteArticle(ctx context.Context, actorUserID, id int64) error
}

var _ KnowledgebaseService = (*Service)(nil)

// Handler exposes the knowledgebase HTTP endpoints.
type Handler struct {
	svc KnowledgebaseService
	mw  Middlewares
}

// NewHandler builds the knowledgebase Handler.
func NewHandler(svc KnowledgebaseService, mw Middlewares) *Handler {
	return &Handler{svc: svc, mw: mw}
}

// RegisterRoutes mounts the knowledgebase routes on r (the /api/v1 group). See
// the package doc for the full route table.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	publicLimit := h.mw.RateLimit("kb", 60, time.Minute)
	pub := r.Group("/kb", publicLimit)
	pub.Get("/categories", h.PublicCategories)
	pub.Get("/categories/:slug", h.PublicCategoryBySlug)
	pub.Get("/articles", h.PublicArticles)
	pub.Get("/articles/:slug", h.PublicArticleBySlug)

	admin := r.Group("/admin/kb",
		h.mw.RequireAuth(),
		h.mw.RequireRole("admin", "staff"),
		h.mw.RequirePermission("knowledgebase"))

	cats := admin.Group("/categories")
	cats.Get("/", h.AdminListCategories)
	cats.Post("/", h.AdminCreateCategory)
	cats.Get("/:id", h.AdminGetCategory)
	cats.Patch("/:id", h.AdminUpdateCategory)
	cats.Delete("/:id", h.AdminDeleteCategory)

	arts := admin.Group("/articles")
	arts.Get("/", h.AdminListArticles)
	arts.Post("/", h.AdminCreateArticle)
	arts.Get("/:id", h.AdminGetArticle)
	arts.Patch("/:id", h.AdminUpdateArticle)
	arts.Delete("/:id", h.AdminDeleteArticle)
}

func parseID(c fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, apperr.Validation("invalid id")
	}
	return id, nil
}

// categoryIDQuery reads the optional ?category_id filter (0 = all / absent).
func categoryIDQuery(c fiber.Ctx) int64 {
	id, err := strconv.ParseInt(c.Query("category_id"), 10, 64)
	if err != nil || id < 1 {
		return 0
	}
	return id
}

func listParams(c fiber.Ctx) (httpx.Page, ports.ListParams) {
	page := httpx.ParsePage(c)
	return page, ports.ListParams{
		Page:    page.Page,
		PerPage: page.PerPage,
		Search:  c.Query("search"),
		Status:  c.Query("status"),
		Sort:    c.Query("sort"),
	}
}

// Public

// PublicCategories returns the non-hidden KB categories.
func (h *Handler) PublicCategories(c fiber.Ctx) error {
	data, err := h.svc.PublicCategories(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, data)
}

// PublicCategoryBySlug returns one category with its published articles.
func (h *Handler) PublicCategoryBySlug(c fiber.Ctx) error {
	data, err := h.svc.PublicCategoryBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	return httpx.OK(c, data)
}

// PublicArticles returns published articles (?category_id&search), paginated.
func (h *Handler) PublicArticles(c fiber.Ctx) error {
	page, params := listParams(c)
	rows, total, err := h.svc.PublicArticles(c.Context(), categoryIDQuery(c), params)
	if err != nil {
		return err
	}
	return httpx.OK(c, rows, page.Meta(total))
}

// PublicArticleBySlug returns one published article (and bumps its views).
func (h *Handler) PublicArticleBySlug(c fiber.Ctx) error {
	data, err := h.svc.PublicArticleBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	return httpx.OK(c, data)
}

// Admin: categories

// AdminListCategories lists categories (?include_hidden=false to exclude).
func (h *Handler) AdminListCategories(c fiber.Ctx) error {
	includeHidden := c.Query("include_hidden") != "false" // admin default: everything
	data, err := h.svc.ListCategories(c.Context(), includeHidden)
	if err != nil {
		return err
	}
	return httpx.OK(c, data)
}

// AdminCreateCategory creates a category.
func (h *Handler) AdminCreateCategory(c fiber.Ctx) error {
	var in CategoryInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	actor := httpx.MustIdentity(c)
	cat, err := h.svc.CreateCategory(c.Context(), actor.UserID, in)
	if err != nil {
		return err
	}
	return httpx.Created(c, cat)
}

// AdminGetCategory returns one category.
func (h *Handler) AdminGetCategory(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	cat, err := h.svc.GetCategory(c.Context(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, cat)
}

// AdminUpdateCategory patches a category.
func (h *Handler) AdminUpdateCategory(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var in CategoryUpdateInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	actor := httpx.MustIdentity(c)
	cat, err := h.svc.UpdateCategory(c.Context(), actor.UserID, id, in)
	if err != nil {
		return err
	}
	return httpx.OK(c, cat)
}

// AdminDeleteCategory soft-deletes a category (CONFLICT while articles exist).
func (h *Handler) AdminDeleteCategory(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	actor := httpx.MustIdentity(c)
	if err := h.svc.DeleteCategory(c.Context(), actor.UserID, id); err != nil {
		return err
	}
	return httpx.NoContent(c)
}

// Admin: articles

// AdminListArticles lists articles (?search&status=published|draft&category_id).
func (h *Handler) AdminListArticles(c fiber.Ctx) error {
	page, params := listParams(c)
	rows, total, err := h.svc.ListArticles(c.Context(), categoryIDQuery(c), params)
	if err != nil {
		return err
	}
	return httpx.OK(c, rows, page.Meta(total))
}

// AdminCreateArticle creates an article.
func (h *Handler) AdminCreateArticle(c fiber.Ctx) error {
	var in ArticleInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	actor := httpx.MustIdentity(c)
	a, err := h.svc.CreateArticle(c.Context(), actor.UserID, in)
	if err != nil {
		return err
	}
	return httpx.Created(c, a)
}

// AdminGetArticle returns one article.
func (h *Handler) AdminGetArticle(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	a, err := h.svc.GetArticle(c.Context(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, a)
}

// AdminUpdateArticle patches an article.
func (h *Handler) AdminUpdateArticle(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var in ArticleUpdateInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	actor := httpx.MustIdentity(c)
	a, err := h.svc.UpdateArticle(c.Context(), actor.UserID, id, in)
	if err != nil {
		return err
	}
	return httpx.OK(c, a)
}

// AdminDeleteArticle soft-deletes an article.
func (h *Handler) AdminDeleteArticle(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	actor := httpx.MustIdentity(c)
	if err := h.svc.DeleteArticle(c.Context(), actor.UserID, id); err != nil {
		return err
	}
	return httpx.NoContent(c)
}
