package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"shadmin/internal/constants"
	"strings"
	"time"

	"shadmin/domain"
	"shadmin/internal/tokenservice"
)

const (
	deviceAuthExpiresIn          = 900
	deviceAuthDefaultInterval    = 5
	deviceAuthMaxCreateAttempts  = 5
	deviceAuthMaxInvalidAttempts = 5
	userCodeAlphabet             = "BCDFGHJKLMNPQRSTVWXYZ23456789"
)

type deviceAuthUsecase struct {
	repo               domain.DeviceAuthRepository
	userRepository     domain.UserRepository
	tokenService       *tokenservice.TokenService
	accessTokenSecret  string
	refreshTokenSecret string
	accessTokenExpiry  int
	refreshTokenExpiry int
	contextTimeout     time.Duration
}

func NewDeviceAuthUsecase(
	repo domain.DeviceAuthRepository,
	userRepository domain.UserRepository,
	tokenService *tokenservice.TokenService,
	accessTokenSecret string,
	refreshTokenSecret string,
	accessTokenExpiry int,
	refreshTokenExpiry int,
	timeout time.Duration,
) domain.DeviceAuthUsecase {
	return &deviceAuthUsecase{
		repo:               repo,
		userRepository:     userRepository,
		tokenService:       tokenService,
		accessTokenSecret:  accessTokenSecret,
		refreshTokenSecret: refreshTokenSecret,
		accessTokenExpiry:  accessTokenExpiry,
		refreshTokenExpiry: refreshTokenExpiry,
		contextTimeout:     timeout,
	}
}

func (u *deviceAuthUsecase) RequestCode(ctx context.Context, req domain.DeviceCodeRequest, verificationURI string) (*domain.DeviceCodeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	_ = u.repo.DeleteExpired(ctx, time.Now())

	clientName := strings.TrimSpace(req.ClientName)
	if clientName == "" {
		clientName = req.ClientID
	}

	for i := 0; i < deviceAuthMaxCreateAttempts; i++ {
		deviceCode, err := generateDeviceCode()
		if err != nil {
			return nil, err
		}
		userCode, err := generateUserCode()
		if err != nil {
			return nil, err
		}

		session := &domain.DeviceAuthSession{
			DeviceCode:      deviceCode,
			UserCode:        userCode,
			ClientID:        req.ClientID,
			ClientName:      clientName,
			Status:          domain.DeviceAuthStatusPending,
			Interval:        deviceAuthDefaultInterval,
			InvalidAttempts: 0,
			ExpiresAt:       time.Now().Add(deviceAuthExpiresIn * time.Second),
		}
		if err := u.repo.Create(ctx, session); err != nil {
			if errors.Is(err, domain.ErrDeviceCodeConflict) {
				continue
			}
			return nil, fmt.Errorf("create device auth session: %w", err)
		}

		return &domain.DeviceCodeResponse{
			DeviceCode:      session.DeviceCode,
			UserCode:        session.UserCode,
			VerificationURI: verificationURI,
			ExpiresIn:       deviceAuthExpiresIn,
			Interval:        session.Interval,
		}, nil
	}

	return nil, fmt.Errorf("generate unique device auth code: %w", domain.ErrDeviceInvalidCode)
}

func (u *deviceAuthUsecase) PollToken(ctx context.Context, req domain.DeviceTokenRequest) (*domain.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	now := time.Now()
	session, err := u.repo.GetByDeviceCode(ctx, req.DeviceCode)
	if err != nil {
		if errors.Is(err, domain.ErrDeviceInvalidCode) {
			return nil, domain.ErrDeviceInvalidCode
		}
		return nil, fmt.Errorf("get device auth session: %w", err)
	}
	if session.ClientID != req.ClientID {
		return nil, domain.ErrDeviceInvalidCode
	}
	if !session.ExpiresAt.After(now) {
		_ = u.repo.Expire(ctx, session.DeviceCode, now)
		return nil, domain.ErrDeviceExpired
	}

	switch session.Status {
	case domain.DeviceAuthStatusPending:
		if session.LastPolledAt != nil && now.Sub(*session.LastPolledAt) < time.Duration(session.Interval)*time.Second {
			nextInterval := session.Interval + 5
			if err := u.repo.UpdatePollState(ctx, session.DeviceCode, now, nextInterval); err != nil {
				log.Printf("WARN: update device poll interval on slow_down: %v", err)
			}
			return nil, domain.ErrDeviceSlowDown
		}
		if err := u.repo.UpdatePollState(ctx, session.DeviceCode, now, session.Interval); err != nil {
			return nil, fmt.Errorf("update device poll state: %w", err)
		}
		return nil, domain.ErrDeviceAuthorizationPending
	case domain.DeviceAuthStatusAuthorized:
		consumed, err := u.repo.ConsumeAuthorized(ctx, session.DeviceCode, now)
		if err != nil {
			if errors.Is(err, domain.ErrDeviceConsumed) {
				return nil, domain.ErrDeviceConsumed
			}
			return nil, fmt.Errorf("consume device auth session: %w", err)
		}
		user, err := u.userRepository.GetByID(ctx, consumed.UserID)
		if err != nil {
			return nil, fmt.Errorf("get authorized user: %w", err)
		}
		// 被禁用的账户不应通过 CLI 拿到令牌。
		if user.Status != constants.UserStatusActive {
			return nil, domain.ErrUserDisabled
		}
		accessToken, err := u.tokenService.CreateAccessToken(user, u.accessTokenSecret, u.accessTokenExpiry)
		if err != nil {
			return nil, fmt.Errorf("create access token: %w", err)
		}
		refreshToken, err := u.tokenService.CreateRefreshToken(user, u.refreshTokenSecret, u.refreshTokenExpiry)
		if err != nil {
			return nil, fmt.Errorf("create refresh token: %w", err)
		}
		return &domain.LoginResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		}, nil
	case domain.DeviceAuthStatusConsumed:
		return nil, domain.ErrDeviceConsumed
	case domain.DeviceAuthStatusExpired:
		return nil, domain.ErrDeviceExpired
	case domain.DeviceAuthStatusDenied:
		return nil, domain.ErrDeviceAccessDenied
	default:
		return nil, domain.ErrDeviceInvalidCode
	}
}

func (u *deviceAuthUsecase) Activate(ctx context.Context, userID string, req domain.DeviceActivateRequest) (*domain.DeviceActivateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, u.contextTimeout)
	defer cancel()

	now := time.Now()
	userCode := normalizeUserCode(req.UserCode)
	session, err := u.repo.GetByUserCode(ctx, userCode)
	if err != nil {
		if errors.Is(err, domain.ErrDeviceInvalidCode) {
			return nil, domain.ErrDeviceInvalidCode
		}
		return nil, fmt.Errorf("get device auth session by user code: %w", err)
	}
	if !session.ExpiresAt.After(now) {
		_ = u.repo.Expire(ctx, session.DeviceCode, now)
		u.recordInvalidActivation(ctx, userCode)
		return nil, domain.ErrDeviceExpired
	}
	if session.InvalidAttempts >= deviceAuthMaxInvalidAttempts {
		_ = u.repo.Deny(ctx, userCode)
		return nil, domain.ErrDeviceAccessDenied
	}
	if session.Status != domain.DeviceAuthStatusPending {
		u.recordInvalidActivation(ctx, userCode)
		return nil, domain.ErrDeviceInvalidCode
	}
	if err := u.repo.MarkAuthorized(ctx, userCode, userID, now); err != nil {
		if errors.Is(err, domain.ErrDeviceInvalidCode) {
			u.recordInvalidActivation(ctx, userCode)
			return nil, domain.ErrDeviceInvalidCode
		}
		return nil, fmt.Errorf("authorize device auth session: %w", err)
	}

	return &domain.DeviceActivateResponse{Status: domain.DeviceAuthStatusAuthorized}, nil
}

func (u *deviceAuthUsecase) recordInvalidActivation(ctx context.Context, userCode string) {
	session, err := u.repo.IncrementInvalidAttempts(ctx, userCode)
	if err != nil || session == nil {
		return
	}
	if session.InvalidAttempts >= deviceAuthMaxInvalidAttempts {
		_ = u.repo.Deny(ctx, userCode)
	}
}

func generateDeviceCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate device code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateUserCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate user code: %w", err)
	}
	var builder strings.Builder
	for i, v := range b {
		if i == 4 {
			builder.WriteByte('-')
		}
		builder.WriteByte(userCodeAlphabet[int(v)%len(userCodeAlphabet)])
	}
	return builder.String(), nil
}

func normalizeUserCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, " ", "")
	if len(code) == 8 && !strings.Contains(code, "-") {
		return code[:4] + "-" + code[4:]
	}
	return code
}
