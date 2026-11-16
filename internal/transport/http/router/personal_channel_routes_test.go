package router

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPersonalChannelRoutesUseHierarchicalPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	canonical := engine.Group("/api/v1/public/personal/channels")
	registerPersonalChannelRoutes(canonical)

	want := []string{
		"GET /api/v1/public/personal/channels/connections",
		"POST /api/v1/public/personal/channels/connections",
		"GET /api/v1/public/personal/channels/connections/:id",
		"PUT /api/v1/public/personal/channels/connections/:id",
		"POST /api/v1/public/personal/channels/connections/:id/verify",
		"DELETE /api/v1/public/personal/channels/connections/:id",
		"GET /api/v1/public/personal/channels/models/routes",
		"PUT /api/v1/public/personal/channels/models/routes",
		"DELETE /api/v1/public/personal/channels/models/routes/:model",
		"GET /api/v1/public/personal/channels/routing/quota",
	}

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range want {
		if _, ok := routes[route]; !ok {
			t.Errorf("canonical route %q was not registered", route)
		}
	}
	for _, retiredPrefix := range []string{"/api/v1/public/personal-channels", "/api/v1/public/personal-provider"} {
		for _, route := range want {
			retiredRoute := strings.Replace(route, "/api/v1/public/personal/channels", retiredPrefix, 1)
			if _, ok := routes[retiredRoute]; ok {
				t.Errorf("retired route %q must not be registered", retiredRoute)
			}
		}
	}
}
