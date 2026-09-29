package shortener

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type LinkService interface {
	CreateLink(ctx context.Context, input CreateLinkInput) (Link, error)
	Resolve(ctx context.Context, shortCode string) (Link, error)
	RecordClick(ctx context.Context, linkID int64) error
	GetStats(ctx context.Context, shortCode string) (LinkStats, error)
}

type Handler struct {
	service LinkService
	baseURL string
}

func NewHandler(service LinkService, baseURL string) *Handler {
	return &Handler{service: service, baseURL: baseURL}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/api/v1/links", h.CreateLink)
	r.GET("/api/v1/links/:code/stats", h.Stats)
	r.GET("/:code", h.Redirect)
}

type createLinkRequest struct {
	LongURL   string     `json:"long_url" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type createLinkResponse struct {
	ShortCode string     `json:"short_code"`
	ShortURL  string     `json:"short_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (h *Handler) CreateLink(c *gin.Context) {
	var req createLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, err := h.service.CreateLink(c.Request.Context(), CreateLinkInput{
		LongURL:   req.LongURL,
		ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create link"})
		return
	}

	c.JSON(http.StatusCreated, createLinkResponse{
		ShortCode: link.ShortCode,
		ShortURL:  h.baseURL + "/" + link.ShortCode,
		ExpiresAt: link.ExpiresAt,
	})
}

func (h *Handler) Redirect(c *gin.Context) {
	code := c.Param("code")
	link, err := h.service.Resolve(c.Request.Context(), code)
	switch {
	case errors.Is(err, ErrNotFound):
		c.Status(http.StatusNotFound)
		return
	case errors.Is(err, ErrExpired):
		c.Status(http.StatusGone)
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve link"})
		return
	}

	go func() {
		if err := h.service.RecordClick(context.Background(), link.ID); err != nil {
			log.Printf("failed to record click for link %d: %v", link.ID, err)
		}
	}()

	c.Redirect(http.StatusFound, link.LongURL)
}

type statsResponse struct {
	ShortCode  string     `json:"short_code"`
	LongURL    string     `json:"long_url"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ClickCount int64      `json:"click_count"`
}

func (h *Handler) Stats(c *gin.Context) {
	code := c.Param("code")
	stats, err := h.service.GetStats(c.Request.Context(), code)
	switch {
	case errors.Is(err, ErrNotFound):
		c.Status(http.StatusNotFound)
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stats"})
		return
	}

	c.JSON(http.StatusOK, statsResponse{
		ShortCode:  stats.ShortCode,
		LongURL:    stats.LongURL,
		CreatedAt:  stats.CreatedAt,
		ExpiresAt:  stats.ExpiresAt,
		ClickCount: stats.ClickCount,
	})
}
