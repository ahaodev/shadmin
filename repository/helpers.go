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

// buildTree 把扁平节点列表构造为树，ParentID 为 nil 或空串的节点作为根节点。
//
// 根集合与每层 Children 均初始化为空切片而非 nil：domain.Response.Data 是 interface{}，
// 其 omitempty 只判断接口自身是否为 nil，类型化的 nil 切片仍会序列化为 null。
// 空树序列化为 [] 而不是 null，故两者都必须显式初始化为空切片。
//
// id / parentID / setChildren 用于在泛型下访问各节点类型上的同名字段。
func buildTree[T any](
	nodes []T,
	id func(T) string,
	parentID func(T) *string,
	setChildren func(*T, []T),
) []T {
	var childrenOf func(parent string) []T
	childrenOf = func(parent string) []T {
		out := make([]T, 0)
		for _, n := range nodes {
			p := parentID(n)
			if p == nil || *p != parent {
				continue
			}
			child := n
			setChildren(&child, childrenOf(id(child)))
			out = append(out, child)
		}
		return out
	}

	roots := make([]T, 0)
	for _, n := range nodes {
		p := parentID(n)
		if p == nil || *p == "" {
			roots = append(roots, n)
		}
	}
	for i := range roots {
		setChildren(&roots[i], childrenOf(id(roots[i])))
	}
	return roots
}
