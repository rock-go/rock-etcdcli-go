
## 一、go 直接调用

```go
package main

import (
	"time"

	etcdcli "github.com/rock-go/rock-etcdcli-go"
)

func main() {
	cli := etcdcli.NewClient(&etcdcli.Config{
		Endpoint: "host1:2379,host2:2379,host3:2379",      // etcd 服务端地址, 多个地址用英文逗号分割
		Username: "4a0301f1-9d2d-41b1-e50f-97af56af132d",  // etcd 的用户名
		Password: "CE79311BeXkW37OM",                      // etcd 的密码
		NodeID:   "4a0301f1-9d2d-41b1-e50f-97af56af132d",  // 节点ID
		Timeout:  5 * time.Second,                         // 操作 etcd 超时时间
		TTL:      60,                                      // 节点存活的 TTL, 单位: 秒
	})
	
	// 启动服务
	_ = cli.Start()

	// 停止服务
	_ = cli.Close()
}
```

## 二、lua 调用

### 1. 编写 etcdcli.lua 配置文件
```lua
-- etcdcli.lua
local cli = etcd.client {
    name = "test-cli",                                   -- Lua虚拟机运行的名字
    endpoint = "host1:2379,host2:2379,host3:2379",       -- etcd 服务端地址, 多个地址用英文逗号分割
    username = "4a0301f1-9d2d-41b1-e50f-97af56af132d",   -- etcd 的用户名
    password = "CE79311BeXkW37OM",                       -- etcd 的密码
    node_id = "4a0301f1-9d2d-41b1-e50f-97af56af132d",    -- 节点ID
    timeout = "3s",                                      -- 操作 etcd 超时时间
    ttl = 60,                                            -- 节点存活的 TTL, 单位: 秒
}

-- 启动服务
proc.start(cli)

-- 停止服务
proc.close(cli)
```

### 2. 将组件注入到虚拟机中

```go
package main

import (
	"github.com/rock-go/rock"
	etcdcli "github.com/rock-go/rock-etcdcli-go"
	"github.com/rock-go/rock/xcall"
)

func main() {
	etcdcli.LuaInjectApi(xcall.Rock)
	rock.Setup(xcall.Rock)
}
```

### 3. 通过 console 加载并执行脚本
```shell
load etcdcli.lua
```
