package controller

import (
	"errors"
	"net/http"
	"shadmin/bootstrap"
	"shadmin/internal/contextutil"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	UserUsecase domain.UserUseCase
	Env         *bootstrap.Env
}

// GetUsers godoc
// @Summary      Get all user
// @Description  Retrieve all user with pagination and optional filtering (admin only)
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query  int     false  "Page number (default: 1)"
// @Param        page_size query  int     false  "Page size (default: 20)"
// @Param        search   query  string  false  "Filter by username or email"
// @Param        status    query  string  false  "Filter by user status (active, inactive, invited, suspended)"
// @Param        role      query  string  false  "Filter by user role"
// @Success      200       {object}  domain.Response  "user retrieved successfully"
// @Failure      500       {object}  domain.Response  "Internal server error"
// @Router       /system/user [get]
func (uc *UserController) GetUsers(c *gin.Context) {
	// 构建查询过滤器
	var filter domain.UserQueryFilter
	filter.QueryParams = BindQueryParams(c)

	// 解析过滤参数
	filter.Status = c.Query("status")
	filter.Role = c.Query("role")
	filter.Username = c.Query("username")
	filter.Email = c.Query("email")
	filter.IncludeRoles = c.Query("include_roles") == "true"

	// 使用统一查询接口
	result, err := uc.UserUsecase.ListUsers(c, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// CreateUser godoc
// @Summary      Create a new user
// @Description  Create a new user with optional creation (admin only)
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      domain.CreateUserRequest  true  "User creation data"
// @Success      201      {object}  domain.Response{data=domain.User}  "User created successfully"
// @Failure      400      {object}  domain.Response  "Invalid request data"
// @Failure      500      {object}  domain.Response  "Internal server error"
// @Router       /system/user [post]
func (uc *UserController) CreateUser(c *gin.Context) {
	var request domain.CreateUserRequest
	if !MustBindJSON(c, &request) {
		return
	}

	user, err := uc.UserUsecase.CreateUser(c, &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, domain.RespSuccess(user))
}

// GetUser godoc
// @Summary      Get user by ID
// @Description  Retrieve a specific user by ID
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "User ID"
// @Success      200  {object}  domain.Response{data=domain.User}  "User retrieved successfully"
// @Failure      404  {object}  domain.Response  "User not found"
// @Router       /system/user/{id} [get]
func (uc *UserController) GetUser(c *gin.Context) {
	id := c.Param("id")
	user, err := uc.UserUsecase.GetUserByID(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, domain.RespError("User not found"))
		return
	}
	c.JSON(http.StatusOK, domain.RespSuccess(user))
}

// UpdateUser godoc
// @Summary      Update user
// @Description  Update a specific user by ID
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path    string                    true  "User ID"
// @Param        user      body    domain.UserUpdateRequest  true  "Updated user data"
// @Success      200       {object}  domain.Response{data=domain.User}  "User updated successfully"
// @Failure      400       {object}  domain.Response  "Invalid request data"
// @Failure      500       {object}  domain.Response  "Internal server error"
// @Router       /system/user/{id} [put]
func (uc *UserController) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var updateReq domain.UserUpdateRequest
	if !MustBindJSON(c, &updateReq) {
		return
	}

	// 执行用户更新
	if err := uc.UserUsecase.UpdateUserPartial(c, id, updateReq); err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	// 获取更新后的用户信息
	user, err := uc.UserUsecase.GetUserByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(user))
}

// InviteUser godoc
// @Summary      Invite user to
// @Description  Send invitation to a user to join a specific  with a role
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      domain.InviteUserRequest  true  "User invitation data"
// @Success      201      {object}  domain.Response{data=domain.User}  "User invitation sent successfully"
// @Failure      400      {object}  domain.Response  "Invalid request data"
// @Failure      401      {object}  domain.Response  "Unauthorized - user information not found"
// @Failure      500      {object}  domain.Response  "Internal server error"
// @Router       /system/user/invite [post]
func (uc *UserController) InviteUser(c *gin.Context) {
	var request domain.InviteUserRequest
	if !MustBindJSON(c, &request) {
		return
	}

	// 获取当前用户ID作为邀请人
	invitedBy, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.RespError("未找到用户信息"))
		return
	}

	user, err := uc.UserUsecase.InviteUser(c, &request, invitedBy.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, domain.RespSuccess(user))
}

// GetUserRoles godoc
// @Summary      Get user roles
// @Description  Get roles of a specific user in the system
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path    string  true   "User ID"
// @Success      200       {object}  domain.Response  "User roles retrieved successfully"
// @Failure      400       {object}  domain.Response  "Missing user ID parameter"
// @Failure      404       {object}  domain.Response  "User not found"
// @Failure      500       {object}  domain.Response  "Internal server error"
// @Router       /system/user/{id}/roles [get]
func (uc *UserController) GetUserRoles(c *gin.Context) {
	userID := c.Param("id")

	roles, err := uc.UserUsecase.GetUserRoles(c, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(roles))
}

// DeleteUser godoc
// @Summary      Delete user
// @Description  Delete a user from the system
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path    string  true   "User ID"
// @Success      200       {object}  domain.Response  "User deleted successfully"
// @Failure      400       {object}  domain.Response  "Missing user ID parameter"
// @Failure      500       {object}  domain.Response  "Internal server error"
// @Router       /system/user/{id} [delete]
func (uc *UserController) DeleteUser(c *gin.Context) {
	userID := c.Param("id")

	// 获取当前操作用户ID
	currentUserID := contextutil.GetUserID(c)
	if currentUserID == "" {
		c.JSON(http.StatusUnauthorized, domain.RespError("用户未认证"))
		return
	}

	// 执行删除用户逻辑（含权限校验）
	if err := uc.UserUsecase.DeleteUser(c, userID, currentUserID); err != nil {
		if errors.Is(err, domain.ErrCannotDeleteSelf) || errors.Is(err, domain.ErrCannotDeleteAdmin) {
			c.JSON(http.StatusForbidden, domain.RespError(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess("User deleted successfully"))
}
