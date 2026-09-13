package repository

import (
	"shadmin/ent"

	"entgo.io/ent/dialect/sql"
)

// emptyToNil 把空串转为 nil 指针，用于可空字段写入 NULL 而非空串。
func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ApplySorting returns an order function based on sort parameters and a field mapping.
// fieldMap maps user-facing sort field names (e.g. "username") to Ent field constants.
// If sortBy is not found in fieldMap, defaultField is used.
// Returns func(*sql.Selector) so it's compatible with all entity-specific OrderOption types.
func ApplySorting(sortBy, order string, fieldMap map[string]string, defaultField string) func(*sql.Selector) {
	field := defaultField
	if mapped, ok := fieldMap[sortBy]; ok {
		field = mapped
	}
	if order == "desc" {
		return ent.Desc(field)
	}
	return ent.Asc(field)
}

// mapSlice 将实体切片按 convert 映射为领域切片。
func mapSlice[E any, D any](ents []*E, convert func(*E) *D) []*D {
	result := make([]*D, 0, len(ents))
	for _, e := range ents {
		result = append(result, convert(e))
	}
	return result
}

// buildTree 把扁平节点列表构造为树
// id / parentID / setChildren 用于在泛型下访问各节点类型上的同名字段。
func buildTree[T any](
	nodes []T,
	id func(T) string,
	parentID func(T) *string,
	setChildren func(*T, []T),
) []T {
	childrenByParent := make(map[string][]T, len(nodes))
	roots := make([]T, 0)
	for _, n := range nodes {
		p := parentID(n)
		if p == nil || *p == "" {
			roots = append(roots, n)
			continue
		}
		childrenByParent[*p] = append(childrenByParent[*p], n)
	}

	// 自顶向下装配子树；每个节点只访问一次，Children 按 nodes 中的原始顺序组装。
	var attach func(T) T
	attach = func(n T) T {
		kids := childrenByParent[id(n)]
		children := make([]T, 0, len(kids))
		for _, kid := range kids {
			children = append(children, attach(kid))
		}
		setChildren(&n, children)
		return n
	}

	for i := range roots {
		roots[i] = attach(roots[i])
	}
	return roots
}
