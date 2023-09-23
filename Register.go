/**
 * @Author: hermes
 * @Description:
 * @File:  Register
 * @Date: 2020/6/2 17:04
 */

package srvDiscover

import (
	"context"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var stateLocker = new(sync.RWMutex)
var currentNodeState = STATE_NOTREADY
var updateRegisterAction int32 = 0

func (this *Repo) UpdateOnce() {
	atomic.StoreInt32(&updateRegisterAction, 1)
}
func (this *Repo) GetState() string {
	var res string
	stateLocker.RLock()
	res = currentNodeState
	stateLocker.RUnlock()
	return res
}

func (this *Repo) ChangeState(state string) {
	stateLocker.Lock()
	currentNodeState = state
	atomic.StoreInt32(&updateRegisterAction, 1)
	stateLocker.Unlock()
}

// Register
// Grante: 创建一个 lease 对象；
// Revoke: 释放一个 lease 对象；
// TimeToLive: 获取 lease 剩余的 TTL 时间；
// Leases: 列举 etcd 中的所有 lease；
// KeepAlive: 自动定时对 lease 续约；
// KeepAliveOnce: 为 lease 续约一次，代码注释中说大部分情况下都应该使用 KeepAlive；
// Close: 关闭当前客户端建立的所有 lease；
func (this *Repo) Register(srvInfo *RegisterInfo, options ...RegisterOptionFunc) {
	regOption := new(RegisterOption)
	*regOption = defaultRegisterOption

	for _, op := range options {
		op(regOption)
	}

	var lease *clientv3.LeaseGrantResponse = nil
	var err error

	for {
		ctx, _ := context.WithTimeout(context.TODO(), regOption.ConnTimeout)
		lease, err = this.client.Grant(ctx, regOption.TTLSec)
		if err != nil || lease == nil {
			if err != nil {
				log.Printf("client Grant error:%s\n", err.Error())
			}
			regOption.ResultCallback(fmt.Errorf("client Grant error:%w", err))
			time.Sleep(time.Second)
			continue
		}

		this.fillRegModuleInfo(srvInfo, regOption.BeforeRegister)
		err := this.clientUpdateLeaseContent(lease, srvInfo, regOption)
		if err != nil {
			log.Printf("clientUpdateLeaseContent error:%s\n", err.Error())
			regOption.ResultCallback(fmt.Errorf("clientUpdateLeaseContent error:%w", err))
			this.client.Lease.Revoke(context.TODO(), lease.ID)
			time.Sleep(time.Second)
			continue
		}

		this.KeepaliveLease(lease, srvInfo, regOption)
	}
}

func (this *Repo) KeepaliveLease(lease *clientv3.LeaseGrantResponse, srvInfo *RegisterInfo, regOption *RegisterOption) {
	keepaliveChan, err := this.client.KeepAlive(context.TODO(), lease.ID) //这里需要一直不断，context不允许设置超时
	if err != nil || keepaliveChan == nil {
		if err != nil {
			log.Printf("client KeepAlive error:%s\n", err.Error())
		}
		regOption.ResultCallback(fmt.Errorf("client KeepAlive error:%w", err))
		this.client.Lease.Revoke(context.TODO(), lease.ID)
		time.Sleep(time.Millisecond * 100)
		return
	}

	//if regOption.BeforeRegister == nil {
	//	for range keepaliveChan {
	//	}
	//	log.Printf("keepaliveChan error\n")
	//	this.client.Lease.Revoke(context.TODO(), lease.ID)
	//	return
	//}

	timeSaved := time.Now()
	for {
		select {
		case keepaliveResponse, ok := <-keepaliveChan:
			if !ok || keepaliveResponse == nil {
				log.Printf("keepaliveResponse error\n")
				regOption.ResultCallback(fmt.Errorf("keepaliveChan Response error"))
				this.client.Lease.Revoke(context.TODO(), lease.ID)
				return
			}
			//fmt.Println("keepaliveResponse", keepaliveResponse)
			break
		default:
			//强制更新操作，则不进入常规判断，直接更新
			if atomic.LoadInt32(&updateRegisterAction) > 0 {
				atomic.StoreInt32(&updateRegisterAction, 0)
			} else {
				if !regOption.AlwaysUpdate {
					regOption.ResultCallback(nil)
					time.Sleep(1000 * time.Millisecond)
					continue
				}

				if time.Since(timeSaved) < regOption.Interval {
					regOption.ResultCallback(nil)
					time.Sleep(200 * time.Millisecond)
					continue
				}
			}

			this.fillRegModuleInfo(srvInfo, regOption.BeforeRegister)
			err := this.clientUpdateLeaseContent(lease, srvInfo, regOption)
			if err != nil {
				log.Printf("clientUpdateLeaseContent error:%s\n", err.Error())
				regOption.ResultCallback(fmt.Errorf("clientUpdateLeaseContent error:%w", err))
				this.client.Lease.Revoke(context.TODO(), lease.ID)
				//this.client.Lease.Close()
				return
			}

			regOption.ResultCallback(nil)
			timeSaved = time.Now()
		}
	}
}

func (this *Repo) clientUpdateLeaseContent(lease *clientv3.LeaseGrantResponse, srvInfo *RegisterInfo, regOption *RegisterOption) error {
	key := srvInfo.FormatRegisterKey(regOption.Namespace)
	value := srvInfo.Serialize()
	valueStr := string(value)

	//fmt.Println("keep", key, valueStr)
	_, err := this.client.Put(context.TODO(), key, valueStr, clientv3.WithLease(lease.ID))
	if err != nil {
		log.Printf("client put error:%s\n", err.Error())
	}
	return err
}

func (this *Repo) fillRegModuleInfo(info *RegisterInfo, beforeRegisterFunc BeforeRegisterFunc) {
	if beforeRegisterFunc != nil {
		beforeRegisterFunc(info)
	}

	stateLocker.RLock()
	info.Global.State = currentNodeState
	stateLocker.RUnlock()
	info.Global.RefreshTimestamp(time.Now())
}
