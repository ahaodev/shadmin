package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/rs/xid"
)

type Role struct {
	ent.Schema
}

func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			MaxLen(20).
			NotEmpty().
			Unique().
			Immutable().
			DefaultFunc(func() string {
				return xid.New().String()
			}).
			Comment("角色ID"),
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Comment("角色名称"),
		field.Int("sequence").
			Default(0).
			Comment("排序序号"),
		field.Bool("is_system").
			Default(false).
			Immutable().
			Comment("是否为系统内置角色，不可删除/重命名"),
		field.String("status").
			MaxLen(20).
			Default("active").
			Comment("角色状态 active/inactive"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),
	}
}

func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("users", User.Type).Ref("roles").Comment("角色用户"),
		edge.To("menus", Menu.Type).Comment("角色菜单权限"),
	}
}

func (Role) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").
			Unique(),
		index.Fields("status"),
		index.Fields("sequence"),
	}
}
