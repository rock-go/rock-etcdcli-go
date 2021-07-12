package etcdcli

import "github.com/rock-go/rock/lua"

func (l *clientLua) header(out lua.Printer) {
	out.Printf("type: %s", l.Type())
	out.Printf("uptime: %s", l.U.Format("2006-01-02 15:04:06"))
	out.Printf("version: v1.0.0")
	out.Println("")
}

func (l *clientLua) Show(out lua.Printer) {
	l.header(out)
	out.Printf("name = %s", l.Name())
	out.Printf("endpoint = %s", l.cli.cfg.Endpoint)
	out.Printf("username = %s", l.cli.cfg.Username)
	out.Printf("password = ******")
	out.Printf("node_id = %s", l.cli.cfg.NodeID)
	out.Printf("timeout = %s", l.cli.cfg.Timeout.String())
	out.Printf("ttl = %d", l.cli.cfg.TTL)
}

func (l *clientLua) Help(out lua.Printer) {
	l.header(out)
	out.Printf("not have export value")
}
