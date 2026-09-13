package usecase

import (
	"context"
	"time"

	"shadmin/domain"
	captchapkg "shadmin/internal/captcha"
)

// captchaUsecase 包装 SlideManager，添加 context 超时控制以遵循项目 usecase 层约定
type captchaUsecase struct {
	manager *captchapkg.SlideManager
	timeout time.Duration
}

// NewCaptchaUsecase 创建 Slide 验证码用例
func NewCaptchaUsecase(manager *captchapkg.SlideManager, timeout time.Duration) domain.CaptchaUsecase {
	return &captchaUsecase{
		manager: manager,
		timeout: timeout,
	}
}

// GenerateSlide 生成新的 Slide 验证码挑战。
// 直接同步调用：图像生成本身不可取消，起 goroutine 只会在超时后留下无法回收的残留工作。
func (u *captchaUsecase) GenerateSlide(ctx context.Context, oldID string) (*domain.SlideCaptchaChallenge, error) {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	return u.manager.Generate(ctx, oldID)
}

// VerifySlide 校验用户提交的滑块坐标
func (u *captchaUsecase) VerifySlide(ctx context.Context, id string, x, y int) error {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	return u.manager.Verify(ctx, id, x, y)
}

// InvalidateSlide 主动失效一个 challenge
func (u *captchaUsecase) InvalidateSlide(ctx context.Context, id string) {
	u.manager.Invalidate(ctx, id)
}
