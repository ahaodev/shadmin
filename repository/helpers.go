package repository

import (
	"shadmin/ent"
	"strings"

	"entgo.io/ent/dialect/sql"
)

// isUniqueConstraintOn matches SQLite columns or the PostgreSQL/MySQL index name.
// Inspect the key, not the duplicate value, which may contain another index name.
func isUniqueConstraintOn(err error, columns, index string) bool {
	if !isUniqueConstraintError(err) {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint failed: "+columns) ||
		strings.Contains(message, `unique constraint "`+index+`"`) {
		return true
	}
	if pos := strings.LastIndex(message, "for key '"); pos >= 0 {
		key, _, _ := strings.Cut(message[pos+len("for key '"):], "'")
		return key == index || strings.HasSuffix(key, "."+index)
	}
	return false
}

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ApplySorting 将外部字段名映射为 Ent 字段；未知字段回退到 defaultField。
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

func mapSlice[E any, D any](ents []*E, convert func(*E) *D) []*D {
	result := make([]*D, 0, len(ents))
	for _, e := range ents {
		result = append(result, convert(e))
	}
	return result
}

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

	// 保留输入中的子节点顺序。
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
