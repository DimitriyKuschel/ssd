package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestRouterProvider_GetAddsRoute(t *testing.T) {
	rp := NewRouterProvider()
	rp.Get("/test", dummyHandler())

	routes := rp.GetRoutes()
	require.Len(t, routes, 1)
	assert.Equal(t, "/test", routes[0].Url)
}

func TestRouterProvider_PostAddsRoute(t *testing.T) {
	rp := NewRouterProvider()
	rp.Post("/submit", dummyHandler())

	routes := rp.GetRoutes()
	require.Len(t, routes, 1)
	assert.Equal(t, "/submit", routes[0].Url)
}

func TestRouterProvider_MultipleRoutes(t *testing.T) {
	rp := NewRouterProvider()
	rp.Get("/a", dummyHandler())
	rp.Post("/b", dummyHandler())
	rp.Get("/c", dummyHandler())

	routes := rp.GetRoutes()
	assert.Len(t, routes, 3)
}

func TestMethodHandler_CorrectMethod(t *testing.T) {
	handler := methodHandler(http.MethodGet, dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

func TestMethodHandler_WrongMethod(t *testing.T) {
	handler := methodHandler(http.MethodGet, dummyHandler())

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestRouterProvider_GetRouteRejectsPost(t *testing.T) {
	rp := NewRouterProvider()
	rp.Get("/test", dummyHandler())

	route := rp.GetRoutes()[0]
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()
	route.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestRouterProvider_PostRouteRejectsGet(t *testing.T) {
	rp := NewRouterProvider()
	rp.Post("/submit", dummyHandler())

	route := rp.GetRoutes()[0]
	req := httptest.NewRequest(http.MethodGet, "/submit", nil)
	rr := httptest.NewRecorder()
	route.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}
