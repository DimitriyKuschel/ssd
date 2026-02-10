package providers

import (
	"net/http"
	"ssd/internal/structures"
)

type RouterProviderInterface interface {
	Get(url string, handler http.Handler)
	Post(url string, handler http.Handler)
	GetRoutes() []structures.Route
}

type RouterProvider struct {
	routes []structures.Route
}

func (rp *RouterProvider) Get(url string, handler http.Handler) {
	rp.routes = append(rp.routes, structures.Route{
		Url:     url,
		Handler: methodHandler(http.MethodGet, handler),
	})
}

func (rp *RouterProvider) Post(url string, handler http.Handler) {
	rp.routes = append(rp.routes, structures.Route{
		Url:     url,
		Handler: methodHandler(http.MethodPost, handler),
	})
}

func (rp *RouterProvider) GetRoutes() []structures.Route {
	return rp.routes
}

func NewRouterProvider() RouterProviderInterface {
	return &RouterProvider{}
}

func methodHandler(method string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
