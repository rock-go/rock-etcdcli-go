package etcdcli

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/rock-go/rock/logger"
	"github.com/rock-go/rock/service"
	"github.com/rock-go/rock/xcall"
	"strings"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/v3"
)

const (
	activePrefix = "rock-go/active/"
	scriptPrefix = "rock-go/script/"
)

var (
	ErrChecksum     = errors.New("配置校验码不一致")
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

	// 申请租约
	lease, err := c.cli.Grant(ctx, c.cfg.TTL)
	if err != nil {
		return err
	}

	resp, err := c.cli.Txn(ctx).
		If(clientv3.Compare(clientv3.LeaseValue(c.active), "=", clientv3.NoLease)).
		Then(clientv3.OpPut(c.active, "", clientv3.WithLease(lease.ID))).
		Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return ErrLogged
	}

	c.lease = lease.ID
	// 保持续租
	ch, err := c.cli.KeepAlive(c.ctx, lease.ID)
	if err != nil {
		return err
	}
	go func() {
		for range ch {
		}
	}()

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
		cod.Error = doReg(cod)
		c.codes[cod.Name] = cod
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
		if res.Canceled {
			return
		}
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
			cod.Error = doService(cod)
			c.codes[cod.Name] = cod
		}
		// 每次变化后向 etcd 上报配置状态
		c.report()
	}
}

// report 向etcd上报最新脚本信息
func (c *client) report() {
	rs := make([]*report, 0, len(c.codes))
	for _, v := range c.codes {
		r := &report{Name: v.Name, Hash: v.Hash, Time: v.Time}
		rs = append(rs, r)
		if v.Error != nil {
			r.Error = v.Error.Error()
		}
	}

	data, err := json.Marshal(rs)
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
func doService(c *code) error {
	defer handle("执行脚本服务")
	// 校验 chunk 的 hash
	if !checksum(c) {
		return ErrChecksum
	}
	return service.Do(c.Name, c.Chunk, xcall.Rock)
}

func doReg(c *code) error {
	defer handle("reg 服务")
	// 校验 chunk 的 hash
	if !checksum(c) {
		return ErrChecksum
	}
	return service.Reg(c.Name, c.Chunk, xcall.Rock)
}

// checksum 校验 chunk 的 hash 是否一致
func checksum(c *code) bool {
	sum := md5.Sum(c.Chunk)
	hash := hex.EncodeToString(sum[:])
	return hash == c.Hash
}

func doWakeup() {
	defer handle("wakeup")
	if e := service.Wakeup(); e != nil {
		logger.Error("%v", e)
	}
}

// delService 从 lua 虚拟机里删除服务
func delService(name string) {
	defer handle("删除服务")

	logger.Infof("[删除服务]: %s", name)
	if err := service.Del(name); err != nil {
		logger.Errorf("[删除服务 %s 错误]: %v", name, err)
	}
}

// handle recover 错误
func handle(tag string) {
	cause := recover()
	if cause == nil {
		return
	}
	logger.Errorf("[%s panic]: %v", tag, cause)
}
