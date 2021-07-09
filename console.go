package etcdcli

import "github.com/rock-go/rock/lua"

func (l *clientLua) header(out lua.Printer) {
	out.Printf("type: %s", l.Type() )
	out.Printf("uptime: %s" , l.U.Format("2006-01-02 15:04:06"))
	out.Printf("version: v1.0.0")
	out.Println("")
}

func (l *clientLua) Show(out lua.Printer) {
	l.header(out)
	//type Config struct {
	//	Name     string        `json:"name"     yaml:"name"     lua:"name,etcdcli"`  // 名字, Lua虚拟机用
	//	Endpoint string        `json:"endpoint" yaml:"endpoint" lua:"endpoint"` 	 // etcd 服务地址, 多个地址用,分割
	//	Username string        `json:"username" yaml:"username" lua:"username"`      // etcd 用户名
	//	Password string        `json:"password" yaml:"password" lua:"password"`      // etcd 密码
	//	NodeID   string        `json:"node_id"  yaml:"node_id"  lua:"node_id"`       // 节点ID
	//	Timeout  time.Duration `json:"timeout"  json:"timeout"  lua:"timeout"`       // etcd 操作超时时间
	//	TTL      int64         `json:"ttl"      yaml:"ttl"      lua:"ttl"`           // 存活TTL时长, 单位: 秒
	//}

	out.Printf("name = %s" , l.Name())
	out.Printf("endpoint = %s" , l.cli.cfg.Endpoint)
	out.Printf("username = %s" , l.cli.cfg.Username)
	out.Printf("password = ******")
	out.Printf("node_id = %s" , l.cli.cfg.NodeID)
	out.Printf("timeout = %s" , l.cli.cfg.Timeout.String())
	out.Printf("ttl = %d" , l.cli.cfg.TTL)
}

func (l *clientLua) Help(out lua.Printer) {
	l.header(out)
	out.Printf("not have export value")
}