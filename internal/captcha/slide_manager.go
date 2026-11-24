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

	cacher "github.com/ahaodev/go-cache"
)

const (
	DefaultTTL = 15 * time.Second
	// DefaultPadding 为坐标容差，单位为像素。
	DefaultPadding     = 5
	DefaultMaxAttempts = 3
)

type challengeRecord struct {
	X         int       `json:"x"`
	Y         int       `json:"y"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
}

const captchaNS = "captcha"

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

// Generate 在 oldID 非空时失效旧挑战。
func (m *SlideManager) Generate(ctx context.Context, oldID string) (*domain.SlideCaptchaChallenge, error) {
	if oldID != "" {
		_ = m.cacher.Delete(ctx, captchaNS, oldID)
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
	if err := m.saveChallenge(ctx, id, rec, m.ttl); err != nil {
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

// Verify 在成功或尝试次数耗尽时消费挑战。
func (m *SlideManager) Verify(ctx context.Context, id string, x, y int) error {
	if id == "" {
		return domain.ErrCaptchaRequired
	}

	v, ok, err := m.cacher.GetAndDelete(ctx, captchaNS, id)
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
		if rec.Attempts < m.maxAttempt {
			if err := m.saveChallenge(ctx, id, rec, time.Until(rec.ExpiresAt)); err != nil {
				return domain.ErrCaptchaInvalid
			}
		}
		return domain.ErrCaptchaInvalid
	}

	return nil
}

func (m *SlideManager) saveChallenge(ctx context.Context, id string, rec challengeRecord, ttl time.Duration) error {
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
	if err := m.cacher.Set(ctx, captchaNS, id, string(b), ttl); err != nil {
		return fmt.Errorf("save challenge: %w", err)
	}
	return nil
}

func (m *SlideManager) Invalidate(ctx context.Context, id string) {
	if id == "" {
		return
	}
	_ = m.cacher.Delete(ctx, captchaNS, id)
}
