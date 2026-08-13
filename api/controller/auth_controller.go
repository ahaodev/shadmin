package controller

import (
	"errors"
	"net/http"
	"shadmin/domain"
	"shadmin/internal/constants"
	"shadmin/internal/contextutil"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	LoginUsecase domain.LoginUsecase
}

// Login godoc
// @Summary      User login
// @Description  Authenticate user and return JWT tokens with brute force protection
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.LoginRequest  true  "Login credentials"
// @Success      200  {object}  domain.Response{data=domain.LoginResponse}  "Login successful"
// @Failure      400  {object}  domain.Response  "Invalid request format"
// @Failure      401  {object}  domain.Response  "Invalid credentials"
// @Failure      423  {object}  domain.Response  "Account temporarily locked due to too many failed attempts"
// @Failure      500  {object}  domain.Response  "Internal server error"
// @Router       /login [post]
func (lc *AuthController) Login(c *gin.Context) {
	var request domain.LoginRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError("Invalid request format"))
		return
	}

	meta := domain.LoginMeta{
		ClientIP:  contextutil.GetClientIP(c),
		UserAgent: c.Request.Header.Get("User-Agent"),
	}

	result, err := lc.LoginUsecase.Login(c.Request.Context(), &request, meta)
	if err != nil {
		lc.writeLoginError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// writeLoginError 将 usecase 返回的登录错误映射为 HTTP 状态码。
func (lc *AuthController) writeLoginError(c *gin.Context, err error) {
	var lockedErr *domain.AccountLockedError
	switch {
	case errors.As(err, &lockedErr):
		c.JSON(http.StatusLocked, domain.RespError(lockedErr.Error()))
	case errors.Is(err, domain.ErrCaptchaRequired):
		c.JSON(http.StatusBadRequest, domain.RespError("请先完成验证码"))
	case errors.Is(err, domain.ErrCaptchaExpired):
		c.JSON(http.StatusBadRequest, domain.RespError("验证码已过期，请刷新"))
	case errors.Is(err, domain.ErrCaptchaInvalid):
		c.JSON(http.StatusBadRequest, domain.RespError("验证码校验失败，请重试"))
	case errors.Is(err, domain.ErrAccountInactive):
		c.JSON(http.StatusForbidden, domain.RespError(err.Error()))
	case errors.Is(err, domain.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, domain.RespError(err.Error()))
	default:
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
	}
}

// RefreshToken godoc
// @Summary      Refresh access token
// @Description  Refresh access token using refresh token
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.RefreshTokenRequest  true  "Refresh token request"
// @Success      200  {object}  domain.Response{data=domain.RefreshTokenResponse}  "Token refreshed successfully"
// @Failure      400  {object}  domain.Response  "Invalid request format"
// @Failure      401  {object}  domain.Response  "Invalid refresh token"
// @Failure      500  {object}  domain.Response  "Internal server error"
// @Router       /auth/refresh [post]
func (lc *AuthController) RefreshToken(c *gin.Context) {
	var request domain.RefreshTokenRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, domain.RespError("Invalid request format"))
		return
	}

	result, err := lc.LoginUsecase.Refresh(c.Request.Context(), request.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRefreshTokenRevoked):
			c.JSON(http.StatusUnauthorized, domain.RespError(err.Error()))
		case errors.Is(err, domain.ErrInvalidRefreshToken):
			c.JSON(http.StatusUnauthorized, domain.RespError("Invalid refresh token"))
		case errors.Is(err, domain.ErrAccountInactive):
			c.JSON(http.StatusForbidden, domain.RespError(err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(result))
}

// Logout godoc
// @Summary      User logout
// @Description  Logout user and invalidate tokens
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.LogoutRequest  false  "Logout request (optional)"
// @Success      200  {object}  domain.Response  "Logout successful"
// @Failure      400  {object}  domain.Response  "Invalid request format"
// @Failure      401  {object}  domain.Response  "Not authorized"
// @Router       /auth/logout [post]
func (lc *AuthController) Logout(c *gin.Context) {
	var request domain.LogoutRequest
	// 解析请求体（可选）
	if err := c.ShouldBind(&request); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, domain.RespError("Invalid request format"))
		return
	}

	accessToken := extractBearerToken(c)

	if err := lc.LoginUsecase.Logout(c.Request.Context(), accessToken, request.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError("注销失败"))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess("Logout successful"))
}

// extractBearerToken 从 Authorization 头中提取 Bearer token，未携带时返回空串。
func extractBearerToken(c *gin.Context) string {
	parts := strings.Split(c.Request.Header.Get(constants.Authorization), " ")
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return ""
}
