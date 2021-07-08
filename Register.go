/**
 * @Author: zhangyw
 * @Description:
 * @File:  Register
 * @Date: 2020/6/2 17:04
 */

package srvDiscover

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"log"
	"sync"
	"time"
)

var stateLocker = new(sync.RWMutex)
var currentNodeState = STATE_ONLINE

func (this *Repo) GetState()string {
	var res string
	stateLocker.RLock()
	res = currentNodeState
	stateLocker.RUnlock()
	return res
}

func (this *Repo) ChangeState(state string) {
	stateLocker.Lock()
	currentNodeState = state
	stateLocker.Unlock()
}

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
			time.Sleep(time.Second)
			continue
		}

		this.fillRegMoudleInfo(srvInfo, regOption.BeforeRegister)
		err := this.clientUpdateLeaseContent(lease, srvInfo, regOption)
		if err != nil {
			log.Printf("clientUpdateLeaseContent error:%s\n", err.Error())
			this.client.Lease.Close()
			time.Sleep(time.Second)
			continue
		}

		this.KeepaliveLease(lease, srvInfo, regOption)
		this.client.Close()
	}
}

func (this *Repo) KeepaliveLease(lease *clientv3.LeaseGrantResponse, srvInfo *RegisterInfo, regOption *RegisterOption) {
	keepaliveChan, err := this.client.KeepAlive(context.TODO(), lease.ID) //这里需要一直不断，context不允许设置超时
	if err != nil || keepaliveChan == nil {
		if err != nil {
			log.Printf("client KeepAlive error:%s\n", err.Error())
		}
		time.Sleep(time.Second)
		return
	}

	timeSaved := time.Now()
	for {
		select {
		case keepaliveResponse, ok := <-keepaliveChan:
			if !ok || keepaliveResponse == nil {
				log.Printf("keepaliveResponse error\n")
				return
			}
			//fmt.Println("keepaliveResponse", keepaliveResponse)
			break
		default:
			if !regOption.AlwaysUpdate {
				time.Sleep(1000 * time.Millisecond)
				continue
			}

			if time.Since(timeSaved) < regOption.Interval {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			this.fillRegMoudleInfo(srvInfo, regOption.BeforeRegister)
			err := this.clientUpdateLeaseContent(lease, srvInfo, regOption)
			if err != nil {
				log.Printf("clientUpdateLeaseContent error:%s\n", err.Error())
				this.client.Lease.Close()
				return
			}

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

func (this *Repo) fillRegMoudleInfo(info *RegisterInfo, beforeRegisterFunc BeforeRegisterFunc) {
	if beforeRegisterFunc != nil {
		beforeRegisterFunc(info)
	}

	stateLocker.RLock()
	info.Global.State = currentNodeState
	stateLocker.RUnlock()
	info.Global.RefreshTimestamp(time.Now())
}
