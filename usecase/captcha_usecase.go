package usecase

import (
	"context"
	"time"

	"shadmin/domain"
)

type captchaUsecase struct {
	manager domain.SlideCaptchaManager
	timeout time.Duration
}

func NewCaptchaUsecase(manager domain.SlideCaptchaManager, timeout time.Duration) domain.CaptchaUsecase {
	return &captchaUsecase{
		manager: manager,
		timeout: timeout,
	}
}

// GenerateSlide 同步生成图像，避免超时后留下不可取消的 goroutine。
func (u *captchaUsecase) GenerateSlide(ctx context.Context, oldID string) (*domain.SlideCaptchaChallenge, error) {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	return u.manager.Generate(ctx, oldID)
}

func (u *captchaUsecase) VerifySlide(ctx context.Context, id string, x, y int) error {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	return u.manager.Verify(ctx, id, x, y)
}

func (u *captchaUsecase) InvalidateSlide(ctx context.Context, id string) {
	u.manager.Invalidate(ctx, id)
}
