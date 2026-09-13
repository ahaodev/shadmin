package repository

import "testing"

type treeNode struct {
	ID       string
	ParentID *string
	Children []treeNode
}

func buildTestTree(nodes []treeNode) []treeNode {
	return buildTree(nodes,
		func(n treeNode) string { return n.ID },
		func(n treeNode) *string { return n.ParentID },
		func(n *treeNode, children []treeNode) { n.Children = children },
	)
}

// 空输入必须返回非 nil 的空切片：类型化的 nil 切片会被序列化为 null，
// 前端依赖空树序列化为 []。
func TestBuildTree_EmptyInputReturnsNonNilEmptySlice(t *testing.T) {
	roots := buildTestTree(nil)
	if roots == nil {
		t.Fatal("roots = nil, want non-nil empty slice")
	}
	if len(roots) != 0 {
		t.Fatalf("len(roots) = %d, want 0", len(roots))
	}
}

// 校验层级、顺序与叶子节点的 Children 非 nil。
func TestBuildTree_AssemblesNestedTreeInInputOrder(t *testing.T) {
	root := treeNode{ID: "a"}
	child1 := treeNode{ID: "b", ParentID: new("a")}
	grandchild := treeNode{ID: "d", ParentID: new("b")}
	child2 := treeNode{ID: "c", ParentID: new("a")}

	roots := buildTestTree([]treeNode{root, child1, grandchild, child2})

	if len(roots) != 1 {
		t.Fatalf("len(roots) = %d, want 1", len(roots))
	}
	if roots[0].ID != "a" {
		t.Fatalf("root id = %q, want a", roots[0].ID)
	}
	if roots[0].Children == nil {
		t.Fatal("root Children = nil, want non-nil empty-or-populated slice")
	}
	if len(roots[0].Children) != 2 {
		t.Fatalf("root children = %d, want 2", len(roots[0].Children))
	}
	// 子节点顺序应保持输入顺序（b 在 c 之前）。
	if roots[0].Children[0].ID != "b" || roots[0].Children[1].ID != "c" {
		t.Fatalf("child order = [%s %s], want [b c]",
			roots[0].Children[0].ID, roots[0].Children[1].ID)
	}
	if len(roots[0].Children[0].Children) != 1 || roots[0].Children[0].Children[0].ID != "d" {
		t.Fatalf("grandchild not attached under b: %+v", roots[0].Children[0].Children)
	}
	// 叶子节点也必须是非 nil 空切片。
	if roots[0].Children[1].Children == nil {
		t.Fatal("leaf Children = nil, want non-nil empty slice")
	}
}

// 父节点不存在的孤儿节点应被丢弃（与旧实现一致：既不是根，也无人认领）。
func TestBuildTree_DropsOrphans(t *testing.T) {
	root := treeNode{ID: "a"}
	orphan := treeNode{ID: "x", ParentID: new("missing")}

	roots := buildTestTree([]treeNode{root, orphan})

	if len(roots) != 1 || roots[0].ID != "a" {
		t.Fatalf("roots = %+v, want only [a]", roots)
	}
}

// nil 与空串 ParentID 都视为根节点。
func TestBuildTree_NilAndEmptyParentAreRoots(t *testing.T) {
	roots := buildTestTree([]treeNode{
		{ID: "a"},
		{ID: "b", ParentID: new("")},
		{ID: "c", ParentID: new("a")},
	})

	if len(roots) != 2 {
		t.Fatalf("len(roots) = %d, want 2 (a, b)", len(roots))
	}
}
