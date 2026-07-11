package casbin

import (
	"fmt"
	"shadmin/ent"
	"shadmin/internal/cacher"
	"shadmin/internal/casbin/entadapter"
	"shadmin/internal/casbin/redisadapter"

	"github.com/casbin/casbin/v3/persist"
)

const casbinKey = "casbin_rules"

func NewAdapter(entClient *ent.Client, conf cacher.RedisConfig) (persist.Adapter, error) {
	if conf.Addr != "" {
		return newRedisAdapter(conf)
	}
	return newEntAdapter(entClient)
}

func newEntAdapter(entClient *ent.Client) (persist.Adapter, error) {
	entAdapter, err := entadapter.NewAdapterWithClient(entClient)
	if err != nil {
		return nil, err
	}
	return entAdapter, nil
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
