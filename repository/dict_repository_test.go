package repository

import (
	"context"
	"errors"
	"testing"

	"shadmin/domain"
)

// createDictTypeForTest 建一个字典类型，作为字典项的归属。
func createDictTypeForTest(t *testing.T, repo domain.DictRepository, code string) *domain.DictType {
	t.Helper()
	dictType := &domain.DictType{Code: code, Name: code, Status: domain.StatusActive}
	if err := repo.CreateType(context.Background(), dictType); err != nil {
		t.Fatalf("create dict type %s: %v", code, err)
	}
	return dictType
}

// 同一类型下 label 唯一：重复 label 必须映射为领域哨兵（→409），而不是落成数据库约束错误（→500）。
func TestDictRepositoryRejectsDuplicateLabel(t *testing.T) {
	client := newAuthorizationStateTestClient(t)
	repo := NewDictRepository(client)
	ctx := context.Background()

	dictType := createDictTypeForTest(t, repo, "payment_status")
	first := &domain.DictItem{
		TypeID: dictType.ID, Label: "已支付", Value: "paid", Status: domain.StatusActive,
	}
	if err := repo.CreateItem(ctx, first); err != nil {
		t.Fatalf("create first item: %v", err)
	}

	duplicate := &domain.DictItem{
		TypeID: dictType.ID, Label: "已支付", Value: "settled", Status: domain.StatusActive,
	}
	if err := repo.CreateItem(ctx, duplicate); !errors.Is(err, domain.ErrDictItemLabelExists) {
		t.Fatalf("create with duplicate label = %v; want ErrDictItemLabelExists", err)
	}

	second := &domain.DictItem{
		TypeID: dictType.ID, Label: "未支付", Value: "unpaid", Status: domain.StatusActive,
	}
	if err := repo.CreateItem(ctx, second); err != nil {
		t.Fatalf("create second item: %v", err)
	}

	renameToDuplicate := "已支付"
	err := repo.UpdateItem(ctx, second.ID, domain.UpdateDictItemRequest{Label: &renameToDuplicate})
	if !errors.Is(err, domain.ErrDictItemLabelExists) {
		t.Fatalf("update to duplicate label = %v; want ErrDictItemLabelExists", err)
	}

	// 改成自身当前 label 不应被自己挡住。
	sameLabel := "未支付"
	if err := repo.UpdateItem(ctx, second.ID, domain.UpdateDictItemRequest{Label: &sameLabel}); err != nil {
		t.Fatalf("update to same label: %v", err)
	}
}
