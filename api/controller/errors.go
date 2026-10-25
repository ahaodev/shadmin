package controller

import (
	"errors"
	"log/slog"
	"net/http"

	"shadmin/domain"

	"github.com/gin-gonic/gin"
)

// errorRule 把一个 usecase 哨兵错误映射为 HTTP 状态码。
// msg 为空时响应体使用完整的 err.Error()（包含 usecase 追加的上下文，如“还可尝试 N 次”）；
// 非空时使用固定文案。
type errorRule struct {
	err    error
	status int
	msg    string
}

// errorRules 是 usecase 错误 → HTTP 响应的唯一映射表，按顺序用 errors.Is 匹配。
// 新增哨兵错误只需在此登记一行；未登记的错误一律视为 500 并只记录日志，不回传内部细节。
var errorRules = []errorRule{
	// 400 参数 / 状态不合法
	{domain.ErrInvalidDictStatus, http.StatusBadRequest, "Invalid dictionary status"},
	{domain.ErrInvalidMenuType, http.StatusBadRequest, "Invalid menu type"},
	{domain.ErrInvalidMenuStatus, http.StatusBadRequest, "Invalid menu status"},
	{domain.ErrInvalidPassword, http.StatusBadRequest, "Current password is incorrect"},
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
	{domain.ErrInvitationPasswordInvalid, http.StatusBadRequest, "密码长度必须为 8 到 72 字节"},
	// 设备授权码流程：CLI 依赖响应体里的原文（authorization_pending / slow_down / ...）
	{domain.ErrDeviceAuthorizationPending, http.StatusBadRequest, ""},
	{domain.ErrDeviceSlowDown, http.StatusBadRequest, ""},
	{domain.ErrDeviceExpired, http.StatusBadRequest, ""},
	{domain.ErrDeviceInvalidCode, http.StatusBadRequest, ""},
	{domain.ErrDeviceConsumed, http.StatusBadRequest, ""},
	{domain.ErrDeviceAccessDenied, http.StatusBadRequest, ""},
	{domain.ErrUserIdentityProviderDisabled, http.StatusBadRequest, ""},

	// 401 未认证
	{domain.ErrInvalidCredentials, http.StatusUnauthorized, ""},
	{domain.ErrInvalidRefreshToken, http.StatusUnauthorized, "Invalid refresh token"},
	{domain.ErrRefreshTokenRevoked, http.StatusUnauthorized, ""},

	// 403 禁止
	{domain.ErrAccountInactive, http.StatusForbidden, ""},
	{domain.ErrUserDisabled, http.StatusForbidden, ""},
	{domain.ErrCannotDeleteSelf, http.StatusForbidden, ""},
	{domain.ErrCannotDeleteAdmin, http.StatusForbidden, ""},
	{domain.ErrCannotEditAdmin, http.StatusForbidden, ""},
	{domain.ErrCannotDeleteAdminRole, http.StatusForbidden, ""},
	{domain.ErrCannotRenameAdminRole, http.StatusForbidden, ""},

	// 404 不存在
	{domain.ErrUserNotFound, http.StatusNotFound, "User not found"},
	{domain.ErrRoleNotFound, http.StatusNotFound, ""},
	{domain.ErrMenuNotFound, http.StatusNotFound, "Menu not found"},
	{domain.ErrDepartmentNotFound, http.StatusNotFound, ""},
	{domain.ErrDictTypeNotFound, http.StatusNotFound, "Dictionary type not found"},
	{domain.ErrDictItemNotFound, http.StatusNotFound, "Dictionary item not found"},
	{domain.ErrUserIdentityCodeInvalid, http.StatusNotFound, ""},

	// 409 冲突
	{domain.ErrRoleNameExists, http.StatusConflict, ""},
	{domain.ErrRoleInUse, http.StatusConflict, ""},
	{domain.ErrDepartmentNameExists, http.StatusConflict, ""},
	{domain.ErrDictTypeCodeExists, http.StatusConflict, "Dictionary type code already exists"},
	{domain.ErrDictItemValueExists, http.StatusConflict, "Dictionary item value already exists in this type"},
	{domain.ErrDictItemDefaultConflict, http.StatusConflict, "Dictionary type already has a default item"},
	{domain.ErrUserExists, http.StatusConflict, ""},
	{domain.ErrUserAlreadyExists, http.StatusConflict, "该邮箱已关联现有用户"},
	{domain.ErrUsernameExists, http.StatusConflict, "用户名已被使用"},

	// 5xx 依赖服务
	{domain.ErrInvitationEmailNotConfigured, http.StatusServiceUnavailable, "邀请邮件服务未配置"},
	{domain.ErrInvitationDeliveryFailed, http.StatusBadGateway, "邀请已保存，但邮件发送失败；可重新提交以重试"},
}

// RespondError 把 usecase 返回的错误写成统一响应，是 controller 处理 usecase 错误的唯一出口。
//   - 账户锁定（*domain.AccountLockedError）→ 423
//   - 已登记的哨兵错误 → 对应状态码
//   - 其他错误 → 500 通用文案；原始错误只写日志，不回传（可能含 SQL / 驱动细节）
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

// classifyError 返回错误对应的 HTTP 状态码与对外文案。
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
