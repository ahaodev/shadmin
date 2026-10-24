package controller

import (
	"net/http"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

type UserInvitationController struct {
	Usecase domain.UserInvitationUseCase
}

// Accept godoc
//
//	@Summary		Accept a user invitation
//	@Description	Set an account username and password using a one-time invitation token
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		domain.AcceptInvitationRequest	true	"Invitation token and account credentials"
//	@Success		200		{object}	domain.Response					"Invitation accepted"
//	@Failure		400		{object}	domain.Response					"Invitation is invalid or expired"
//	@Failure		409		{object}	domain.Response					"Username already exists"
//	@Router			/auth/invitations/accept [post]
func (c *UserInvitationController) Accept(ctx *gin.Context) {
	var request domain.AcceptInvitationRequest
	if !MustBindJSON(ctx, &request) {
		return
	}

	if err := c.Usecase.Accept(ctx.Request.Context(), &request); err != nil {
		RespondError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, domain.RespSuccess(nil))
}
