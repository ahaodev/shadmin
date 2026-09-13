package repository

import (
	"testing"
	"time"

	"shadmin/ent"
)

func assertStrPtrEq(t *testing.T, field string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %q", field, want)
	}
	if *got != want {
		t.Fatalf("%s = %q, want %q", field, *got, want)
	}
}

// converter 被 GetMenus / GetMenuByID / GetChildrenMenus 共用，字段级断言防止
// 后续增删字段时静默漏映射。
func TestEntMenuToDomain_MapsEveryField(t *testing.T) {
	parentID := "parent-1"
	created := time.Now().Add(-time.Hour).Truncate(time.Second)
	updated := time.Now().Truncate(time.Second)

	m := &ent.Menu{
		ID:          "menu-1",
		Name:        "用户管理",
		Sequence:    7,
		Type:        "button",
		Path:        "/system/users",
		Icon:        "user",
		Component:   "system/user/index",
		RouteName:   "SystemUser",
		Query:       `{"a":"b"}`,
		IsFrame:     true,
		Visible:     "show",
		Permissions: "system:user:list",
		Status:      "active",
		ParentID:    &parentID,
		CreatedAt:   created,
		UpdatedAt:   updated,
		Edges: ent.MenuEdges{
			APIResources: []*ent.ApiResource{
				{ID: "GET:/api/v1/users"},
				{ID: "POST:/api/v1/users"},
			},
		},
	}

	got := entMenuToDomain(m)

	if got.ID != m.ID || got.Name != m.Name || got.Sequence != m.Sequence {
		t.Fatalf("identity fields not mapped: %+v", got)
	}
	if got.Type != m.Type || got.Icon != m.Icon || got.Visible != m.Visible || got.Status != m.Status {
		t.Fatalf("scalar fields not mapped: %+v", got)
	}
	if got.IsFrame != m.IsFrame {
		t.Fatalf("IsFrame = %v, want %v", got.IsFrame, m.IsFrame)
	}
	assertStrPtrEq(t, "Path", got.Path, m.Path)
	assertStrPtrEq(t, "Component", got.Component, m.Component)
	assertStrPtrEq(t, "RouteName", got.RouteName, m.RouteName)
	assertStrPtrEq(t, "Query", got.Query, m.Query)
	assertStrPtrEq(t, "Permissions", got.Permissions, m.Permissions)
	assertStrPtrEq(t, "ParentID", got.ParentID, parentID)

	if !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(updated) {
		t.Fatalf("timestamps not mapped: got %v/%v want %v/%v",
			got.CreatedAt, got.UpdatedAt, created, updated)
	}

	wantIDs := []string{"GET:/api/v1/users", "POST:/api/v1/users"}
	if len(got.ApiResources) != len(wantIDs) {
		t.Fatalf("ApiResources = %v, want %v", got.ApiResources, wantIDs)
	}
	for i, want := range wantIDs {
		if got.ApiResources[i] != want {
			t.Fatalf("ApiResources[%d] = %q, want %q", i, got.ApiResources[i], want)
		}
	}
}

// 空字符串的可空字段必须映射为 nil（写库时表示 NULL），而不是指向空串。
func TestEntMenuToDomain_EmptyNullableFieldsBecomeNil(t *testing.T) {
	got := entMenuToDomain(&ent.Menu{ID: "menu-1"})

	if got.Path != nil || got.Component != nil || got.RouteName != nil ||
		got.Query != nil || got.Permissions != nil {
		t.Fatalf("empty nullable fields should map to nil, got %+v", got)
	}
	if got.ParentID != nil {
		t.Fatalf("ParentID = %v, want nil", *got.ParentID)
	}
	// 未预加载 API 资源边时必须是 nil 切片，与旧实现一致。
	if got.ApiResources != nil {
		t.Fatalf("ApiResources = %v, want nil when edge not loaded", got.ApiResources)
	}
}
