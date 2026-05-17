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

// GenerateSlide 生成新的 Slide 验证码挑战
func (u *captchaUsecase) GenerateSlide(ctx context.Context, oldID string) (*domain.SlideCaptchaChallenge, error) {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	type result struct {
		challenge *domain.SlideCaptchaChallenge
		err       error
	}
	ch := make(chan result, 1)
	go func() {
		challenge, err := u.manager.Generate(oldID)
		ch <- result{challenge: challenge, err: err}
	}()

	select {
	case r := <-ch:
		return r.challenge, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// VerifySlide 校验用户提交的滑块坐标
func (u *captchaUsecase) VerifySlide(ctx context.Context, id string, x, y int) error {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		ch <- u.manager.Verify(id, x, y)
	}()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// InvalidateSlide 主动失效一个 challenge
func (u *captchaUsecase) InvalidateSlide(_ context.Context, id string) {
	u.manager.Invalidate(id)
}
