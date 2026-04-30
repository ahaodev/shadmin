package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"shadmin/domain"
)

// CaptchaController 公开的验证码控制器
type CaptchaController struct {
	CaptchaUsecase domain.CaptchaUsecase
}

// GetSlideCaptcha godoc
// @Summary      Get slide captcha challenge
// @Description  Generate a new slide captcha challenge for login. Pass old_captcha_id to invalidate the previous challenge when refreshing.
// @Tags         Authentication
// @Produce      json
// @Param        old_captcha_id  query     string  false  "Previous captcha id to invalidate"
// @Success      200  {object}  domain.Response{data=domain.SlideCaptchaChallenge}
// @Failure      500  {object}  domain.Response
// @Router       /auth/captcha/slide [get]
func (cc *CaptchaController) GetSlideCaptcha(c *gin.Context) {
	if cc.CaptchaUsecase == nil {
		c.JSON(http.StatusInternalServerError, domain.RespError("captcha service not initialized"))
		return
	}

	oldID := c.Query("old_captcha_id")
	challenge, err := cc.CaptchaUsecase.GenerateSlide(c, oldID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.RespSuccess(challenge))
}
