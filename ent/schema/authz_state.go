package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// AuthzState stores the generation of the authorization data projection.
type AuthzState struct {
	ent.Schema
}

func (AuthzState) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			MaxLen(16).
			NotEmpty().
			Unique().
			Immutable(),
		field.Int64("generation").
			Default(0),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}
