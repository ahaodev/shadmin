package schema

import (
	"time"

	"github.com/rs/xid"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DeviceAuthSession holds OAuth 2.0 device authorization session state.
type DeviceAuthSession struct {
	ent.Schema
}

// Fields of the DeviceAuthSession.
func (DeviceAuthSession) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string {
				return xid.New().String()
			}),
		field.String("device_code").
			MaxLen(128).
			Unique().
			Comment("CLI持有的设备验证码"),
		field.String("user_code").
			MaxLen(16).
			Unique().
			Comment("用户在浏览器输入的授权码"),
		field.String("client_id").
			MaxLen(64).
			Comment("客户端ID"),
		field.String("client_name").
			MaxLen(100).
			Optional().
			Comment("客户端显示名称"),
		field.Enum("status").
			Values("pending", "authorized", "consumed", "expired", "denied").
			Default("pending").
			Comment("设备授权状态"),
		field.String("user_id").
			Optional().
			Comment("授权用户ID"),
		field.Int("interval").
			Default(5).
			Comment("轮询间隔秒数"),
		field.Int("invalid_attempts").
			Default(0).
			Comment("无效授权尝试次数"),
		field.Time("last_polled_at").
			Optional().
			Comment("最近轮询时间"),
		field.Time("expires_at").
			Comment("过期时间"),
		field.Time("authorized_at").
			Optional().
			Comment("授权时间"),
		field.Time("consumed_at").
			Optional().
			Comment("换取token时间"),
		field.Time("created_at").
			Default(time.Now),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the DeviceAuthSession.
func (DeviceAuthSession) Edges() []ent.Edge {
	return nil
}

// Indexes of the DeviceAuthSession.
func (DeviceAuthSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("device_code").Unique(),
		index.Fields("user_code").Unique(),
		index.Fields("status"),
		index.Fields("expires_at"),
	}
}
