package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/rs/xid"
)

type Department struct {
	ent.Schema
}

func (Department) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string {
				return xid.New().String()
			}).
			Unique().
			Immutable(),
		field.String("parent_id").
			Optional().
			Nillable(),
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.Int("sequence").
			Default(0),
		field.String("leader").
			MaxLen(64).
			Default(""),
		field.String("phone").
			MaxLen(20).
			Default(""),
		field.String("email").
			MaxLen(100).
			Default(""),
		field.Enum("status").
			Values("active", "inactive").
			Default("active"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (Department) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("parent", Department.Type).
			Ref("children").
			Field("parent_id").
			Unique(),
		edge.To("children", Department.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("users", User.Type),
	}
}

func (Department) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "parent_id").
			Unique(),
		index.Fields("status"),
	}
}
