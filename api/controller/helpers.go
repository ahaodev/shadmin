package controller

import (
	"net/http"
	"strconv"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

func BindQueryParams(c *gin.Context) domain.QueryParams {
	var params domain.QueryParams
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			params.Page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil {
			params.PageSize = v
		}
	}
	params.SortBy = c.Query("sort_by")
	params.Order = c.Query("order")
	return params
}

// MustBindJSON 失败时写入 400 响应并返回 false。
func MustBindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
		return false
	}
	return true
}

// MustBindQuery 失败时写入 400 响应并返回 false。
func MustBindQuery(c *gin.Context, obj any) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
		return false
	}
	return true
}
