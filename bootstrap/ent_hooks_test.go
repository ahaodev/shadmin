package bootstrap

import (
	"context"
	"errors"
	"testing"

	"shadmin/ent"
)

func TestMutationIDsOnCreate(t *testing.T) {
	client := newTestEntClient(t)
	ctx := context.Background()

	create := client.User.Create().SetUsername("u1").Mutation()
	if got := create.Op(); !got.Is(ent.OpCreate) {
		t.Fatalf("Op() = %v, want OpCreate", got)
	}
	ids, err := mutationIDs(ctx, create)
	if err != nil {
		t.Fatalf("mutationIDs on create: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("ids = %v, want empty", ids)
	}

	withID := client.Role.Create().SetID("r1")
	ids, err = mutationIDs(ctx, withID.Mutation())
	if err != nil {
		t.Fatalf("mutationIDs on create with SetID: %v", err)
	}
	if len(ids) != 1 || ids[0] != "r1" {
		t.Fatalf("ids = %v, want [r1]", ids)
	}
}

func TestIdsFromMutation(t *testing.T) {
	ctx := context.Background()
	noID := func() (string, bool) { return "", false }
	failingIDs := func(context.Context) ([]string, error) {
		return nil, errors.New("IDs is not allowed on create operations")
	}

	t.Run("create does not consult IDs", func(t *testing.T) {
		called := false
		idsFn := func(context.Context) ([]string, error) {
			called = true
			return nil, errors.New("IDs is not allowed on create operations")
		}

		ids, err := idsFromMutation(ctx, ent.OpCreate, noID, idsFn)
		if err != nil {
			t.Fatalf("idsFromMutation(create): %v", err)
		}
		if called {
			t.Fatal("IDs() was called on create; ent returns an error there")
		}
		if len(ids) != 0 {
			t.Fatalf("ids = %v, want empty", ids)
		}
	})

	t.Run("create with explicit ID keeps it", func(t *testing.T) {
		ids, err := idsFromMutation(ctx, ent.OpCreate, func() (string, bool) { return "r1", true }, failingIDs)
		if err != nil {
			t.Fatalf("idsFromMutation(create with SetID): %v", err)
		}
		if len(ids) != 1 || ids[0] != "r1" {
			t.Fatalf("ids = %v, want [r1]", ids)
		}
	})

	t.Run("batched update resolves IDs", func(t *testing.T) {
		ids, err := idsFromMutation(ctx, ent.OpUpdate, noID, func(context.Context) ([]string, error) {
			return []string{"u1", "u2"}, nil
		})
		if err != nil {
			t.Fatalf("idsFromMutation(bulk update): %v", err)
		}
		if len(ids) != 2 || ids[0] != "u1" || ids[1] != "u2" {
			t.Fatalf("ids = %v, want [u1 u2]", ids)
		}
	})

	t.Run("single update does not consult IDs", func(t *testing.T) {
		ids, err := idsFromMutation(ctx, ent.OpUpdateOne, func() (string, bool) { return "u1", true }, failingIDs)
		if err != nil {
			t.Fatalf("idsFromMutation(update one): %v", err)
		}
		if len(ids) != 1 || ids[0] != "u1" {
			t.Fatalf("ids = %v, want [u1]", ids)
		}
	})
}
