package controller

import (
	"net/http"
	"shadmin/domain"
	"shadmin/internal/constants"
	"shadmin/internal/contextutil"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	LoginUsecase domain.LoginUsecase
}

// Login godoc
//
//	@Summary		User login
//	@Description	Authenticate user and return JWT tokens with brute force protection
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		domain.LoginRequest							true	"Login credentials"
//	@Success		200		{object}	domain.Response{data=domain.LoginResponse}	"Login successful"
//	@Failure		400		{object}	domain.Response								"Invalid request format"
//	@Failure		401		{object}	domain.Response								"Invalid credentials"
//	@Failure		423		{object}	domain.Response								"Account temporarily locked due to too many failed attempts"
//	@Failure		500		{object}	domain.Response								"Internal server error"
//	@Router			/login [post]
func (lc *AuthController) Login(c *gin.Context) {
	var request domain.LoginRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError("Invalid request format"))
		return
	}

	meta := domain.LoginMeta{
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.Header.Get("User-Agent"),
	}

	result, err := lc.LoginUsecase.Login(c.Request.Context(), &request, meta)
	if err != nil {
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// RefreshToken godoc
//
//	@Summary		Refresh access token
//	@Description	Refresh access token using refresh token
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		domain.RefreshTokenRequest							true	"Refresh token request"
//	@Success		200		{object}	domain.Response{data=domain.RefreshTokenResponse}	"Token refreshed successfully"
//	@Failure		400		{object}	domain.Response										"Invalid request format"
//	@Failure		401		{object}	domain.Response										"Invalid refresh token"
//	@Failure		500		{object}	domain.Response										"Internal server error"
//	@Router			/auth/refresh [post]
func (lc *AuthController) RefreshToken(c *gin.Context) {
	var request domain.RefreshTokenRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError("Invalid request format"))
		return
	}

	result, err := lc.LoginUsecase.Refresh(c.Request.Context(), request.RefreshToken)
	if err != nil {
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// Logout godoc
//
//	@Summary		User logout
//	@Description	Logout user and invalidate tokens
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		domain.LogoutRequest	false	"Logout request (optional)"
//	@Success		200		{object}	domain.Response			"Logout successful"
//	@Failure		400		{object}	domain.Response			"Invalid request format"
//	@Failure		401		{object}	domain.Response			"Not authorized"
//	@Router			/auth/logout [post]
func (lc *AuthController) Logout(c *gin.Context) {
	var request domain.LogoutRequest
	// 解析请求体（可选）
	if err := c.ShouldBind(&request); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, domain.RespError("Invalid request format"))
		return
	}

	accessToken, _ := contextutil.BearerToken(c.Request.Header.Get(constants.Authorization))

	if err := lc.LoginUsecase.Logout(c.Request.Context(), accessToken, request.RefreshToken); err != nil {
		RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess("Logout successful"))
}
