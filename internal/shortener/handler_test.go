package shortener

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeService struct {
	createLinkFn  func(ctx context.Context, input CreateLinkInput) (Link, error)
	resolveFn     func(ctx context.Context, shortCode string) (Link, error)
	recordClickFn func(ctx context.Context, linkID int64) error
	getStatsFn    func(ctx context.Context, shortCode string) (LinkStats, error)
}

func (f *fakeService) CreateLink(ctx context.Context, input CreateLinkInput) (Link, error) {
	return f.createLinkFn(ctx, input)
}
func (f *fakeService) Resolve(ctx context.Context, shortCode string) (Link, error) {
	return f.resolveFn(ctx, shortCode)
}
func (f *fakeService) RecordClick(ctx context.Context, linkID int64) error {
	if f.recordClickFn != nil {
		return f.recordClickFn(ctx, linkID)
	}
	return nil
}
func (f *fakeService) GetStats(ctx context.Context, shortCode string) (LinkStats, error) {
	return f.getStatsFn(ctx, shortCode)
}

func newTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterRoutes(r)
	return r
}

func TestHandler_CreateLink(t *testing.T) {
	svc := &fakeService{
		createLinkFn: func(ctx context.Context, input CreateLinkInput) (Link, error) {
			return Link{ID: 1, LongURL: input.LongURL, ShortCode: "1"}, nil
		},
	}
	h := NewHandler(svc, "http://localhost:8080")
	router := newTestRouter(h)

	body, _ := json.Marshal(map[string]string{"long_url": "https://example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp createLinkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ShortURL != "http://localhost:8080/1" {
		t.Errorf("ShortURL = %q, want %q", resp.ShortURL, "http://localhost:8080/1")
	}
}

func TestHandler_Redirect(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		svc := &fakeService{
			resolveFn: func(ctx context.Context, shortCode string) (Link, error) {
				return Link{ID: 1, LongURL: "https://example.com", ShortCode: shortCode}, nil
			},
		}
		h := NewHandler(svc, "http://localhost:8080")
		router := newTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
		}
		if loc := w.Header().Get("Location"); loc != "https://example.com" {
			t.Errorf("Location = %q, want %q", loc, "https://example.com")
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := &fakeService{
			resolveFn: func(ctx context.Context, shortCode string) (Link, error) {
				return Link{}, ErrNotFound
			},
		}
		h := NewHandler(svc, "http://localhost:8080")
		router := newTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("expired", func(t *testing.T) {
		svc := &fakeService{
			resolveFn: func(ctx context.Context, shortCode string) (Link, error) {
				return Link{}, ErrExpired
			},
		}
		h := NewHandler(svc, "http://localhost:8080")
		router := newTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/old", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusGone {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusGone)
		}
	})
}

func TestHandler_Stats(t *testing.T) {
	created := time.Now()
	svc := &fakeService{
		getStatsFn: func(ctx context.Context, shortCode string) (LinkStats, error) {
			return LinkStats{
				Link:       Link{ShortCode: shortCode, LongURL: "https://example.com", CreatedAt: created},
				ClickCount: 5,
			}, nil
		},
	}
	h := NewHandler(svc, "http://localhost:8080")
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/abc/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp statsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ClickCount != 5 {
		t.Errorf("ClickCount = %d, want 5", resp.ClickCount)
	}
}
