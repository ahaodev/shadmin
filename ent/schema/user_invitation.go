package schema

import (
	"time"

	"github.com/rs/xid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserInvitation stores a one-time invitation token hash, never the raw token.
type UserInvitation struct {
	ent.Schema
}

func (UserInvitation) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").DefaultFunc(func() string { return xid.New().String() }),
		field.String("user_id").MaxLen(32),
		field.String("token_hash").MaxLen(64),
		field.Time("expires_at"),
		field.Time("accepted_at").Optional(),
		field.Time("revoked_at").Optional(),
		field.String("created_by").MaxLen(32),
		field.Time("created_at").Default(time.Now),
	}
}

func (UserInvitation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("invitations").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (UserInvitation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_hash").Unique(),
		index.Fields("user_id"),
		index.Fields("expires_at"),
	}
}
