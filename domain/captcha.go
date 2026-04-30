package domain

import (
	"context"
	"errors"
)

// SlideCaptchaChallenge Slide 模式验证码挑战返回给前端的数据
type SlideCaptchaChallenge struct {
	CaptchaID    string `json:"captcha_id"`
	MasterImage  string `json:"master_image"`
	TileImage    string `json:"tile_image"`
	TileX        int    `json:"tile_x"`
	TileY        int    `json:"tile_y"`
	TileWidth    int    `json:"tile_width"`
	TileHeight   int    `json:"tile_height"`
	MasterWidth  int    `json:"master_width"`
	MasterHeight int    `json:"master_height"`
	ExpiresIn    int    `json:"expires_in"`
}

// CaptchaUsecase Slide 验证码用例接口
type CaptchaUsecase interface {
	GenerateSlide(ctx context.Context, oldID string) (*SlideCaptchaChallenge, error)
	VerifySlide(ctx context.Context, id string, x, y int) error
	InvalidateSlide(ctx context.Context, id string)
}

// 验证码相关错误
var (
	ErrCaptchaRequired = errors.New("captcha required")
	ErrCaptchaInvalid  = errors.New("captcha invalid")
	ErrCaptchaExpired  = errors.New("captcha expired")
)
