package controller

import (
	"net/http"
	"shadmin/domain"
	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

type ResourceController struct {
	ResourceUsecase domain.ResourceUseCase
}

// GetResources 获取用户可访问的系统资源菜单 (基于RBAC权限过滤)
// @Summary      Get user accessible resources
// @Description  Retrieve menu tree and button permissions filtered by user permissions
// @Tags         Resources
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200       {object} domain.Response{data=domain.UserResources}  "Successfully retrieved user resources with permissions"
// @Failure      500       {object} domain.Response  "Internal server error"
// @Router       /resources [get]
func (rc *ResourceController) GetResources(c *gin.Context) {
	userID := c.GetString(constants.UserID)
	isAdmin := c.GetBool(constants.IsAdmin)

	resources, err := rc.ResourceUsecase.GetUserResources(c, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(resources))
}
