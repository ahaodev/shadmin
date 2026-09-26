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

// Helper function to convert domain status string to ent status enum
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
		// 未知/非法状态：返回空串使其不匹配任何行，避免静默回退 active 掩盖错误
		return ""
	}
}

// Helper function to convert ent status enum to domain status string
func entStatusToDomainStatus(status user.Status) string {
	return string(status)
}

// derefString 安全解引用可空字符串指针，nil 返回空串。
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// entUserToDomainUser converts an ent.User to domain.User and extracts role IDs from edges.
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

	// Extract role IDs from database relationship
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

	// Extract department name from edge
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

func (ur *entUserRepository) Create(c context.Context, u *domain.User) error {
	return ur.create(c, u, nil)
}

func (ur *entUserRepository) CreateWithRoles(c context.Context, u *domain.User, roleIDs []string) error {
	return ur.create(c, u, &roleIDs)
}

func (ur *entUserRepository) create(c context.Context, u *domain.User, roleIDs *[]string) error {
	status := domainStatusToEntStatus(u.Status)
	var created *ent.User

	err := WithAuthorizationTx(c, ur.client, func(txCtx context.Context, tx *ent.Tx) error {
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
		return err
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

func (ur *entUserRepository) Query(c context.Context, filter domain.UserQueryFilter) (*domain.UserPagedResult, error) {
	// 构建查询条件
	var predicates []predicate.User
	if filter.Status != "" {
		// status 支持逗号多值（web 多选拼接，如 active,suspended）
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
		// role 过滤：逗号分隔的角色 ID，匹配拥有任一角色的用户
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
	baseQuery := ur.client.User.Query().Where(predicates...)

	// 默认排除 admin 用户（除非明确查询）
	//if filter.Username != "admin" {
	//	baseQuery = baseQuery.Where(user.Not(user.Username("admin")))
	//}

	// 获取总数
	total, err := baseQuery.Clone().Count(c)
	if err != nil {
		return nil, err
	}

	// 应用排序（在Select之前）
	if filter.SortBy != "" {
		baseQuery = baseQuery.Order(ApplySorting(filter.SortBy, filter.Order, map[string]string{
			"username": user.FieldUsername,
			"email":    user.FieldEmail,
		}, user.FieldCreatedAt))
	}

	// 构建数据查询 (移除租户信息预加载)
	query := baseQuery

	// 需要角色时在基础查询上一次性预加载，避免逐行回查（N+1）
	if filter.IncludeRoles {
		query = query.WithRoles()
	}

	offset, limit := filter.Paginate()
	query = query.Offset(offset).Limit(limit)

	users, err := query.WithDepartment().All(c)
	if err != nil {
		return nil, err
	}

	// 列表响应不携带密码哈希，与单条查询共用同一转换函数
	var result []*domain.User
	for _, u := range users {
		result = append(result, entUserToDomainUser(u, false))
	}

	return domain.NewPagedResult(result, total, filter.Page, filter.PageSize), nil
}

func (ur *entUserRepository) GetByIdentifier(c context.Context, identifier string) (*domain.User, error) {
	u, err := ur.client.User.
		Query().
		Where(user.And(
			user.Or(
				user.Username(identifier),
				user.Email(identifier),
				user.Phone(identifier),
			),
			// 密码登录只匹配本地用户：oidc 用户无密码且 email 允许与本地用户重复，
			// 限定 source=shadmin 避免同 email 双行解析歧义，也与“provider 不使用密码登录”一致。
			user.SourceEQ(user.Source(constants.UserSourceLocal)),
		)).
		WithRoles().
		WithDepartment().
		First(c)

	if err != nil {
		return nil, err
	}

	return entUserToDomainUser(u, true), nil
}

func (ur *entUserRepository) GetByID(c context.Context, id string) (*domain.User, error) {
	u, err := ur.client.User.
		Query().
		Where(user.ID(id)).
		WithRoles().
		WithDepartment().
		First(c)

	if err != nil {
		return nil, err
	}

	return entUserToDomainUser(u, true), nil
}

// Update updates user fields that do not affect authorization.
// Status and role changes must use UpdateWithAuthorization or UpdateWithRoles.
// 🔒 IsAdmin is intentionally excluded and can only be set during creation.
func (ur *entUserRepository) Update(c context.Context, u *domain.User) error {
	return ur.update(c, u, nil, false)
}

// UpdateWithAuthorization updates user fields and advances the authorization
// generation for changes such as an account status transition.
func (ur *entUserRepository) UpdateWithAuthorization(c context.Context, u *domain.User) error {
	return ur.update(c, u, nil, true)
}

func (ur *entUserRepository) UpdateWithRoles(c context.Context, u *domain.User, roleIDs []string) error {
	return ur.update(c, u, &roleIDs, true)
}

func (ur *entUserRepository) update(c context.Context, u *domain.User, roleIDs *[]string, authorizationChanged bool) error {
	var updated *ent.User
	mutate := func(txCtx context.Context, tx *ent.Tx) error {
		updateQuery := tx.User.
			UpdateOneID(u.ID).
			SetUsername(u.Username).
			SetNickname(u.Nickname).
			SetAvatar(u.Avatar).
			SetStatus(domainStatusToEntStatus(u.Status))

		// email 唯一且可空：空值写 NULL（第三方来源用户可能无邮箱），非空则更新
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
			updateQuery = updateQuery.ClearRoles().AddRoleIDs((*roleIDs)...)
		}

		var err error
		updated, err = updateQuery.Save(txCtx)
		return err
	}

	var err error
	if authorizationChanged {
		err = WithAuthorizationTx(c, ur.client, mutate)
	} else {
		err = withEntTransaction(c, ur.client, mutate)
	}
	if err != nil {
		return err
	}

	u.UpdatedAt = updated.UpdatedAt
	return nil
}

// UpdateIdentityProfile refreshes OIDC presentation fields without changing authorization state.
func (ur *entUserRepository) UpdateIdentityProfile(ctx context.Context, userID, nickname, avatar string) error {
	_, err := ur.client.User.UpdateOneID(userID).
		SetNickname(nickname).
		SetAvatar(avatar).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update identity profile: %w", err)
	}
	return nil
}

func (ur *entUserRepository) Delete(c context.Context, id string) error {
	return WithAuthorizationTx(c, ur.client, func(txCtx context.Context, tx *ent.Tx) error {
		return tx.User.DeleteOneID(id).Exec(txCtx)
	})
}

// GetStatusByID 只查询用户状态字段，避免加载整条记录。
// 用在登录/刷新/中间件等高频路径上。
func (ur *entUserRepository) GetStatusByID(c context.Context, id string) (string, error) {
	status, err := ur.client.User.
		Query().
		Where(user.ID(id)).
		Select(user.FieldStatus).
		String(c)
	if err != nil {
		return "", err
	}
	return status, nil
}

func (ur *entUserRepository) GetRoleIDs(c context.Context, id string) ([]string, error) {
	u, err := ur.client.User.
		Query().
		Where(user.ID(id)).
		WithRoles().
		First(c)
	if err != nil {
		return nil, err
	}

	roleIDs := make([]string, 0, len(u.Edges.Roles))
	for _, role := range u.Edges.Roles {
		roleIDs = append(roleIDs, role.ID)
	}
	return roleIDs, nil
}
