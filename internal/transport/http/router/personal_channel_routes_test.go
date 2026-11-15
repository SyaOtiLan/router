package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPersonalChannelRoutePrefixesExposeSameEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	canonical := engine.Group("/api/v1/public/personal-channels")
	legacy := engine.Group("/api/v1/public/personal-provider")
	registerPersonalChannelRoutes(canonical)
	registerPersonalChannelRoutes(legacy)

	want := map[string]string{
		"GET /api/v1/public/personal-channels/connections":             "GET /api/v1/public/personal-provider/connections",
		"POST /api/v1/public/personal-channels/connections":            "POST /api/v1/public/personal-provider/connections",
		"GET /api/v1/public/personal-channels/connections/:id":         "GET /api/v1/public/personal-provider/connections/:id",
		"PUT /api/v1/public/personal-channels/connections/:id":         "PUT /api/v1/public/personal-provider/connections/:id",
		"POST /api/v1/public/personal-channels/connections/:id/verify": "POST /api/v1/public/personal-provider/connections/:id/verify",
		"DELETE /api/v1/public/personal-channels/connections/:id":      "DELETE /api/v1/public/personal-provider/connections/:id",
		"GET /api/v1/public/personal-channels/model-routes":            "GET /api/v1/public/personal-provider/model-routes",
		"PUT /api/v1/public/personal-channels/model-routes":            "PUT /api/v1/public/personal-provider/model-routes",
		"DELETE /api/v1/public/personal-channels/model-routes/:model":  "DELETE /api/v1/public/personal-provider/model-routes/:model",
		"GET /api/v1/public/personal-channels/routing-quota":           "GET /api/v1/public/personal-provider/routing-quota",
	}

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for canonicalRoute, legacyRoute := range want {
		if _, ok := routes[canonicalRoute]; !ok {
			t.Errorf("canonical route %q was not registered", canonicalRoute)
		}
		if _, ok := routes[legacyRoute]; !ok {
			t.Errorf("legacy route %q was not registered", legacyRoute)
		}
	}
}
