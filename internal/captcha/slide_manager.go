package captcha

import (
	"errors"
	"fmt"
	"image"
	"sync"
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
	// cleanupInterval 后台清理过期 challenge 的间隔
	cleanupInterval = 30 * time.Second
)

type challengeRecord struct {
	x         int
	y         int
	expiresAt time.Time
	used      bool
	attempts  int
}

// SlideManager 进程内 Slide 验证码生成与挑战存储，封装 go-captcha v2 与 challenge 生命周期。
// 它是基础设施组件，与 internal/login_security.go 同级，由 usecase 层包装后供 controller 使用。
type SlideManager struct {
	captcha    slide.Captcha
	mu         sync.Mutex
	challenges map[string]*challengeRecord
	ttl        time.Duration
	padding    int
	maxAttempt int
	stopCh     chan struct{}
	stopOnce   sync.Once
}

// NewSlideManager 创建 SlideManager；不可用时返回错误，调用方应将其作为致命错误处理
func NewSlideManager() (*SlideManager, error) {
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

	m := &SlideManager{
		captcha:    builder.Make(),
		challenges: make(map[string]*challengeRecord),
		ttl:        DefaultTTL,
		padding:    DefaultPadding,
		maxAttempt: DefaultMaxAttempts,
		stopCh:     make(chan struct{}),
	}

	go m.startCleanup()

	return m, nil
}

// Close 停止后台清理 goroutine，多次调用安全
func (m *SlideManager) Close() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

// Generate 生成新的 Slide 验证码挑战；oldID 非空时会主动失效旧 challenge
func (m *SlideManager) Generate(oldID string) (*domain.SlideCaptchaChallenge, error) {
	if oldID != "" {
		m.deleteChallenge(oldID)
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

	m.mu.Lock()
	m.challenges[id] = &challengeRecord{
		x:         block.X,
		y:         block.Y,
		expiresAt: time.Now().Add(m.ttl),
	}
	m.mu.Unlock()

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

	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.challenges[id]
	if !ok {
		return domain.ErrCaptchaInvalid
	}
	if rec.used {
		delete(m.challenges, id)
		return domain.ErrCaptchaInvalid
	}
	if time.Now().After(rec.expiresAt) {
		delete(m.challenges, id)
		return domain.ErrCaptchaExpired
	}

	rec.attempts++
	if !slide.Validate(x, y, rec.x, rec.y, m.padding) {
		if rec.attempts >= m.maxAttempt {
			delete(m.challenges, id)
		}
		return domain.ErrCaptchaInvalid
	}

	// 一次性消费：验证成功后立即失效
	rec.used = true
	delete(m.challenges, id)
	return nil
}

// Invalidate 主动失效一个 challenge
func (m *SlideManager) Invalidate(id string) {
	if id == "" {
		return
	}
	m.deleteChallenge(id)
}

func (m *SlideManager) deleteChallenge(id string) {
	m.mu.Lock()
	delete(m.challenges, id)
	m.mu.Unlock()
}

func (m *SlideManager) startCleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpired()
		case <-m.stopCh:
			return
		}
	}
}

func (m *SlideManager) cleanupExpired() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rec := range m.challenges {
		if rec.used || now.After(rec.expiresAt) {
			delete(m.challenges, id)
		}
	}
}
