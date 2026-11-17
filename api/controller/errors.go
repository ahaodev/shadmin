package controller

import (
	"errors"
	"log/slog"
	"net/http"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

// errorRule 的 msg 为空时透传错误文案，否则使用固定文案。
type errorRule struct {
	err    error
	status int
	msg    string
}

// errorRules 按顺序用 errors.Is 匹配；未登记的错误返回 500。
var errorRules = []errorRule{
	{domain.ErrInvalidDictStatus, http.StatusBadRequest, "Invalid dictionary status"},
	{domain.ErrInvalidMenuType, http.StatusBadRequest, "Invalid menu type"},
	{domain.ErrInvalidMenuStatus, http.StatusBadRequest, "Invalid menu status"},
	{domain.ErrInvalidPassword, http.StatusBadRequest, "Current password is incorrect"},
	{domain.ErrPasswordLength, http.StatusBadRequest, ""},
	{domain.ErrCaptchaRequired, http.StatusBadRequest, "请先完成验证码"},
	{domain.ErrCaptchaExpired, http.StatusBadRequest, "验证码已过期，请刷新"},
	{domain.ErrCaptchaInvalid, http.StatusBadRequest, "验证码校验失败，请重试"},
	{domain.ErrCircularDepartment, http.StatusBadRequest, ""},
	{domain.ErrDepartmentHasActiveChildren, http.StatusBadRequest, ""},
	{domain.ErrDepartmentHasChildren, http.StatusBadRequest, ""},
	{domain.ErrDepartmentHasUsers, http.StatusBadRequest, ""},
	{domain.ErrDictTypeHasItems, http.StatusBadRequest, "Dictionary type has items, cannot delete"},
	{domain.ErrMenuHasChildren, http.StatusBadRequest, "Cannot delete menu with children"},
	{domain.ErrInvalidInvitation, http.StatusBadRequest, "邀请链接无效或已过期"},
	{domain.ErrInvitationRoleInvalid, http.StatusBadRequest, "所选角色不可用"},
	{domain.ErrInvitationPasswordInvalid, http.StatusBadRequest, domain.ErrPasswordLength.Error()},
	// CLI 依赖设备授权错误原文。
	{domain.ErrDeviceAuthorizationPending, http.StatusBadRequest, ""},
	{domain.ErrDeviceSlowDown, http.StatusBadRequest, ""},
	{domain.ErrDeviceExpired, http.StatusBadRequest, ""},
	{domain.ErrDeviceInvalidCode, http.StatusBadRequest, ""},
	{domain.ErrDeviceConsumed, http.StatusBadRequest, ""},
	{domain.ErrDeviceAccessDenied, http.StatusBadRequest, ""},
	{domain.ErrUserIdentityProviderDisabled, http.StatusBadRequest, ""},

	{domain.ErrInvalidCredentials, http.StatusUnauthorized, ""},
	{domain.ErrInvalidRefreshToken, http.StatusUnauthorized, "Invalid refresh token"},
	{domain.ErrRefreshTokenRevoked, http.StatusUnauthorized, ""},

	{domain.ErrAccountInactive, http.StatusForbidden, ""},
	{domain.ErrUserDisabled, http.StatusForbidden, ""},
	{domain.ErrCannotDeleteSelf, http.StatusForbidden, ""},
	{domain.ErrCannotDeleteAdmin, http.StatusForbidden, ""},
	{domain.ErrCannotEditAdmin, http.StatusForbidden, ""},
	{domain.ErrCannotDeleteAdminRole, http.StatusForbidden, ""},
	{domain.ErrCannotRenameAdminRole, http.StatusForbidden, ""},

	{domain.ErrUserNotFound, http.StatusNotFound, "User not found"},
	{domain.ErrRoleNotFound, http.StatusNotFound, ""},
	{domain.ErrMenuNotFound, http.StatusNotFound, "Menu not found"},
	{domain.ErrDepartmentNotFound, http.StatusNotFound, ""},
	{domain.ErrDictTypeNotFound, http.StatusNotFound, "Dictionary type not found"},
	{domain.ErrDictItemNotFound, http.StatusNotFound, "Dictionary item not found"},
	{domain.ErrUserIdentityCodeInvalid, http.StatusNotFound, ""},

	{domain.ErrRoleNameExists, http.StatusConflict, ""},
	{domain.ErrRoleInUse, http.StatusConflict, ""},
	{domain.ErrDepartmentNameExists, http.StatusConflict, ""},
	{domain.ErrDictTypeCodeExists, http.StatusConflict, "Dictionary type code already exists"},
	{domain.ErrDictItemValueExists, http.StatusConflict, "Dictionary item value already exists in this type"},
	{domain.ErrDictItemLabelExists, http.StatusConflict, "Dictionary item label already exists in this type"},
	{domain.ErrDictItemDefaultConflict, http.StatusConflict, "Dictionary type already has a default item"},
	{domain.ErrUserExists, http.StatusConflict, ""},
	{domain.ErrUserAlreadyExists, http.StatusConflict, "该邮箱已关联现有用户"},
	{domain.ErrUsernameExists, http.StatusConflict, "用户名已被使用"},

	{domain.ErrInvitationEmailNotConfigured, http.StatusServiceUnavailable, "邀请邮件服务未配置"},
	{domain.ErrInvitationDeliveryFailed, http.StatusBadGateway, "邀请已保存，但邮件发送失败；可重新提交以重试"},
}

// RespondError 统一映射用例错误；未知错误仅写日志，不回传内部细节。
func RespondError(c *gin.Context, err error) {
	if lockedErr, ok := errors.AsType[*domain.AccountLockedError](err); ok {
		c.JSON(http.StatusLocked, domain.RespError(lockedErr.Error()))
		return
	}

	status, msg := classifyError(err)
	if status == http.StatusInternalServerError {
		slog.Error("request failed", "method", c.Request.Method, "path", c.FullPath(), "error", err)
	}
	c.JSON(status, domain.RespError(msg))
}

func classifyError(err error) (status int, msg string) {
	for _, r := range errorRules {
		if errors.Is(err, r.err) {
			if r.msg != "" {
				return r.status, r.msg
			}
			return r.status, err.Error()
		}
	}
	return http.StatusInternalServerError, "Internal server error"
}
