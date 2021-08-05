package etcdcli

import (
	"strings"
	"time"

	"github.com/rock-go/rock/lua"
	"github.com/rock-go/rock/xreflect"

	"go.etcd.io/etcd/client/v3"
)

// code 从 etcd 接收的配置下发格式
type code struct {
	Name  string    `json:"name"`  // 配置名称
	Hash  string    `json:"hash"`  // Hash
	Chunk []byte    `json:"chunk"` // Lua 配置脚本 Base64
	Time  time.Time `json:"time"`  // 发布时间
	Error error     `json:"-"`     // 运行产生的错误
}

type report struct {
	Name  string    `json:"name"`
	Hash  string    `json:"hash"`
	Time  time.Time `json:"time"`
	Error string    `json:"error"`
}

// Config 配置文件
type Config struct {
	Name     string        `json:"name"     yaml:"name"     lua:"name,etcdcli"` // 名字, Lua虚拟机用
	Endpoint string        `json:"endpoint" yaml:"endpoint" lua:"endpoint"`     // etcd 服务地址, 多个地址用,分割
	Username string        `json:"username" yaml:"username" lua:"username"`     // etcd 用户名
	Password string        `json:"password" yaml:"password" lua:"password"`     // etcd 密码
	NodeID   string        `json:"node_id"  yaml:"node_id"  lua:"node_id"`      // 节点ID
	Timeout  time.Duration `json:"timeout"  json:"timeout"  lua:"timeout"`      // etcd 操作超时时间
	TTL      int64         `json:"ttl"      yaml:"ttl"      lua:"ttl"`          // 存活TTL时长, 单位: 秒
}

func newConfig(L *lua.LState) *Config {
	tbl := L.CheckTable(1)
	cfg := &Config{}
	if e := xreflect.ToStruct(tbl, cfg); e != nil {
		L.RaiseError("%v", e)
		return nil
	}

	if e := cfg.validate(); e != nil {
		L.RaiseError("%v", e)
		return nil
	}

	return cfg
}

// validate 校验一下
func (c *Config) validate() error {
	// 去除所有的空格
	c.Endpoint = strings.ReplaceAll(c.Endpoint, " ", "")
	if c.Endpoint == "" {
		return ErrEndpoints
	}

	if c.TTL < 10 || c.TTL > 3600 {
		c.TTL = 60
	}
	if c.Timeout < time.Second || c.Timeout > time.Minute {
		c.Timeout = 5 * time.Second
	}
	return nil
}

// toEtcdConfig 将配置信息转为 etcd 的 config
func (c *Config) toEtcdConfig() clientv3.Config {
	return clientv3.Config{
		Endpoints:   strings.Split(c.Endpoint, ","),
		Username:    c.Username,
		Password:    c.Password,
		DialTimeout: c.Timeout,
	}
}
