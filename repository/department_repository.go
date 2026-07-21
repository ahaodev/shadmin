package repository

import (
	"context"
	"shadmin/domain"
	"shadmin/ent"
	"shadmin/ent/department"
	"shadmin/internal/constants"
)

type entDepartmentRepository struct {
	client *ent.Client
}

func NewDepartmentRepository(client *ent.Client) domain.DepartmentRepository {
	return &entDepartmentRepository{client: client}
}

func (r *entDepartmentRepository) Create(ctx context.Context, dept *domain.Department) error {
	create := r.client.Department.Create().
		SetName(dept.Name).
		SetSequence(dept.Sequence).
		SetLeader(dept.Leader).
		SetPhone(dept.Phone).
		SetEmail(dept.Email).
		SetStatus(department.Status(dept.Status)).
		SetNillableParentID(dept.ParentID)

	d, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return domain.ErrDepartmentNameExists
		}
		return err
	}
	dept.ID = d.ID
	dept.CreatedAt = d.CreatedAt
	dept.UpdatedAt = d.UpdatedAt
	return nil
}

func (r *entDepartmentRepository) GetByID(ctx context.Context, id string) (*domain.Department, error) {
	d, err := r.client.Department.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, domain.ErrDepartmentNotFound
		}
		return nil, err
	}
	return entDepartmentToDomain(d), nil
}

func (r *entDepartmentRepository) FetchTree(ctx context.Context) ([]domain.Department, error) {
	depts, err := r.client.Department.Query().
		Order(department.BySequence()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	var all []domain.Department
	for _, d := range depts {
		all = append(all, *entDepartmentToDomain(d))
	}
	return buildDepartmentTree(all), nil
}

func (r *entDepartmentRepository) FetchList(ctx context.Context, filter domain.DepartmentQueryFilter) ([]domain.Department, error) {
	query := r.client.Department.Query()

	if filter.Name != "" {
		query = query.Where(department.NameContains(filter.Name))
	}
	if filter.Status != "" {
		query = query.Where(department.StatusEQ(department.Status(filter.Status)))
	}
	if filter.Search != "" {
		query = query.Where(
			department.Or(
				department.NameContains(filter.Search),
				department.LeaderContains(filter.Search),
			),
		)
	}

	depts, err := query.Order(department.BySequence()).All(ctx)
	if err != nil {
		return nil, err
	}

	var result []domain.Department
	for _, d := range depts {
		result = append(result, *entDepartmentToDomain(d))
	}
	return result, nil
}

func (r *entDepartmentRepository) Update(ctx context.Context, dept *domain.Department) error {
	update := r.client.Department.UpdateOneID(dept.ID)

	update = update.SetName(dept.Name).
		SetSequence(dept.Sequence).
		SetLeader(dept.Leader).
		SetPhone(dept.Phone).
		SetEmail(dept.Email).
		SetStatus(department.Status(dept.Status))

	if dept.ParentID != nil {
		update = update.SetParentID(*dept.ParentID)
	} else {
		update = update.ClearParentID()
	}

	d, err := update.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return domain.ErrDepartmentNameExists
		}
		return err
	}
	dept.UpdatedAt = d.UpdatedAt
	return nil
}

func (r *entDepartmentRepository) Delete(ctx context.Context, id string) error {
	return r.client.Department.DeleteOneID(id).Exec(ctx)
}

func (r *entDepartmentRepository) HasChildren(ctx context.Context, id string) (bool, error) {
	return r.client.Department.Query().
		Where(department.ParentID(id)).
		Exist(ctx)
}

func (r *entDepartmentRepository) HasUsers(ctx context.Context, id string) (bool, error) {
	d, err := r.client.Department.Get(ctx, id)
	if err != nil {
		return false, err
	}
	return d.QueryUsers().Exist(ctx)
}

func (r *entDepartmentRepository) GetAllChildrenIDs(ctx context.Context, id string) ([]string, error) {
	// Fetch all departments and walk tree in memory
	all, err := r.client.Department.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	childMap := make(map[string][]string)
	for _, d := range all {
		if d.ParentID != nil {
			childMap[*d.ParentID] = append(childMap[*d.ParentID], d.ID)
		}
	}

	var ids []string
	var collect func(parentID string)
	collect = func(parentID string) {
		for _, cid := range childMap[parentID] {
			ids = append(ids, cid)
			collect(cid)
		}
	}
	collect(id)
	return ids, nil
}

func (r *entDepartmentRepository) HasActiveChildren(ctx context.Context, id string) (bool, error) {
	all, err := r.client.Department.Query().All(ctx)
	if err != nil {
		return false, err
	}

	// Build parent→children map and status map in one pass
	childMap := make(map[string][]string)
	statusMap := make(map[string]string)
	for _, d := range all {
		statusMap[d.ID] = string(d.Status)
		if d.ParentID != nil {
			childMap[*d.ParentID] = append(childMap[*d.ParentID], d.ID)
		}
	}

	// DFS to find any active descendant
	var hasActive bool
	var walk func(parentID string)
	walk = func(parentID string) {
		for _, cid := range childMap[parentID] {
			if statusMap[cid] == constants.StatusActive {
				hasActive = true
				return
			}
			walk(cid)
		}
	}
	walk(id)
	return hasActive, nil
}

// entDepartmentToDomain converts ent Department to domain Department
func entDepartmentToDomain(d *ent.Department) *domain.Department {
	return &domain.Department{
		ID:        d.ID,
		ParentID:  d.ParentID,
		Name:      d.Name,
		Sequence:  d.Sequence,
		Leader:    d.Leader,
		Phone:     d.Phone,
		Email:     d.Email,
		Status:    string(d.Status),
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// buildDepartmentTree constructs hierarchical tree from flat list.
func buildDepartmentTree(nodes []domain.Department) []domain.Department {
	roots := make([]domain.Department, 0)
	for _, n := range nodes {
		if n.ParentID == nil || *n.ParentID == "" {
			roots = append(roots, n)
		}
	}
	for i := range roots {
		roots[i].Children = findDepartmentChildren(roots[i].ID, nodes)
	}
	return roots
}

func findDepartmentChildren(parentID string, all []domain.Department) []domain.Department {
	children := make([]domain.Department, 0)
	for _, n := range all {
		if n.ParentID != nil && *n.ParentID == parentID {
			child := n
			child.Children = findDepartmentChildren(child.ID, all)
			children = append(children, child)
		}
	}
	return children
}
