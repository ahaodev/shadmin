package repository

import (
	"context"
	"fmt"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/predicate"
	"shadmin/ent/role"
	"shadmin/ent/user"
	"shadmin/internal/constants"
	"strings"
)

func domainStatusToEntStatus(status string) user.Status {
	switch status {
	case constants.StatusActive:
		return user.StatusActive
	case constants.StatusInactive:
		return user.StatusInactive
	case constants.UserStatusInvited:
		return user.StatusInvited
	case constants.UserStatusSuspended:
		return user.StatusSuspended
	default:
		// 非法状态不匹配任何行，避免回退为 active。
		return ""
	}
}

func entStatusToDomainStatus(status user.Status) string {
	return string(status)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func entUserToDomainUser(u *ent.User, withPassword bool) *domain.User {
	domainUser := &domain.User{
		ID:           u.ID,
		Username:     u.Username,
		Nickname:     u.Nickname,
		Email:        derefString(u.Email),
		Phone:        derefString(u.Phone),
		Source:       string(u.Source),
		Avatar:       u.Avatar,
		IsAdmin:      u.IsAdmin,
		Status:       entStatusToDomainStatus(u.Status),
		DepartmentID: u.DepartmentID,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
	if withPassword {
		domainUser.Password = derefString(u.Password)
	}

	if u.Edges.Roles != nil {
		var roleIDs []string
		for _, role := range u.Edges.Roles {
			roleIDs = append(roleIDs, role.ID)
		}
		domainUser.Roles = roleIDs
		if len(roleIDs) > 0 {
			domainUser.IsActive = true
		}
	}

	if u.Edges.Department != nil {
		domainUser.DepartmentName = u.Edges.Department.Name
	}

	return domainUser
}

type entUserRepository struct {
	client *ent.Client
}

func NewUserRepository(client *ent.Client) domain.UserRepository {
	return &entUserRepository{
		client: client,
	}
}

func (ur *entUserRepository) Create(ctx context.Context, u *domain.User) error {
	return ur.create(ctx, u, nil)
}

func (ur *entUserRepository) CreateWithRoles(ctx context.Context, u *domain.User, roleIDs []string) error {
	return ur.create(ctx, u, &roleIDs)
}

func (ur *entUserRepository) create(ctx context.Context, u *domain.User, roleIDs *[]string) error {
	status := domainStatusToEntStatus(u.Status)
	var created *ent.User

	err := WithAuthorizationTx(ctx, ur.client, func(txCtx context.Context, tx *ent.Tx) error {
		createQuery := tx.User.
			Create().
			SetUsername(u.Username).
			SetNickname(u.Nickname).
			SetNillableEmail(emptyToNil(u.Email)).
			SetNillablePhone(emptyToNil(u.Phone)).
			SetNillablePassword(emptyToNil(u.Password)).
			SetAvatar(u.Avatar).
			SetStatus(status).
			SetNillableInvitedAt(u.InvitedAt).
			SetNillableInvitedBy(&u.InvitedBy).
			SetNillableDepartmentID(u.DepartmentID)
		if u.Source != "" {
			createQuery = createQuery.SetSource(user.Source(u.Source))
		}
		if roleIDs != nil && len(*roleIDs) > 0 {
			createQuery = createQuery.AddRoleIDs(*roleIDs...)
		}
		var err error
		created, err = createQuery.Save(txCtx)
		return err
	})
	if err != nil {
		if isUniqueConstraintError(err) {
			return domain.ErrUserExists
		}
		return fmt.Errorf("create user: %w", err)
	}

	u.ID = created.ID
	u.Status = entStatusToDomainStatus(created.Status)
	u.CreatedAt = created.CreatedAt
	u.UpdatedAt = created.UpdatedAt
	if roleIDs != nil {
		u.Roles = append([]string(nil), *roleIDs...)
		u.IsActive = len(*roleIDs) > 0
	}
	return nil
}

func (ur *entUserRepository) Query(ctx context.Context, filter domain.UserQueryFilter) (*domain.UserPagedResult, error) {
	var predicates []predicate.User
	if filter.Status != "" {
		// 支持逗号分隔的多个状态。
		var entStatuses []user.Status
		for s := range strings.SplitSeq(filter.Status, ",") {
			if s = strings.TrimSpace(s); s != "" {
				entStatuses = append(entStatuses, domainStatusToEntStatus(s))
			}
		}
		predicates = append(predicates, user.StatusIn(entStatuses...))
	}
	if filter.Username != "" {
		predicates = append(predicates, user.UsernameContains(filter.Username))
	}
	if filter.Email != "" {
		predicates = append(predicates, user.EmailContains(filter.Email))
	}
	if filter.Keyword != "" {
		kw := strings.TrimSpace(filter.Keyword)
		if kw != "" {
			predicates = append(predicates, user.Or(
				user.UsernameContains(kw),
				user.NicknameContains(kw),
				user.EmailContains(kw),
			))
		}
	}
	if filter.Role != "" {
		// 匹配拥有任一指定角色的用户。
		var roleIDs []string
		for id := range strings.SplitSeq(filter.Role, ",") {
			if id = strings.TrimSpace(id); id != "" {
				roleIDs = append(roleIDs, id)
			}
		}
		if len(roleIDs) > 0 {
			predicates = append(predicates, user.HasRolesWith(role.IDIn(roleIDs...)))
		}
	}
	if filter.IsAdmin != nil {
		predicates = append(predicates, user.IsAdmin(*filter.IsAdmin))
	}
	if filter.DepartmentID != "" {
		predicates = append(predicates, user.DepartmentID(filter.DepartmentID))
	}
	baseQuery := clientFromContext(ctx, ur.client).User.Query().Where(predicates...)

	total, err := baseQuery.Clone().Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}

	if filter.SortBy != "" {
		baseQuery = baseQuery.Order(ApplySorting(filter.SortBy, filter.Order, map[string]string{
			"username": user.FieldUsername,
			"email":    user.FieldEmail,
		}, user.FieldCreatedAt))
	}

	query := baseQuery

	// 预加载角色，避免 N+1 查询。
	if filter.IncludeRoles {
		query = query.WithRoles()
	}

	offset, limit := filter.Paginate()
	query = query.Offset(offset).Limit(limit)

	users, err := query.WithDepartment().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}

	var result []*domain.User
	for _, u := range users {
		result = append(result, entUserToDomainUser(u, false))
	}

	return domain.NewPagedResult(result, total, filter.Page, filter.PageSize), nil
}

func (ur *entUserRepository) GetByIdentifier(ctx context.Context, identifier string) (*domain.User, error) {
	u, err := clientFromContext(ctx, ur.client).User.
		Query().
		Where(user.And(
			user.Or(
				user.Username(identifier),
				user.Email(identifier),
				user.Phone(identifier),
			),
			// 跨来源允许同邮箱；密码登录仅查询本地用户。
			user.SourceEQ(user.Source(constants.UserSourceLocal)),
		)).
		WithRoles().
		WithDepartment().
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by identifier: %w", err)
	}

	return entUserToDomainUser(u, true), nil
}

func (ur *entUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	u, err := clientFromContext(ctx, ur.client).User.
		Query().
		Where(user.ID(id)).
		WithRoles().
		WithDepartment().
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by ID: %w", err)
	}

	return entUserToDomainUser(u, true), nil
}

// Update 仅更新不影响授权的字段。
// Status and role changes must use UpdateWithAuthorization or UpdateWithRoles.
// IsAdmin 仅允许在创建时设置。
func (ur *entUserRepository) Update(ctx context.Context, u *domain.User) error {
	return ur.update(ctx, u, nil, false)
}

// UpdateWithAuthorization 在同一事务内更新用户并推进授权代数。
func (ur *entUserRepository) UpdateWithAuthorization(ctx context.Context, u *domain.User) error {
	return ur.update(ctx, u, nil, true)
}

func (ur *entUserRepository) UpdateWithRoles(ctx context.Context, u *domain.User, roleIDs []string) error {
	return ur.update(ctx, u, &roleIDs, true)
}

func (ur *entUserRepository) update(ctx context.Context, u *domain.User, roleIDs *[]string, authorizationChanged bool) error {
	var updated *ent.User
	mutate := func(txCtx context.Context, tx *ent.Tx) error {
		updateQuery := tx.User.
			UpdateOneID(u.ID).
			SetUsername(u.Username).
			SetNickname(u.Nickname).
			SetAvatar(u.Avatar)

		if authorizationChanged {
			updateQuery = updateQuery.SetStatus(domainStatusToEntStatus(u.Status))
		} else {
			currentStatus, err := tx.User.Query().
				Where(user.ID(u.ID)).
				Select(user.FieldStatus).
				String(txCtx)
			if err != nil {
				return fmt.Errorf("load current user status: %w", err)
			}
			if currentStatus != string(domainStatusToEntStatus(u.Status)) {
				return fmt.Errorf("user status changes require UpdateWithAuthorization")
			}
		}

		// 空邮箱写 NULL。
		if u.Email == "" {
			updateQuery = updateQuery.ClearEmail()
		} else {
			updateQuery = updateQuery.SetEmail(u.Email)
		}
		if u.Phone == "" {
			updateQuery = updateQuery.ClearPhone()
		} else {
			updateQuery = updateQuery.SetPhone(u.Phone)
		}
		if u.DepartmentID != nil && *u.DepartmentID != "" {
			updateQuery = updateQuery.SetDepartmentID(*u.DepartmentID)
		} else {
			updateQuery = updateQuery.ClearDepartmentID()
		}
		if u.Password != "" {
			updateQuery = updateQuery.SetPassword(u.Password)
		}
		if roleIDs != nil {
			updateQuery = updateQuery.ClearRoles().AddRoleIDs(*roleIDs...)
		}

		var err error
		updated, err = updateQuery.Save(txCtx)
		return err
	}

	var err error
	if authorizationChanged {
		err = WithAuthorizationTx(ctx, ur.client, mutate)
	} else {
		err = withEntTransaction(ctx, ur.client, mutate)
	}
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrUserNotFound
		}
		if isUniqueConstraintError(err) {
			return domain.ErrUserExists
		}
		return fmt.Errorf("update user: %w", err)
	}

	u.UpdatedAt = updated.UpdatedAt
	return nil
}

// UpdateIdentityProfile 不推进授权代数。
func (ur *entUserRepository) UpdateIdentityProfile(ctx context.Context, userID, nickname, avatar string) error {
	_, err := clientFromContext(ctx, ur.client).User.UpdateOneID(userID).
		SetNickname(nickname).
		SetAvatar(avatar).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update identity profile: %w", err)
	}
	return nil
}

func (ur *entUserRepository) Delete(ctx context.Context, id string) error {
	if err := WithAuthorizationTx(ctx, ur.client, func(txCtx context.Context, tx *ent.Tx) error {
		return tx.User.DeleteOneID(id).Exec(txCtx)
	}); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (ur *entUserRepository) GetStatusByID(ctx context.Context, id string) (string, error) {
	status, err := clientFromContext(ctx, ur.client).User.
		Query().
		Where(user.ID(id)).
		Select(user.FieldStatus).
		String(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", domain.ErrUserNotFound
		}
		return "", fmt.Errorf("get user status by id: %w", err)
	}
	return status, nil
}

func (ur *entUserRepository) GetRoleIDs(ctx context.Context, id string) ([]string, error) {
	u, err := clientFromContext(ctx, ur.client).User.
		Query().
		Where(user.ID(id)).
		WithRoles().
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get role IDs for user: %w", err)
	}

	roleIDs := make([]string, 0, len(u.Edges.Roles))
	for _, role := range u.Edges.Roles {
		roleIDs = append(roleIDs, role.ID)
	}
	return roleIDs, nil
}
