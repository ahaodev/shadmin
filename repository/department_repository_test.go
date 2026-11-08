package repository

import (
	"context"
	"errors"
	"testing"

	"shadmin/domain"
	"shadmin/ent"
)

func newSiblingFixture(t *testing.T) (domain.DepartmentRepository, context.Context, *domain.Department, *domain.Department) {
	t.Helper()
	ctx := context.Background()
	repo := NewDepartmentRepository(newAuthorizationStateTestClient(t))
	parent := &domain.Department{Name: "parent", Status: domain.StatusActive}
	if err := repo.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	sibling := &domain.Department{Name: "sibling", Status: domain.StatusActive, ParentID: &parent.ID}
	if err := repo.Create(ctx, sibling); err != nil {
		t.Fatal(err)
	}
	return repo, ctx, parent, sibling
}

func TestDepartmentRepositoryMapsNameConflict(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		repo, ctx, parent, _ := newSiblingFixture(t)
		err := repo.Create(ctx, &domain.Department{Name: "sibling", Status: domain.StatusActive, ParentID: &parent.ID})
		if !errors.Is(err, domain.ErrDepartmentNameExists) {
			t.Fatalf("create duplicate sibling = %v; want ErrDepartmentNameExists", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		repo, ctx, parent, sibling := newSiblingFixture(t)
		other := &domain.Department{Name: "other", Status: domain.StatusActive, ParentID: &parent.ID}
		if err := repo.Create(ctx, other); err != nil {
			t.Fatal(err)
		}
		other.Name = sibling.Name
		if err := repo.Update(ctx, other); !errors.Is(err, domain.ErrDepartmentNameExists) {
			t.Fatalf("rename onto sibling = %v; want ErrDepartmentNameExists", err)
		}
	})
}

func TestDepartmentRepositoryPreservesForeignKeyError(t *testing.T) {
	ctx := context.Background()
	repo := NewDepartmentRepository(newAuthorizationStateTestClient(t))
	missing := "missing-parent"

	if err := repo.Create(ctx, &domain.Department{Name: "orphan", Status: domain.StatusActive, ParentID: &missing}); !ent.IsConstraintError(err) || errors.Is(err, domain.ErrDepartmentNameExists) {
		t.Fatalf("create with missing parent = %v; want original constraint error", err)
	}

	root := &domain.Department{Name: "root", Status: domain.StatusActive}
	if err := repo.Create(ctx, root); err != nil {
		t.Fatal(err)
	}
	root.ParentID = &missing
	if err := repo.Update(ctx, root); !ent.IsConstraintError(err) || errors.Is(err, domain.ErrDepartmentNameExists) {
		t.Fatalf("update with missing parent = %v; want original constraint error", err)
	}
}
