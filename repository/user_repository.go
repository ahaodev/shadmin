package repository

import (
	"context"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/predicate"
	"shadmin/ent/user"
)

// Helper function to convert domain status string to ent status enum
func domainStatusToEntStatus(status string) user.Status {
	switch status {
	case domain.StatusActive:
		return user.StatusActive
	case domain.StatusInactive:
		return user.StatusInactive
	case domain.UserStatusInvited:
		return user.StatusInvited
	case domain.UserStatusSuspended:
		return user.StatusSuspended
	default:
		return user.StatusActive
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

// emptyToNil 把空串转为 nil 指针，用于可空唯一字段写入 NULL 而非空串。
func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// entUserToDomainUser converts an ent.User to domain.User and extracts role IDs from edges
func entUserToDomainUser(u *ent.User) *domain.User {
	domainUser := &domain.User{
		ID:           u.ID,
		Username:     u.Username,
		Email:        derefString(u.Email),
		Phone:        derefString(u.Phone),
		Password:     derefString(u.Password),
		Source:       string(u.Source),
		Avatar:       u.Avatar,
		IsAdmin:      u.IsAdmin,
		Status:       entStatusToDomainStatus(u.Status),
		DepartmentID: u.DepartmentID,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
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
	status := domainStatusToEntStatus(u.Status)

	createQuery := ur.client.User.
		Create().
		SetUsername(u.Username).
		SetNillableEmail(emptyToNil(u.Email)).
		SetNillablePhone(emptyToNil(u.Phone)).
		SetNillablePassword(emptyToNil(u.Password)).
		SetAvatar(u.Avatar).
		SetStatus(status).
		SetNillableInvitedAt(u.InvitedAt).
		SetNillableInvitedBy(&u.InvitedBy).
		SetNillableDepartmentID(u.DepartmentID)

	// 来源：默认 local，第三方登录用户由 usecase 显式置为 oauth
	if u.Source != "" {
		createQuery = createQuery.SetSource(user.Source(u.Source))
	}

	created, err := createQuery.Save(c)

	if err != nil {
		return err
	}

	u.ID = created.ID
	u.Status = entStatusToDomainStatus(created.Status)
	u.CreatedAt = created.CreatedAt
	u.UpdatedAt = created.UpdatedAt
	return nil
}

func (ur *entUserRepository) Query(c context.Context, filter domain.UserQueryFilter) (*domain.UserPagedResult, error) {
	// 构建查询条件
	var predicates []predicate.User
	if filter.Status != "" {
		predicates = append(predicates, user.StatusEQ(domainStatusToEntStatus(filter.Status)))
	}
	if filter.Username != "" {
		predicates = append(predicates, user.UsernameContains(filter.Username))
	}
	if filter.Email != "" {
		predicates = append(predicates, user.EmailContains(filter.Email))
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

	// 应用分页
	if offset, limit, ok := filter.Paginate(); ok {
		query = query.Offset(offset).Limit(limit)
	}
	users, err := query.WithDepartment().All(c)
	if err != nil {
		return nil, err
	}

	// 转换为 domain.User (移除租户相关代码)
	var result []*domain.User
	for _, u := range users {
		domainUser := &domain.User{
			ID:           u.ID,
			Username:     u.Username,
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
		if u.Edges.Department != nil {
			domainUser.DepartmentName = u.Edges.Department.Name
		}

		// 如果需要包含角色信息 (从数据库关系获取)
		if filter.IncludeRoles {
			// 重新查询包含角色信息的用户
			userWithRoles, err := ur.client.User.
				Query().
				Where(user.ID(u.ID)).
				WithRoles().
				First(c)
			if err == nil && userWithRoles.Edges.Roles != nil {
				var roleIDs []string
				for _, role := range userWithRoles.Edges.Roles {
					roleIDs = append(roleIDs, role.ID)
				}
				domainUser.Roles = roleIDs
				if len(roleIDs) > 0 {
					domainUser.IsActive = true // 有角色说明是活跃的
				}
			}
		}

		result = append(result, domainUser)
	}

	return domain.NewPagedResult(result, total, filter.Page, filter.PageSize), nil
}

func (ur *entUserRepository) GetByUsername(c context.Context, userName string) (*domain.User, error) {
	u, err := ur.client.User.
		Query().
		Where(user.Username(userName)).
		WithRoles().
		WithDepartment().
		First(c)

	if err != nil {
		return nil, err
	}

	return entUserToDomainUser(u), nil
}

func (ur *entUserRepository) GetByEmail(c context.Context, email string) (*domain.User, error) {
	u, err := ur.client.User.
		Query().
		Where(user.Email(email)).
		WithRoles().
		WithDepartment().
		First(c)

	if err != nil {
		return nil, err
	}

	return entUserToDomainUser(u), nil
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

	return entUserToDomainUser(u), nil
}

// Update 更新用户信息
// 🔒 安全: IsAdmin 字段在此处被故意排除，仅在创建时设置，不可通过 API 修改
func (ur *entUserRepository) Update(c context.Context, u *domain.User) error {
	updateQuery := ur.client.User.
		UpdateOneID(u.ID).
		SetUsername(u.Username).
		SetAvatar(u.Avatar).
		SetStatus(domainStatusToEntStatus(u.Status))

	// email 唯一且可空：空值写 NULL（第三方来源用户可能无邮箱），非空则更新
	if u.Email == "" {
		updateQuery = updateQuery.ClearEmail()
	} else {
		updateQuery = updateQuery.SetEmail(u.Email)
	}

	// phone 唯一且可空：空值写入 NULL（避免空串触发唯一冲突），非空则更新
	if u.Phone == "" {
		updateQuery = updateQuery.ClearPhone()
	} else {
		updateQuery = updateQuery.SetPhone(u.Phone)
	}

	// Handle department_id
	if u.DepartmentID != nil && *u.DepartmentID != "" {
		updateQuery = updateQuery.SetDepartmentID(*u.DepartmentID)
	} else {
		updateQuery = updateQuery.ClearDepartmentID()
	}

	// 🔒 如果提供了密码，则更新密码哈希
	if u.Password != "" {
		updateQuery = updateQuery.SetPassword(u.Password)
	}

	updated, err := updateQuery.Save(c)

	if err != nil {
		return err
	}

	u.UpdatedAt = updated.UpdatedAt
	return nil
}

func (ur *entUserRepository) Delete(c context.Context, id string) error {
	return ur.client.User.
		DeleteOneID(id).
		Exec(c)
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
