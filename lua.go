package etcdcli

import (
	"reflect"

	"github.com/rock-go/rock/lua"
	"github.com/rock-go/rock/xcall"
)

var TClientLua = reflect.TypeOf((*clientLua)(nil)).String()

type clientLua struct {
	lua.Super
	cli *client // client
}

func newClientLua(cfg *Config) *clientLua {
	cl := &clientLua{
		cli: NewClient(cfg),
	}

	cl.S = lua.INIT
	cl.T = TClientLua
	return cl
}

func constructor(L *lua.LState) int {
	cfg := newConfig(L)
	proc := L.NewProc(cfg.Name, TClientLua)

	if proc.IsNil() {
		proc.Set(newClientLua(cfg))
	} else {
		proc.Value.(*clientLua).cli.cfg = cfg
	}

	L.Push(proc)
	return 1
}

func (l *clientLua) Name() string {
	return l.cli.cfg.Name
}

// Start lua方式启动
func (l *clientLua) Start() error {
	if err := l.cli.Start(); err != nil {
		l.S = lua.PANIC
		return err
	}

	l.S = lua.RUNNING
	return nil
}

// Close lua方式停止
func (l *clientLua) Close() error {
	err := l.cli.Close()
	if err == nil {
		l.S = lua.CLOSE
	}
	return err
}

// LuaInjectApi 注入到Lua环境中
func LuaInjectApi(env xcall.Env) {
	kv := lua.NewUserKV()
	kv.Set("client", lua.NewFunction(constructor))
	env.SetGlobal("etcd", kv)
}
