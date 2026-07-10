package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/rs/xid"
	assetImages "github.com/wenlng/go-captcha-assets/resources/imagesv2"
	assetTiles "github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/slide"

	"shadmin/domain"
	"shadmin/internal/cacher"
)

const (
	// DefaultTTL 默认验证码过期时间
	DefaultTTL = 15 * time.Second
	// DefaultPadding Slide 校验坐标容忍度（像素）
	DefaultPadding = 5
	// DefaultMaxAttempts 单个 challenge 最多校验尝试次数
	DefaultMaxAttempts = 3
)

// challengeRecord 单次滑块挑战的服务端状态。JSON 序列化用于底层 cacher.Cacher 持久化。
type challengeRecord struct {
	X         int       `json:"x"`
	Y         int       `json:"y"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
}

const captchaNS = "captcha"

// SlideManager 进程内 Slide 验证码生成与挑战存储，封装 go-captcha v2 与 challenge 生命周期。
// 它是基础设施组件，与 internal/login_security.go 同级，由 usecase 层包装后供 controller 使用。
type SlideManager struct {
	captcha    slide.Captcha
	cacher     cacher.Cacher
	ttl        time.Duration
	padding    int
	maxAttempt int
}

func NewSlideManager(cacher cacher.Cacher) (*SlideManager, error) {
	if cacher == nil {
		return nil, errors.New("captcha: cacher is required")
	}
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
		cacher:     cacher,
		ttl:        DefaultTTL,
		padding:    DefaultPadding,
		maxAttempt: DefaultMaxAttempts,
	}, nil
}

// Generate 生成新的 Slide 验证码挑战；oldID 非空时会主动失效旧 challenge
func (m *SlideManager) Generate(oldID string) (*domain.SlideCaptchaChallenge, error) {
	if oldID != "" {
		_ = m.cacher.Delete(context.Background(), captchaNS, oldID)
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
	if err := m.saveChallenge(id, rec, m.ttl); err != nil {
		return nil, err
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

// Verify 校验用户提交的滑块坐标，校验成功或失败次数耗尽后会消费掉该 challenge。
func (m *SlideManager) Verify(id string, x, y int) error {
	if id == "" {
		return domain.ErrCaptchaRequired
	}

	v, ok, err := m.cacher.GetAndDelete(context.Background(), captchaNS, id)
	if err != nil {
		return fmt.Errorf("load challenge: %w", err)
	}
	if !ok {
		return domain.ErrCaptchaInvalid
	}

	var rec challengeRecord
	if err := json.Unmarshal([]byte(v), &rec); err != nil {
		return fmt.Errorf("unmarshal challenge: %w", err)
	}
	if time.Now().After(rec.ExpiresAt) {
		return domain.ErrCaptchaExpired
	}

	rec.Attempts++
	if !slide.Validate(x, y, rec.X, rec.Y, m.padding) {
		// 仍有剩余次数则写回自增后的计数；否则保持已删除（次数耗尽即失效）。
		if rec.Attempts < m.maxAttempt {
			if err := m.saveChallenge(id, rec, time.Until(rec.ExpiresAt)); err != nil {
				return domain.ErrCaptchaInvalid
			}
		}
		return domain.ErrCaptchaInvalid
	}

	// 一次性消费：认领时已删除，校验成功直接返回。
	return nil
}

func (m *SlideManager) saveChallenge(id string, rec challengeRecord, ttl time.Duration) error {
	if ttl < 0 {
		ttl = 0
	}
	if ttl == 0 {
		return nil
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal challenge: %w", err)
	}
	if err := m.cacher.Set(context.Background(), captchaNS, id, string(b), ttl); err != nil {
		return fmt.Errorf("save challenge: %w", err)
	}
	return nil
}

// Invalidate 主动失效一个 challenge
func (m *SlideManager) Invalidate(id string) {
	if id == "" {
		return
	}
	_ = m.cacher.Delete(context.Background(), captchaNS, id)
}
