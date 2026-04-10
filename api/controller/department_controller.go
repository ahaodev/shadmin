package controller

import (
	"errors"
	"net/http"
	"shadmin/bootstrap"
	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

type DepartmentController struct {
	DepartmentUseCase domain.DepartmentUseCase
	Env               *bootstrap.Env
}

// GetDepartmentTree 获取部门树结构
// @Summary      Get department tree
// @Description  Retrieve departments in hierarchical tree structure
// @Tags         Departments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object} domain.Response{data=[]domain.Department}  "Successfully retrieved department tree"
// @Failure      500  {object} domain.Response  "Internal server error"
// @Router       /system/department/tree [get]
func (dc *DepartmentController) GetDepartmentTree(c *gin.Context) {
	tree, err := dc.DepartmentUseCase.FetchTree(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError("获取部门树失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.RespSuccess(tree))
}

// GetDepartment 获取单个部门
// @Summary      Get department by ID
// @Description  Get a specific department by its ID
// @Tags         Departments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path     string  true  "Department ID"
// @Success      200  {object} domain.Response{data=domain.Department}  "Successfully retrieved department"
// @Failure      404  {object} domain.Response  "Department not found"
// @Router       /system/department/{id} [get]
func (dc *DepartmentController) GetDepartment(c *gin.Context) {
	id := c.Param("id")

	dept, err := dc.DepartmentUseCase.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrDepartmentNotFound) {
			c.JSON(http.StatusNotFound, domain.RespError("部门不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError("获取部门失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.RespSuccess(dept))
}

// CreateDepartment 创建部门
// @Summary      Create department
// @Description  Create a new department
// @Tags         Departments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        department  body     domain.CreateDepartmentRequest  true  "Department information"
// @Success      201         {object} domain.Response  "Successfully created department"
// @Failure      400         {object} domain.Response  "Bad request"
// @Failure      409         {object} domain.Response  "Conflict - duplicate name"
// @Router       /system/department [post]
func (dc *DepartmentController) CreateDepartment(c *gin.Context) {
	var req domain.CreateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
		return
	}

	err := dc.DepartmentUseCase.Create(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, domain.ErrDepartmentNameExists) {
			c.JSON(http.StatusConflict, domain.RespError("同级下已存在同名部门"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError("创建部门失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusCreated, domain.RespSuccess(nil))
}

// UpdateDepartment 更新部门
// @Summary      Update department
// @Description  Update a specific department by its ID
// @Tags         Departments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id          path     string                          true  "Department ID"
// @Param        department  body     domain.UpdateDepartmentRequest  true  "Updated department information"
// @Success      200         {object} domain.Response{data=domain.Department}  "Successfully updated department"
// @Failure      400         {object} domain.Response  "Bad request"
// @Failure      404         {object} domain.Response  "Department not found"
// @Failure      409         {object} domain.Response  "Conflict - duplicate name or circular ref"
// @Router       /system/department/{id} [put]
func (dc *DepartmentController) UpdateDepartment(c *gin.Context) {
	id := c.Param("id")
	var req domain.UpdateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError(err.Error()))
		return
	}

	dept, err := dc.DepartmentUseCase.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, domain.ErrDepartmentNotFound) {
			c.JSON(http.StatusNotFound, domain.RespError("部门不存在"))
			return
		}
		if errors.Is(err, domain.ErrDepartmentNameExists) {
			c.JSON(http.StatusConflict, domain.RespError("同级下已存在同名部门"))
			return
		}
		if errors.Is(err, domain.ErrCircularDepartment) {
			c.JSON(http.StatusBadRequest, domain.RespError("不能将部门移动到其子部门下"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError("更新部门失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.RespSuccess(dept))
}

// DeleteDepartment 删除部门
// @Summary      Delete department
// @Description  Delete a specific department by its ID
// @Tags         Departments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path     string  true  "Department ID"
// @Success      200  {object} domain.Response  "Successfully deleted department"
// @Failure      400  {object} domain.Response  "Has children or users"
// @Failure      404  {object} domain.Response  "Department not found"
// @Router       /system/department/{id} [delete]
func (dc *DepartmentController) DeleteDepartment(c *gin.Context) {
	id := c.Param("id")

	err := dc.DepartmentUseCase.Delete(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrDepartmentNotFound) {
			c.JSON(http.StatusNotFound, domain.RespError("部门不存在"))
			return
		}
		if errors.Is(err, domain.ErrDepartmentHasChildren) {
			c.JSON(http.StatusBadRequest, domain.RespError("该部门下存在子部门，无法删除"))
			return
		}
		if errors.Is(err, domain.ErrDepartmentHasUsers) {
			c.JSON(http.StatusBadRequest, domain.RespError("该部门下存在用户，无法删除"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.RespError("删除部门失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.RespSuccess(map[string]interface{}{
		"department_id": id,
		"message":       "部门删除成功",
	}))
}
