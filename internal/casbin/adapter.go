package casbin

import (
	"fmt"
	"shadmin/ent"
	"shadmin/internal/cacher"

	redisadapter "github.com/ahaodev/casbin-redis-adapter/v3"
	"github.com/casbin/casbin/v3/persist"
	entadapter "github.com/casbin/ent-adapter"
	adapterent "github.com/casbin/ent-adapter/ent"
)

const casbinKey = "casbin_rules"

func NewAdapter(entClient *ent.Client, conf cacher.RedisConfig) (persist.Adapter, error) {
	if conf.Addr != "" {
		return newRedisAdapter(conf)
	}
	return newEntAdapter(entClient)
}

func newEntAdapter(entClient *ent.Client) (persist.Adapter, error) {
	adapterClient := adapterent.NewClient(adapterent.Driver(entClient.Driver()))
	return entadapter.NewAdapterWithClient(adapterClient)
}

func newRedisAdapter(conf cacher.RedisConfig) (persist.Adapter, error) {
	adapterKey := casbinKey
	if conf.DB != 0 {
		adapterKey = fmt.Sprintf("%s:%d", casbinKey, conf.DB)
	}
	config := &redisadapter.Config{Network: "tcp", Address: conf.Addr, Key: adapterKey}
	adapter, _ := redisadapter.NewAdapter(config)
	return adapter, nil
}
