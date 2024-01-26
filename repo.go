/**
 * @Author: hermes
 * @Description:
 * @File:  Repo
 * @Date: 2020/5/9 10:09
 */

package srvDiscover

import (
	"context"
	"fmt"
	"github.com/xukgo/gsaber/utils/randomUtil"
	"github.com/xukgo/gsaber/utils/stringUtil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"io"
	"sort"
	"sync"
	"time"
)

//./etcdctl --endpoints=172.16.2.13:2479 get --prefix registry.voice

//const DEFAULT_CONN_TIMEOUT = 1500

type Repo struct {
	locker sync.RWMutex
	config *ConfRoot
	client *clientv3.Client //etcd客户端

	subsNodeCache map[string]*SubSrvNodeList

	subLicResultInfo *SubLicResultInfo
	licLocker        sync.RWMutex
	licPrivkey       string
	licWatchFunc     func(*LicResultInfo)

	//predefine
	predefEndpoint        *PredefEndpoint
	preDefRegisterVersion string
	preDefSubsVerDict     map[string]string
}

func (this *Repo) GetEtcdClient() *clientv3.Client {
	return this.client
}

func (this *Repo) SetPrivateIP(ip string) {
	this.config.RegisterConf.Global.PrivateIP = ip
}

func (this *Repo) SetPublicIP(ip string) {
	this.config.RegisterConf.Global.PublicIP = ip
}

func (this *Repo) SetNodeID(id string) {
	this.config.RegisterConf.Global.NodeId = id
}

func (this *Repo) WithPredefEndpoint(s *PredefEndpoint) {
	this.predefEndpoint = s
}
func (this *Repo) PreDefineRegisterVersion(ver string) {
	this.preDefRegisterVersion = ver
}
func (this *Repo) AddPreDefineSubsVersion(svcName string, ver string) {
	if this.preDefSubsVerDict == nil {
		this.preDefSubsVerDict = make(map[string]string, 4)
	}
	this.preDefSubsVerDict[svcName] = ver
}

//func (this *Repo) SetEndPoints(endpoints []string) {
//	this.config.Endpoints = endpoints
//}
//func (this *Repo) InitClient() error {
//	var err error
//	this.client, err = clientv3.New(clientv3.Config{
//		Username:    this.config.Username,
//		Password:    this.config.Password,
//		Endpoints:   this.config.Endpoints,
//		DialTimeout: time.Duration(this.config.Timeout) * time.Second,
//	})
//	if err != nil {
//		return err
//	}
//	return nil
//}

func ConfigUnmarshalFromReader(srcReader io.Reader) (*ConfRoot, error) {
	content, err := io.ReadAll(srcReader)
	if err != nil {
		return nil, err
	}

	srvConf := new(ConfRoot)
	err = srvConf.FillWithXml(content)
	if err != nil {
		return nil, err
	}
	return srvConf, nil
}

func (this *Repo) InitFromReader(srcReader io.Reader) error {
	srvConf, err := ConfigUnmarshalFromReader(srcReader)
	if err != nil {
		return err
	}

	this.config = srvConf
	this.replacePredefRegisterVersion()
	this.replacePredefSubsVersion()
	this.client, err = clientv3.New(clientv3.Config{
		Username:    this.config.Username,
		Password:    this.config.Password,
		Endpoints:   this.config.Endpoints,
		DialTimeout: time.Duration(this.config.Timeout) * time.Second,
	})
	if err != nil {
		return err
	}
	return nil
}

func (this *Repo) StartRegister(beforeRegisterFunc BeforeRegisterFunc, resultCallback RegisterResultCallback) error {
	if this.config == nil {
		return fmt.Errorf("register conf is nil")
	}

	registerOp := this.config.GetRegisterOptionFuncs()
	if beforeRegisterFunc != nil {
		registerOp = append(registerOp, WithBeforeRegister(beforeRegisterFunc))
	}
	if resultCallback != nil {
		registerOp = append(registerOp, WithRegisterResultCallback(resultCallback))
	} else {
		registerOp = append(registerOp, WithRegisterResultCallback(func(err error) {}))
	}

	srvInfo, err := this.config.GetRegisterModule()
	if err != nil {
		return err
	}

	go this.Register(srvInfo, registerOp...)
	return nil
}

func (this *Repo) GetPrefixKvs(prefix string) ([]Ekv, error) {
	if len(prefix) == 0 {
		prefix = "/registry."
	}
	response, err := this.client.Get(context.TODO(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	list := make([]Ekv, 0, len(response.Kvs))
	for _, kv := range response.Kvs {
		list = append(list, InitEkv(kv.Key, kv.Value))
	}
	vs := Ekvs(list)
	sort.Sort(vs)
	return vs, nil
}

func (this *Repo) StartSubscribe() error {
	if this.config == nil {
		return fmt.Errorf("register conf is nil")
	}

	subBasicInfos, err := this.config.GetSubscribeBasicInfos()
	if err != nil {
		return err
	}

	this.initSubsNodeCache(subBasicInfos)
	err = this.SubScribe(subBasicInfos)
	return err
}

func (this *Repo) GetLocalRegisterInfo() *RegisterConf {
	conf := this.config
	return conf.RegisterConf
}

func (this *Repo) GetSubsNames() []string {
	subsconf := this.config.SubScribeConf
	if subsconf == nil {
		return nil
	}
	if len(subsconf.Services) == 0 {
		return nil
	}

	arr := make([]string, 0, len(subsconf.Services))
	for idx := range subsconf.Services {
		arr = append(arr, subsconf.Services[idx].Name)
	}
	return arr
}

// 只会查询online的
func (this *Repo) GetServiceByName(name string) []RegisterInfo {
	this.locker.RLock()
	defer this.locker.RUnlock()

	var srvInfos []RegisterInfo = nil
	for srvName, srvNodeList := range this.subsNodeCache {
		if stringUtil.CompareIgnoreCase(srvName, name) {
			srvInfos = make([]RegisterInfo, 0, len(srvNodeList.NodeInfos))
			for n := range srvNodeList.NodeInfos {
				if stringUtil.CompareIgnoreCase(srvNodeList.NodeInfos[n].RegInfo.Global.State, STATE_ONLINE) {
					srvInfos = append(srvInfos, srvNodeList.NodeInfos[n].RegInfo.DeepClone(true))
				}
			}
			break
		}
	}
	return srvInfos
}

func (this *Repo) GetFilterServices(name string, filterFunc func(*SrvNodeInfo) bool) []RegisterInfo {
	this.locker.RLock()
	defer this.locker.RUnlock()

	var srvInfos []RegisterInfo = nil
	for srvName, srvNodeList := range this.subsNodeCache {
		if stringUtil.CompareIgnoreCase(srvName, name) {
			srvInfos = make([]RegisterInfo, 0, len(srvNodeList.NodeInfos))
			for n := range srvNodeList.NodeInfos {
				if filterFunc(srvNodeList.NodeInfos[n]) {
					srvInfos = append(srvInfos, srvNodeList.NodeInfos[n].RegInfo.DeepClone(true))
				}
				//if stringUtil.CompareIgnoreCase(srvNodeList.NodeInfos[n].RegInfo.Global.State, STATE_ONLINE) {
				//	srvInfos = append(srvInfos, srvNodeList.NodeInfos[n].RegInfo.DeepClone(false))
				//}
			}
			break
		}
	}
	return srvInfos
}

func (this *Repo) GetFilterServiceCount(name string, filterFunc func(*SrvNodeInfo) bool) int {
	this.locker.RLock()
	defer this.locker.RUnlock()

	var count = 0
	for srvName, srvNodeList := range this.subsNodeCache {
		if stringUtil.CompareIgnoreCase(srvName, name) {
			for n := range srvNodeList.NodeInfos {
				if filterFunc(srvNodeList.NodeInfos[n]) {
					count++
				}
			}
			break
		}
	}
	return count
}
func (this *Repo) GetRandomServiceArray(svcName string) []RegisterInfo {
	infos := this.GetServiceByName(svcName)
	if len(infos) == 0 {
		return nil
	}

	randomSortSlice(infos)
	return infos
}

func (this *Repo) GetRandomServiceByName(svcName string) (bool, RegisterInfo) {
	infos := this.GetServiceByName(svcName)
	if len(infos) == 0 {
		return false, RegisterInfo{}
	}

	idx := randomUtil.NewInt32(0, int32(len(infos)))
	return true, infos[idx].DeepClone(false)
}

func (this *Repo) GetServiceByNameAndNodeId(svcName string, id string) (bool, RegisterInfo) {
	this.locker.RLock()
	defer this.locker.RUnlock()

	for srvName, srvNodeList := range this.subsNodeCache {
		if srvName != svcName {
			continue
		}

		nodes := srvNodeList.NodeInfos
		for n := range nodes {
			if nodes[n].RegInfo.Global.NodeId != id {
				continue
			}

			state := nodes[n].RegInfo.Global.State
			if stringUtil.CompareIgnoreCase(state, STATE_ONLINE) || stringUtil.CompareIgnoreCase(state, STATE_BYPASS) {
				return true, srvNodeList.NodeInfos[n].RegInfo.DeepClone(false)
			}
			return false, RegisterInfo{}
		}
	}
	return false, RegisterInfo{}
}

func (this *Repo) initSubsNodeCache(subSrvInfos []SubBasicInfo) {
	serviceCount := len(subSrvInfos)
	if serviceCount <= 0 {
		return
	}

	this.subsNodeCache = make(map[string]*SubSrvNodeList)
	for m := 0; m < serviceCount; m++ {
		srvNodeList := new(SubSrvNodeList)
		srvNodeList.SubBasicInfo = *NewSubSrvBasicInfo(subSrvInfos[m].Name, subSrvInfos[m].Version, subSrvInfos[m].Namespace)
		srvNodeList.NodeInfos = make([]*SrvNodeInfo, 0, 1)
		this.subsNodeCache[subSrvInfos[m].Name] = srvNodeList
	}
}

func (this *Repo) replacePredefEndpoints() {
	if this.predefEndpoint != nil {
		this.config.Endpoints = this.predefEndpoint.Endpoints
		this.config.Username = this.predefEndpoint.UserName
		this.config.Password = this.predefEndpoint.Password
	}
}
func (this *Repo) replacePredefRegisterVersion() {
	if len(this.preDefRegisterVersion) > 0 {
		this.config.RegisterConf.Global.Version = this.preDefRegisterVersion
	}
}

func (this *Repo) replacePredefSubsVersion() {
	if len(this.preDefSubsVerDict) == 0 || this.config.SubScribeConf == nil {
		return
	}
	for name, ver := range this.preDefSubsVerDict {
		index := this.config.SubScribeConf.GetIndexByName(name)
		if index < 0 {
			continue
		}
		this.config.SubScribeConf.Services[index].Version = ver
	}
}

// 随机打乱数组
func randomSortSlice(arr []RegisterInfo) {
	if len(arr) <= 0 || len(arr) == 1 {
		return
	}

	for i := len(arr) - 1; i > 0; i-- {
		num := randomUtil.NewInt32(0, int32(i+1))
		arr[i], arr[num] = arr[num], arr[i]
	}
}

//func GetSrvDiscover() *Repo {
//	return srvDiscoverInstance
//}
//
//func GetSrvDiscoverConf() *ConfRoot {
//	return &srvDiscoverConf
//}

//func (this *ServiceDiscovery) TriggerRegister() {
//	this.registerHupChan <- true
//}
//

//func (this *Repo) GetConfig() *ConfRoot {
//	return this.config
//}

//func NewSrvDiscover(endpoints []string, options ...SdOption) (*Repo, error) {
//	if len(endpoints) == 0 {
//		return nil, fmt.Errorf("endpoints addrs is empty")
//	}
//
//	serviceDiscovery := &Repo{
//		Endpoints: endpoints,
//	}
//	serviceDiscovery.Timeout = DEFAULT_CONN_TIMEOUT * time.Millisecond
//
//	for _, op := range options {
//		op(serviceDiscovery)
//	}
//
//	var err error
//	serviceDiscovery.client, err = clientv3.New(clientv3.Config{
//		Endpoints:   endpoints,
//		DialTimeout: serviceDiscovery.Timeout,
//	})
//	if err != nil {
//		return nil, err
//	}
//
//	serviceDiscovery.subsNodeCache = make(map[string]*SubSrvNodeList)
//	serviceDiscovery.locker = &sync.RWMutex{}
//
//	return serviceDiscovery, nil
//}
