package etcdcli

import (
	"reflect"

	"github.com/rock-go/rock/lua"
	"github.com/rock-go/rock/xcall"
	"github.com/rock-go/rock/xreflect"
)

type clientLua struct {
	lua.Super
	cli   *client                 // client
	state lua.LightUserDataStatus // 运行状态
}

// Start lua方式启动
func (l *clientLua) Start() error {
	err := l.cli.Start()
	if err == nil {
		l.state = lua.RUNNING
	}
	return err
}

// Close lua方式停止
func (l *clientLua) Close() error {
	err := l.cli.Close()
	if err == nil {
		l.state = lua.CLOSE
	}
	return err
}

// State 运行状态
func (l *clientLua) State() lua.LightUserDataStatus {
	return l.state
}

// Inject 注入到Lua环境中
func Inject(env xcall.Env) {
	kv := lua.NewUserKV()
	kv.Set("client", lua.NewFunction(constructor))
	env.SetGlobal("etcd", kv)
}

func constructor(state *lua.LState) int {
	tbl := state.CheckTable(1)
	var cfg Config
	if cfg.Name == "" {
		cfg.Name = "etcdcli"
	}
	if err := xreflect.ToStruct(tbl, &cfg); err != nil {
		state.RaiseError("%s", err)
		return 0
	}

	if err := cfg.validate(); err != nil {
		state.RaiseError("%s", err)
		return 0
	}

	proc := state.NewProc(cfg.Name, reflect.TypeOf((*clientLua)(nil)).String())
	if proc.IsNil() {
		cli := &clientLua{cli: New(cfg), state: lua.INIT}
		proc.Set(cli)
	} else {
		proc.Value.(*clientLua).cli.cfg = cfg
	}

	state.Push(proc)
	return 1
}
