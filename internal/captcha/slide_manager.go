package captcha

import (
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/rs/xid"
	assetImages "github.com/wenlng/go-captcha-assets/resources/imagesv2"
	assetTiles "github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/slide"

	"shadmin/domain"
)

const (
	// DefaultTTL 默认验证码过期时间
	DefaultTTL = 15 * time.Second
	// DefaultPadding Slide 校验坐标容忍度（像素）
	DefaultPadding = 5
	// DefaultMaxAttempts 单个 challenge 最多校验尝试次数
	DefaultMaxAttempts = 3
)

// SlideManager 进程内 Slide 验证码生成与挑战存储，封装 go-captcha v2 与 challenge 生命周期。
// 它是基础设施组件，与 internal/login_security.go 同级，由 usecase 层包装后供 controller 使用。
type SlideManager struct {
	captcha    slide.Captcha
	store      ChallengeStore
	ttl        time.Duration
	padding    int
	maxAttempt int
}

// NewSlideManager 创建 SlideManager；不可用时返回错误，调用方应将其作为致命错误处理。
// store 决定 challenge 落地：进程内存或 Redis，由调用方注入的 cachex.Cacher 后端决定。
func NewSlideManager(store ChallengeStore) (*SlideManager, error) {
	backgrounds, err := assetImages.GetImages()
	if err != nil {
		return nil, fmt.Errorf("load captcha backgrounds: %w", err)
	}
	tiles, err := assetTiles.GetTiles()
	if err != nil {
		return nil, fmt.Errorf("load captcha tiles: %w", err)
	}

	graphs := make([]*slide.GraphImage, 0, len(tiles))
	for _, t := range tiles {
		graphs = append(graphs, &slide.GraphImage{
			OverlayImage: t.OverlayImage,
			ShadowImage:  t.ShadowImage,
			MaskImage:    t.MaskImage,
		})
	}

	builder := slide.NewBuilder()
	builder.SetResources(
		slide.WithBackgrounds([]image.Image(backgrounds)),
		slide.WithGraphImages(graphs),
	)

	return &SlideManager{
		captcha:    builder.Make(),
		store:      store,
		ttl:        DefaultTTL,
		padding:    DefaultPadding,
		maxAttempt: DefaultMaxAttempts,
	}, nil
}

// Close 释放底层存储资源（如内存 store 的清理 goroutine）。
func (m *SlideManager) Close() {
	_ = m.store.Close()
}

// Generate 生成新的 Slide 验证码挑战；oldID 非空时会主动失效旧 challenge
func (m *SlideManager) Generate(oldID string) (*domain.SlideCaptchaChallenge, error) {
	if oldID != "" {
		_ = m.store.Delete(oldID)
	}

	captData, err := m.captcha.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate slide captcha: %w", err)
	}
	block := captData.GetData()
	if block == nil {
		return nil, errors.New("generate slide captcha: empty block")
	}

	master := captData.GetMasterImage()
	tile := captData.GetTileImage()
	masterB64, err := master.ToBase64()
	if err != nil {
		return nil, fmt.Errorf("encode master image: %w", err)
	}
	tileB64, err := tile.ToBase64()
	if err != nil {
		return nil, fmt.Errorf("encode tile image: %w", err)
	}

	id := xid.New().String()
	rec := challengeRecord{
		X:         block.X,
		Y:         block.Y,
		ExpiresAt: time.Now().Add(m.ttl),
	}
	if err := m.store.Save(id, rec, m.ttl); err != nil {
		return nil, fmt.Errorf("save challenge: %w", err)
	}

	imageSize := m.captcha.GetOptions().GetImageSize()

	return &domain.SlideCaptchaChallenge{
		CaptchaID:    id,
		MasterImage:  masterB64,
		TileImage:    tileB64,
		TileX:        block.DX,
		TileY:        block.DY,
		TileWidth:    block.Width,
		TileHeight:   block.Height,
		MasterWidth:  imageSize.Width,
		MasterHeight: imageSize.Height,
		ExpiresIn:    int(m.ttl / time.Second),
	}, nil
}

// Verify 校验用户提交的滑块坐标，校验成功或失败次数耗尽后会消费掉该 challenge
func (m *SlideManager) Verify(id string, x, y int) error {
	if id == "" {
		return domain.ErrCaptchaRequired
	}

	rec, ok, err := m.store.Load(id)
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok {
		return domain.ErrCaptchaInvalid
	}
	if rec.Used {
		_ = m.store.Delete(id)
		return domain.ErrCaptchaInvalid
	}
	if time.Now().After(rec.ExpiresAt) {
		_ = m.store.Delete(id)
		return domain.ErrCaptchaExpired
	}

	rec.Attempts++
	if !slide.Validate(x, y, rec.X, rec.Y, m.padding) {
		if rec.Attempts >= m.maxAttempt {
			_ = m.store.Delete(id)
		} else {
			_ = m.store.Save(id, rec, time.Until(rec.ExpiresAt))
		}
		return domain.ErrCaptchaInvalid
	}

	// 一次性消费：验证成功后立即失效
	_ = m.store.Delete(id)
	return nil
}

// Invalidate 主动失效一个 challenge
func (m *SlideManager) Invalidate(id string) {
	if id == "" {
		return
	}
	_ = m.store.Delete(id)
}
