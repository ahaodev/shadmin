package repository

import (
	"context"
	"time"

	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/deviceauthsession"
)

type entDeviceAuthRepository struct {
	client *ent.Client
}

func NewDeviceAuthRepository(client *ent.Client) domain.DeviceAuthRepository {
	return &entDeviceAuthRepository{client: client}
}

func entDeviceAuthSessionToDomain(s *ent.DeviceAuthSession) *domain.DeviceAuthSession {
	if s == nil {
		return nil
	}

	session := &domain.DeviceAuthSession{
		ID:              s.ID,
		DeviceCode:      s.DeviceCode,
		UserCode:        s.UserCode,
		ClientID:        s.ClientID,
		ClientName:      s.ClientName,
		Status:          string(s.Status),
		UserID:          s.UserID,
		Interval:        s.Interval,
		InvalidAttempts: s.InvalidAttempts,
		ExpiresAt:       s.ExpiresAt,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
	if !s.LastPolledAt.IsZero() {
		session.LastPolledAt = &s.LastPolledAt
	}
	if !s.AuthorizedAt.IsZero() {
		session.AuthorizedAt = &s.AuthorizedAt
	}
	if !s.ConsumedAt.IsZero() {
		session.ConsumedAt = &s.ConsumedAt
	}
	return session
}

func (r *entDeviceAuthRepository) Create(ctx context.Context, session *domain.DeviceAuthSession) error {
	create := r.client.DeviceAuthSession.
		Create().
		SetDeviceCode(session.DeviceCode).
		SetUserCode(session.UserCode).
		SetClientID(session.ClientID).
		SetStatus(deviceauthsession.Status(session.Status)).
		SetInterval(session.Interval).
		SetInvalidAttempts(session.InvalidAttempts).
		SetExpiresAt(session.ExpiresAt)

	if session.ClientName != "" {
		create.SetClientName(session.ClientName)
	}
	if session.UserID != "" {
		create.SetUserID(session.UserID)
	}
	if session.LastPolledAt != nil {
		create.SetLastPolledAt(*session.LastPolledAt)
	}
	if session.AuthorizedAt != nil {
		create.SetAuthorizedAt(*session.AuthorizedAt)
	}
	if session.ConsumedAt != nil {
		create.SetConsumedAt(*session.ConsumedAt)
	}

	created, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return domain.ErrDeviceCodeConflict
		}
		return err
	}
	*session = *entDeviceAuthSessionToDomain(created)
	return nil
}

func (r *entDeviceAuthRepository) GetByDeviceCode(ctx context.Context, deviceCode string) (*domain.DeviceAuthSession, error) {
	session, err := r.client.DeviceAuthSession.
		Query().
		Where(deviceauthsession.DeviceCode(deviceCode)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrDeviceInvalidCode
		}
		return nil, err
	}
	return entDeviceAuthSessionToDomain(session), nil
}

func (r *entDeviceAuthRepository) GetByUserCode(ctx context.Context, userCode string) (*domain.DeviceAuthSession, error) {
	session, err := r.client.DeviceAuthSession.
		Query().
		Where(deviceauthsession.UserCode(userCode)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrDeviceInvalidCode
		}
		return nil, err
	}
	return entDeviceAuthSessionToDomain(session), nil
}

func (r *entDeviceAuthRepository) MarkAuthorized(ctx context.Context, userCode string, userID string, now time.Time) error {
	affected, err := r.client.DeviceAuthSession.
		Update().
		Where(
			deviceauthsession.UserCode(userCode),
			deviceauthsession.StatusEQ(deviceauthsession.StatusPending),
			deviceauthsession.ExpiresAtGT(now),
		).
		SetStatus(deviceauthsession.StatusAuthorized).
		SetUserID(userID).
		SetAuthorizedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrDeviceInvalidCode
	}
	return nil
}

func (r *entDeviceAuthRepository) ConsumeAuthorized(ctx context.Context, deviceCode string, now time.Time) (*domain.DeviceAuthSession, error) {
	affected, err := r.client.DeviceAuthSession.
		Update().
		Where(
			deviceauthsession.DeviceCode(deviceCode),
			deviceauthsession.StatusEQ(deviceauthsession.StatusAuthorized),
			deviceauthsession.ExpiresAtGT(now),
		).
		SetStatus(deviceauthsession.StatusConsumed).
		SetConsumedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, domain.ErrDeviceConsumed
	}
	return r.GetByDeviceCode(ctx, deviceCode)
}

func (r *entDeviceAuthRepository) UpdatePollState(ctx context.Context, deviceCode string, lastPolledAt time.Time, interval int) error {
	return r.client.DeviceAuthSession.
		Update().
		Where(deviceauthsession.DeviceCode(deviceCode)).
		SetLastPolledAt(lastPolledAt).
		SetInterval(interval).
		Exec(ctx)
}

func (r *entDeviceAuthRepository) IncrementInvalidAttempts(ctx context.Context, userCode string) (*domain.DeviceAuthSession, error) {
	if err := r.client.DeviceAuthSession.
		Update().
		Where(deviceauthsession.UserCode(userCode)).
		AddInvalidAttempts(1).
		Exec(ctx); err != nil {
		return nil, err
	}
	return r.GetByUserCode(ctx, userCode)
}

func (r *entDeviceAuthRepository) Deny(ctx context.Context, userCode string) error {
	return r.client.DeviceAuthSession.
		Update().
		Where(
			deviceauthsession.UserCode(userCode),
			deviceauthsession.StatusEQ(deviceauthsession.StatusPending),
		).
		SetStatus(deviceauthsession.StatusDenied).
		Exec(ctx)
}

func (r *entDeviceAuthRepository) Expire(ctx context.Context, deviceCode string, now time.Time) error {
	return r.client.DeviceAuthSession.
		Update().
		Where(
			deviceauthsession.DeviceCode(deviceCode),
			deviceauthsession.StatusIn(deviceauthsession.StatusPending, deviceauthsession.StatusAuthorized),
			deviceauthsession.ExpiresAtLTE(now),
		).
		SetStatus(deviceauthsession.StatusExpired).
		Exec(ctx)
}

func (r *entDeviceAuthRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := r.client.DeviceAuthSession.
		Delete().
		Where(deviceauthsession.ExpiresAtLT(now.Add(-time.Hour))).
		Exec(ctx)
	return err
}
