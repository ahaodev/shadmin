package controller

import (
	"errors"
	"net/http"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

type UserInvitationController struct {
	Usecase domain.UserInvitationUseCase
}

// Accept godoc
// @Summary      Accept a user invitation
// @Description  Set an account username and password using a one-time invitation token
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.AcceptInvitationRequest  true  "Invitation token and account credentials"
// @Success      200      {object}  domain.Response  "Invitation accepted"
// @Failure      400      {object}  domain.Response  "Invitation is invalid or expired"
// @Failure      409      {object}  domain.Response  "Username already exists"
// @Router       /auth/invitations/accept [post]
func (c *UserInvitationController) Accept(ctx *gin.Context) {
	var request domain.AcceptInvitationRequest
	if !MustBindJSON(ctx, &request) {
		return
	}

	if err := c.Usecase.Accept(ctx.Request.Context(), &request); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInvitation):
			ctx.JSON(http.StatusBadRequest, domain.RespError("邀请链接无效或已过期"))
		case errors.Is(err, domain.ErrUsernameExists):
			ctx.JSON(http.StatusConflict, domain.RespError("用户名已被使用"))
		case errors.Is(err, domain.ErrInvitationPasswordInvalid):
			ctx.JSON(http.StatusBadRequest, domain.RespError("密码长度必须为 8 到 72 字节"))
		default:
			ctx.JSON(http.StatusInternalServerError, domain.RespError("接受邀请失败"))
		}
		return
	}

	ctx.JSON(http.StatusOK, domain.RespSuccess(nil))
}
