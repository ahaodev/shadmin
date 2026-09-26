package bootstrap

import (
	"context"
	"fmt"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/apiresource"
	"shadmin/ent/menu"
	"shadmin/internal/constants"
	"shadmin/repository"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// InitApiResources rebuilds route resources and restores menu associations in one transaction.
func InitApiResources(app *Application) error {
	ctx := context.Background()
	discoveredResources := scanGinRoutes(app.ApiEngine)

	tx, err := app.DB.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin API resource rebuild: %w", err)
	}
	defer tx.Rollback()

	// Lock the singleton generation row before comparing/rebuilding the route inventory.
	// Concurrent application instances then serialize this startup operation.
	if err := repository.BumpAuthorizationGeneration(ctx, tx); err != nil {
		return err
	}

	existingResources, err := tx.ApiResource.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("read existing API resources: %w", err)
	}
	if apiResourceInventoryMatches(existingResources, discoveredResources) {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("rollback unchanged API resource inventory: %w", err)
		}
		return nil
	}

	associations, err := getMenuApiResourceAssociations(ctx, tx.Client())
	if err != nil {
		return fmt.Errorf("read menu/API resource associations: %w", err)
	}
	deleted, err := tx.ApiResource.Delete().Exec(ctx)
	if err != nil {
		return fmt.Errorf("clear API resources: %w", err)
	}
	created, err := bulkCreateApiResources(ctx, tx, discoveredResources)
	if err != nil {
		return err
	}
	restored, err := restoreMenuApiResourceAssociations(ctx, tx, associations)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit API resource rebuild: %w", err)
	}

	log.Printf("API resources rebuilt: scanned=%d created=%d deleted=%d restored_associations=%d",
		len(discoveredResources), created, deleted, restored)
	return nil
}

func apiResourceInventoryMatches(existing []*ent.ApiResource, discovered []*domain.ApiResource) bool {
	if len(existing) != len(discovered) {
		return false
	}

	byID := make(map[string]*ent.ApiResource, len(existing))
	for _, resource := range existing {
		byID[resource.ID] = resource
	}
	for _, resource := range discovered {
		id := domain.GenerateApiResourceID(resource.Method, resource.Path)
		current, ok := byID[id]
		if !ok || current.Method != resource.Method || current.Path != resource.Path ||
			current.Handler != resource.Handler || current.Module != resource.Module || current.IsPublic != resource.IsPublic {
			return false
		}
	}
	return true
}

// scanGinRoutes 扫描Gin路由获取API资源
func scanGinRoutes(ginEngine *gin.Engine) []*domain.ApiResource {
	var resources []*domain.ApiResource

	// 获取所有路由
	routes := ginEngine.Routes()

	for _, route := range routes {
		// 跳过不需要存储的路由
		if shouldSkipRoute(route.Path) {
			continue
		}

		// 从路径提取模块名
		module := extractModuleFromPath(route.Path)

		// 提取处理器名称
		handlerName := route.Handler

		// 判断是否为公开端点
		isPublic := isPublicEndpoint(route.Path)

		resource := &domain.ApiResource{
			Method:   route.Method,
			Path:     route.Path,
			Handler:  handlerName,
			Module:   module,
			IsPublic: isPublic,
		}

		resources = append(resources, resource)
	}

	return resources
}

// bulkCreateApiResources 批量创建API资源
func bulkCreateApiResources(ctx context.Context, tx *ent.Tx, apiResources []*domain.ApiResource) (int, error) {
	if len(apiResources) == 0 {
		return 0, nil
	}

	now := time.Now()
	createdCount := 0

	for _, apiResource := range apiResources {
		// 验证必要字段
		if apiResource.Method == "" {
			log.Printf("WARN: Skipping API resource with empty method, path: %s", apiResource.Path)
			continue
		}
		if apiResource.Path == "" {
			log.Printf("WARN: Skipping API resource with empty path, method: %s", apiResource.Method)
			continue
		}

		// 生成稳定的ID（method:path组合）
		apiResource.ID = domain.GenerateApiResourceID(apiResource.Method, apiResource.Path)
		apiResource.CreatedAt = now
		apiResource.UpdatedAt = now

		_, err := tx.ApiResource.Create().
			SetID(apiResource.ID).
			SetMethod(apiResource.Method).
			SetPath(apiResource.Path).
			SetHandler(apiResource.Handler).
			SetModule(apiResource.Module).
			SetIsPublic(apiResource.IsPublic).
			SetCreatedAt(apiResource.CreatedAt).
			SetUpdatedAt(apiResource.UpdatedAt).
			Save(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to create API resource %s (method: %s, path: %s): %w", apiResource.ID, apiResource.Method, apiResource.Path, err)
		}
		createdCount++
	}

	return createdCount, nil
}

// shouldSkipRoute 判断是否应该跳过该路由
func shouldSkipRoute(path string) bool {
	allSkipPaths := constants.GetAPIPathsToSkip()

	for _, skipPath := range allSkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

// extractModuleFromPath 从API路径中提取模块名
func extractModuleFromPath(path string) string {
	// 移除前导斜杠并按斜杠分割
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "v1" {
		// 从 /api/v1/module/... 提取模块名
		return parts[2]
	}

	if len(parts) >= 1 {
		return parts[0]
	}

	return "unknown"
}

// isPublicEndpoint 判断端点是否为公开端点
func isPublicEndpoint(path string) bool {
	// 合并公开API路径和Swagger路径
	publicPaths := append(constants.PublicAPIPaths, constants.SwaggerAPIPaths...)

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}

// MenuApiResourceAssociation 菜单-API资源关联关系
type MenuApiResourceAssociation struct {
	MenuID        string
	ApiResourceID string
}

// getMenuApiResourceAssociations 获取所有菜单-API资源关联关系
func getMenuApiResourceAssociations(ctx context.Context, client *ent.Client) ([]MenuApiResourceAssociation, error) {
	var associations []MenuApiResourceAssociation

	// 查询所有菜单及其关联的API资源
	menus, err := client.Menu.Query().
		WithAPIResources().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query menus with API resources: %w", err)
	}

	for _, menu := range menus {
		for _, apiResource := range menu.Edges.APIResources {
			associations = append(associations, MenuApiResourceAssociation{
				MenuID:        menu.ID,
				ApiResourceID: apiResource.ID,
			})
		}
	}

	return associations, nil
}

// restoreMenuApiResourceAssociations 恢复菜单-API资源关联关系
func restoreMenuApiResourceAssociations(ctx context.Context, tx *ent.Tx, associations []MenuApiResourceAssociation) (int, error) {
	if len(associations) == 0 {
		return 0, nil
	}

	menuIDs, err := tx.Menu.Query().Select(menu.FieldID).Strings(ctx)
	if err != nil {
		return 0, fmt.Errorf("read existing menu IDs: %w", err)
	}
	existingMenus := make(map[string]struct{}, len(menuIDs))
	for _, id := range menuIDs {
		existingMenus[id] = struct{}{}
	}

	apiResourceIDs, err := tx.ApiResource.Query().Select(apiresource.FieldID).Strings(ctx)
	if err != nil {
		return 0, fmt.Errorf("read existing API resource IDs: %w", err)
	}
	existingApiResources := make(map[string]struct{}, len(apiResourceIDs))
	for _, id := range apiResourceIDs {
		existingApiResources[id] = struct{}{}
	}

	menuAssociations := make(map[string][]string)
	for _, assoc := range associations {
		menuAssociations[assoc.MenuID] = append(menuAssociations[assoc.MenuID], assoc.ApiResourceID)
	}

	restoredCount := 0
	for menuID, apiResourceIDs := range menuAssociations {
		if _, exists := existingMenus[menuID]; !exists {
			log.Printf("WARN: Menu %s no longer exists, skipping association restore", menuID)
			continue
		}

		validApiResourceIDs := make([]string, 0, len(apiResourceIDs))
		for _, apiResourceID := range apiResourceIDs {
			if _, exists := existingApiResources[apiResourceID]; exists {
				validApiResourceIDs = append(validApiResourceIDs, apiResourceID)
			} else {
				log.Printf("WARN: API resource %s no longer exists, skipping", apiResourceID)
			}
		}
		if len(validApiResourceIDs) == 0 {
			continue
		}

		if _, err := tx.Menu.UpdateOneID(menuID).
			AddAPIResourceIDs(validApiResourceIDs...).
			Save(ctx); err != nil {
			return 0, fmt.Errorf("restore API resource associations for menu %s: %w", menuID, err)
		}
		restoredCount += len(validApiResourceIDs)
	}

	return restoredCount, nil
}
