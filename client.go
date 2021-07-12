package etcdcli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/rock-go/rock/logger"
	"github.com/rock-go/rock/service"
	"github.com/rock-go/rock/xcall"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	activePrefix = "rock-go/active/"
	scriptPrefix = "rock-go/script/"
)

var (
	ErrLogged       = errors.New("节点已经登录")
	ErrAlreadyStart = errors.New("client 已经运行")
	ErrEndpoints    = errors.New("endpoint 必须输入")
	ErrNotStarted   = errors.New("client 未没有")
)

type client struct {
	cli     *clientv3.Client   // etcd client
	cfg     *Config            // client 配置信息
	ctx     context.Context    // ctx
	cancel  context.CancelFunc // cancel 取消函数
	lease   clientv3.LeaseID   // 注册 TTL 时生成的租约 ID, Close 时要通知 etcd 删除
	active  string             // 该 client 在 etcd 上保持TTL存活的 key
	script  string             // 该 client 在 etcd 订阅配置 namespace 前缀
	running bool               // 是否正在运行
	codes   map[string]*code   // 从 etcd 拿到的所有配置信息
}

// NewClient 创建一个client
func NewClient(cfg *Config) *client {
	return &client{cfg: cfg}
}

// Start 启动Client
func (c *client) Start() (err error) {
	if c.running {
		return ErrAlreadyStart
	}

	c.cli, err = clientv3.New(c.cfg.toEtcdConfig())
	if err != nil {
		return err
	}
	c.running = true
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.active = activePrefix + c.cfg.NodeID
	c.script = scriptPrefix + c.cfg.NodeID + "/"
	c.codes = make(map[string]*code, 16)

	if err = c.keepalive(); err != nil {
		return err
	}
	return c.read()
}

// Close 关闭client
func (c *client) Close() (err error) {
	if !c.running {
		return ErrNotStarted
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()
	// 删除租约
	if c.lease != clientv3.NoLease {
		if _, err = c.cli.Revoke(ctx, c.lease); err == nil {
			c.lease = clientv3.NoLease
		}
	}

	c.cancel()
	c.running = false
	return err
}

// keepalive 注册带有TTL的key保持在线状态
func (c *client) keepalive() error {

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()
	// 检查是否重复登录
	if resp, err := c.cli.Get(ctx, c.active, clientv3.WithCountOnly()); err != nil {
		return err
	} else if resp.Count > 0 {
		return ErrLogged
	}

	// 申请租约
	lease, err := c.cli.Grant(ctx, c.cfg.TTL)
	if err != nil {
		return err
	}
	ch, err := c.cli.KeepAlive(c.ctx, lease.ID)
	if err != nil {
		return err
	}
	go func() {
		for range ch {
		}
	}()

	if _, err = c.cli.Put(ctx, c.active, "", clientv3.WithLease(lease.ID)); err == nil {
		c.lease = lease.ID
	}

	return err
}

// read 从 etcd 读取配置
func (c *client) read() error {
	// 先拉取已经存在的配置
	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	resp, err := c.cli.Get(ctx, c.script, clientv3.WithPrefix())
	cancel()
	if err != nil {
		return err
	}

	for _, kv := range resp.Kvs {
		cod := &code{}
		if err = json.Unmarshal(kv.Value, cod); err != nil {
			continue
		}
		c.codes[cod.Name] = cod
		doService(cod , true)
	}
	if len(resp.Kvs) > 0 {
		c.report()
	}

	doWakeup()
	go c.watch()

	return nil
}

// watch 监听读取配置变更
func (c *client) watch() {
	ch := c.cli.Watch(c.ctx, c.script, clientv3.WithPrefix())

	for res := range ch {
		for _, event := range res.Events {

			// 当删除时, etcd只会发送删除的key, 不会发送value
			if event.Type == mvccpb.DELETE {
				keys := strings.Split(string(event.Kv.Key), c.script)
				if len(keys) > 1 {
					name := keys[1]
					delete(c.codes, name)
					delService(name)
				}
				continue
			}

			cod := &code{}
			if err := json.Unmarshal(event.Kv.Value, cod); err != nil {
				continue
			}
			c.codes[cod.Name] = cod
			doService(cod , false)
		}
		if res.Canceled {
			return
		}

		// 每次变化后向 etcd 上报配置状态
		c.report()
	}
}

// report 向etcd上报最新脚本信息
func (c *client) report() {
	codes := make([]*code, 0, len(c.codes))
	for _, v := range c.codes {
		codes = append(codes, v)
	}

	data, err := json.Marshal(codes)
	if err != nil {
		logger.Error(err)
		return
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	_, err = c.cli.Put(ctx, c.active, string(data), clientv3.WithLease(c.lease))
	cancel()
	if err != nil {
		logger.Error(err)
	}
}

// doService 让 lua 虚拟机执行配置脚本
func doService(c *code , reg bool) {
	defer func() {
		if cause := recover(); cause != nil {
			logger.Errorf("[执行 %s 发生 panic]: %v", c.Name, cause)
		}
	}()

	logger.Infof("[执行]: %s", c.Name)
	data, err := base64.StdEncoding.DecodeString(c.Chunk)
	if err != nil {
		logger.Warn(err)
		return
	}

	if reg {
		err = service.Reg(c.Name , data , xcall.Rock)
	} else {
		err = service.Do(c.Name, data, xcall.Rock)
	}
}

func doWakeup() {
	if e := service.Wakeup(); e != nil {
		logger.Error("%v" , e)
	}
}

// delService 从 lua 虚拟机里删除服务
func delService(name string) {
	defer func() {
		if cause := recover(); cause != nil {
			logger.Errorf("[删除 %s 发生 panic]: %v", name, cause)
		}
	}()

	logger.Infof("[删除]: %s", name)
	if err := service.Del(name); err != nil {
		logger.Error(err)
	}
}
