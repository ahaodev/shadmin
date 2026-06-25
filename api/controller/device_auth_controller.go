package controller

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"shadmin/domain"
	"shadmin/internal/constants"

	"github.com/gin-gonic/gin"
)

type DeviceAuthController struct {
	DeviceAuthUsecase domain.DeviceAuthUsecase
	requestLimiter    *deviceAuthRateLimiter
	pollLimiter       *deviceAuthRateLimiter
	activateLimiter   *deviceAuthRateLimiter
}

func NewDeviceAuthController(deviceAuthUsecase domain.DeviceAuthUsecase) *DeviceAuthController {
	return &DeviceAuthController{
		DeviceAuthUsecase: deviceAuthUsecase,
		requestLimiter:    newDeviceAuthRateLimiter(10, time.Minute),
		pollLimiter:       newDeviceAuthRateLimiter(60, time.Minute),
		activateLimiter:   newDeviceAuthRateLimiter(20, time.Minute),
	}
}

// RequestCode godoc
// @Summary      Request device authorization code
// @Description  Create a device authorization session for CLI login
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.DeviceCodeRequest  true  "Device code request"
// @Success      200  {object}  domain.Response{data=domain.DeviceCodeResponse}
// @Failure      400  {object}  domain.Response
// @Failure      500  {object}  domain.Response
// @Router       /auth/device/code [post]
func (dc *DeviceAuthController) RequestCode(c *gin.Context) {
	if !dc.allow(c, dc.requestLimiter) {
		return
	}

	var request domain.DeviceCodeRequest
	if !MustBindJSON(c, &request) {
		return
	}

	resp, err := dc.DeviceAuthUsecase.RequestCode(c, request, "/auth/device")
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.RespError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.RespSuccess(resp))
}

// PollToken godoc
// @Summary      Poll device authorization token
// @Description  Poll until user authorizes a device code, then return JWT tokens
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.DeviceTokenRequest  true  "Device token request"
// @Success      200  {object}  domain.Response{data=domain.LoginResponse}
// @Failure      400  {object}  domain.Response
// @Failure      500  {object}  domain.Response
// @Router       /auth/device/token [post]
func (dc *DeviceAuthController) PollToken(c *gin.Context) {
	if !dc.allow(c, dc.pollLimiter) {
		return
	}

	var request domain.DeviceTokenRequest
	if !MustBindJSON(c, &request) {
		return
	}

	resp, err := dc.DeviceAuthUsecase.PollToken(c, request)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, domain.ErrDeviceAuthorizationPending),
			errors.Is(err, domain.ErrDeviceSlowDown),
			errors.Is(err, domain.ErrDeviceExpired),
			errors.Is(err, domain.ErrDeviceConsumed),
			errors.Is(err, domain.ErrDeviceAccessDenied),
			errors.Is(err, domain.ErrDeviceInvalidCode):
			status = http.StatusBadRequest
		case errors.Is(err, domain.ErrUserDisabled):
			status = http.StatusForbidden
		default:
			status = http.StatusInternalServerError
		}
		c.JSON(status, domain.RespError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.RespSuccess(resp))
}

// Activate godoc
// @Summary      Activate device authorization code
// @Description  Authorize a CLI device code using the current frontend user session
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      domain.DeviceActivateRequest  true  "Device activation request"
// @Success      200  {object}  domain.Response{data=domain.DeviceActivateResponse}
// @Failure      400  {object}  domain.Response
// @Failure      401  {object}  domain.Response
// @Failure      500  {object}  domain.Response
// @Router       /auth/device/activate [post]
func (dc *DeviceAuthController) Activate(c *gin.Context) {
	if !dc.allow(c, dc.activateLimiter) {
		return
	}

	var request domain.DeviceActivateRequest
	if !MustBindJSON(c, &request) {
		return
	}

	userIDValue, exists := c.Get(constants.UserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.RespError("Not authorized"))
		return
	}
	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, domain.RespError("Invalid user context"))
		return
	}

	resp, err := dc.DeviceAuthUsecase.Activate(c, userID, request)
	if err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, domain.ErrDeviceInvalidCode) &&
			!errors.Is(err, domain.ErrDeviceExpired) &&
			!errors.Is(err, domain.ErrDeviceAccessDenied) {
			status = http.StatusInternalServerError
		}
		c.JSON(status, domain.RespError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, domain.RespSuccess(resp))
}

func (dc *DeviceAuthController) allow(c *gin.Context, limiter *deviceAuthRateLimiter) bool {
	if limiter == nil || limiter.Allow(c.ClientIP()) {
		return true
	}
	c.JSON(http.StatusTooManyRequests, domain.RespError("too_many_requests"))
	return false
}

// Close is kept for callers that manage controller lifecycles.
func (dc *DeviceAuthController) Close() {
	dc.requestLimiter.close()
	dc.pollLimiter.close()
	dc.activateLimiter.close()
}

type deviceAuthRateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	hits        map[string]*deviceAuthRateLimitEntry
	lastCleanup time.Time
}

type deviceAuthRateLimitEntry struct {
	count       int
	windowStart time.Time
}

func newDeviceAuthRateLimiter(limit int, window time.Duration) *deviceAuthRateLimiter {
	l := &deviceAuthRateLimiter{
		limit:       limit,
		window:      window,
		hits:        make(map[string]*deviceAuthRateLimitEntry),
		lastCleanup: time.Now(),
	}
	return l
}

func (l *deviceAuthRateLimiter) cleanupExpiredLocked(now time.Time) {
	for key, entry := range l.hits {
		if now.Sub(entry.windowStart) >= l.window {
			delete(l.hits, key)
		}
	}
}

// close is kept for controller lifecycle compatibility.
func (l *deviceAuthRateLimiter) close() {
}

func (l *deviceAuthRateLimiter) Allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastCleanup) >= l.window {
		l.cleanupExpiredLocked(now)
		l.lastCleanup = now
	}

	entry, ok := l.hits[key]
	if !ok || now.Sub(entry.windowStart) >= l.window {
		l.hits[key] = &deviceAuthRateLimitEntry{count: 1, windowStart: now}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	return true
}
