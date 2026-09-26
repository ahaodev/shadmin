package bootstrap

import (
	"context"
	"net/http"
	"testing"

	"shadmin/ent/apiresource"
	"shadmin/ent/menu"
	"shadmin/repository"

	"github.com/gin-gonic/gin"
)

func TestInitApiResourcesBumpsGenerationOnlyWhenInventoryChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newTestEntClient(t)
	if err := repository.EnsureAuthorizationState(context.Background(), client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}

	engine := gin.New()
	engine.GET("/api/v1/first", func(c *gin.Context) { c.Status(http.StatusOK) })
	app := &Application{DB: client, ApiEngine: engine}
	ctx := context.Background()

	if err := InitApiResources(app); err != nil {
		t.Fatalf("initial route scan: %v", err)
	}
	firstGeneration, err := repository.CurrentAuthorizationGeneration(ctx, client)
	if err != nil {
		t.Fatalf("read initial generation: %v", err)
	}
	if firstGeneration != 1 {
		t.Fatalf("initial generation = %d, want 1", firstGeneration)
	}

	if err := InitApiResources(app); err != nil {
		t.Fatalf("repeat identical route scan: %v", err)
	}
	unchangedGeneration, err := repository.CurrentAuthorizationGeneration(ctx, client)
	if err != nil {
		t.Fatalf("read unchanged generation: %v", err)
	}
	if unchangedGeneration != firstGeneration {
		t.Fatalf("generation after no-op scan = %d, want %d", unchangedGeneration, firstGeneration)
	}

	engine = gin.New()
	engine.GET("/api/v1/second", func(c *gin.Context) { c.Status(http.StatusOK) })
	app.ApiEngine = engine
	if err := InitApiResources(app); err != nil {
		t.Fatalf("changed route scan: %v", err)
	}
	changedGeneration, err := repository.CurrentAuthorizationGeneration(ctx, client)
	if err != nil {
		t.Fatalf("read changed generation: %v", err)
	}
	if changedGeneration != firstGeneration+1 {
		t.Fatalf("generation after route change = %d, want %d", changedGeneration, firstGeneration+1)
	}

	if exists, err := client.ApiResource.Query().Where(apiresource.MethodEQ(http.MethodGet), apiresource.PathEQ("/api/v1/first")).Exist(ctx); err != nil || exists {
		t.Fatalf("old API resource exists = %v, err = %v; want deleted", exists, err)
	}
	if exists, err := client.ApiResource.Query().Where(apiresource.MethodEQ(http.MethodGet), apiresource.PathEQ("/api/v1/second")).Exist(ctx); err != nil || !exists {
		t.Fatalf("new API resource exists = %v, err = %v; want present", exists, err)
	}
}

func TestInitApiResourcesRestoresOnlyAssociationsForRetainedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newTestEntClient(t)
	ctx := context.Background()
	if err := repository.EnsureAuthorizationState(ctx, client); err != nil {
		t.Fatalf("ensure authorization state: %v", err)
	}

	engine := gin.New()
	engine.GET("/api/v1/retained", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/api/v1/removed", func(c *gin.Context) { c.Status(http.StatusOK) })
	app := &Application{DB: client, ApiEngine: engine}
	if err := InitApiResources(app); err != nil {
		t.Fatalf("initial route scan: %v", err)
	}

	const menuID = "reports"
	if err := client.Menu.Create().
		SetID(menuID).
		SetName("Reports").
		SetType("menu").
		SetVisible("show").
		SetStatus("active").
		AddAPIResourceIDs("GET:/api/v1/retained", "GET:/api/v1/removed").
		Exec(ctx); err != nil {
		t.Fatalf("create menu and API resource bindings: %v", err)
	}

	engine = gin.New()
	engine.GET("/api/v1/retained", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/api/v1/added", func(c *gin.Context) { c.Status(http.StatusOK) })
	app.ApiEngine = engine
	if err := InitApiResources(app); err != nil {
		t.Fatalf("rebuild changed route inventory: %v", err)
	}

	storedMenu, err := client.Menu.Query().Where(menu.ID(menuID)).WithAPIResources().Only(ctx)
	if err != nil {
		t.Fatalf("read restored menu: %v", err)
	}
	if len(storedMenu.Edges.APIResources) != 1 || storedMenu.Edges.APIResources[0].ID != "GET:/api/v1/retained" {
		t.Fatalf("restored API resources = %v, want only retained route", storedMenu.Edges.APIResources)
	}
}
