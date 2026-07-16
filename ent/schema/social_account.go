package schema

import (
	"time"

	"github.com/rs/xid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SocialAccount holds the schema definition for the SocialAccount entity.
// 用于把第三方身份（如 Google / GitHub 用户ID）绑定到 shadmin 用户。
type SocialAccount struct {
	ent.Schema
}

// Fields of the SocialAccount.
func (SocialAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string {
				return xid.New().String()
			}),
		field.String("user_id").
			MaxLen(32).
			Comment("绑定的 shadmin 用户ID"),
		field.String("provider").
			MaxLen(32).
			Comment("第三方 provider 标识，如 google、github"),
		field.String("provider_subject").
			MaxLen(255).
			Comment("第三方用户唯一标识（provider 内的用户ID）"),
		field.String("email").
			MaxLen(255).
			Optional().
			Comment("第三方返回的邮箱（可能为空）"),
		field.String("name").
			MaxLen(255).
			Optional().
			Comment("第三方返回的用户名"),
		field.String("avatar_url").
			MaxLen(512).
			Optional().
			Comment("第三方返回的头像地址"),
		field.Time("created_at").
			Default(time.Now),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the SocialAccount.
func (SocialAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("social_accounts").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the SocialAccount.
func (SocialAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "provider_subject").Unique(),
		index.Fields("user_id"),
	}
}
