/**
 * @Author: xuk
 * @Description:
 * @File:  Conf
 * @Date: 2020/6/3 9:25
 */

package srvDiscover

import (
	"encoding/xml"
	"fmt"
	uuid "github.com/satori/go.uuid"
	"github.com/xukgo/gsaber/utils/arrayUtil"
	"github.com/xukgo/gsaber/utils/netUtil"
	"strings"
	"time"
)

const DEFAULT_NAMESPACE = "voice"

type PredefEndpoint struct {
	Endpoints []string
	UserName  string
	Password  string
}

type ConfRoot struct {
	XMLName       xml.Name
	Username      string           `xml:"Username"`       //
	Password      string           `xml:"Password"`       //
	Timeout       int              `xml:"Timeout"`        //etcd连接超时时间,单秒秒
	Endpoints     []string         `xml:"Endpoints>Addr"` //etcd服务器地址, 172.16.0.212:2379
	ClientTls     *ClientTlsConfig `xml:"Tls"`            //
	RegisterConf  *RegisterConf    `xml:"Register"`
	SubScribeConf *SubscribeConf   `xml:"Subscribe"`
}

type ClientTlsConfig struct {
	CaFilePath   string `xml:"ca,attr"`
	CertFilePath string `xml:"cert,attr"`
	KeyFilePath  string `xml:"key,attr"`
}

type RegisterConf struct {
	Interval  int                     `xml:"Interval"`  //注册间隔, 单位秒, 默认值为2
	TTL       int                     `xml:"TTL"`       //注册服务的TimeToLive, 单位秒,默认值为6
	Namespace string                  `xml:"Namespace"` //注册Key的namespace, 默认为voice, /registry/namespace/..
	Global    RegisterGlobalConf      `xml:"Global"`
	SvcInfos  []RegisterSvcDefineConf `xml:"SvcInfos>Svc"`
	//PrivateMap []SrvRegisterPrivateConf `xml:"PrivateMap>Private"`
}

type RegisterGlobalConf struct {
	Name            string `xml:"Name"`
	State           string `xml:"State"`
	NodeId          string `xml:"NodeId"`
	Version         string `xml:"Version"`
	PrivateIPString string `xml:"PrivateIP"`
	PrivateIP       string `xml:"-"`
	PublicIPString  string `xml:"PublicIP"`
	PublicIP        string `xml:"-"`
}

type RegisterSvcDefineConf struct {
	Name string `xml:"name,attr" json:"name"`
	Port int    `xml:"port,attr" json:"port"`
}

func (this *RegisterSvcDefineConf) DeepClone() *RegisterSvcDefineConf {
	model := new(RegisterSvcDefineConf)
	model.Name = this.Name
	model.Port = this.Port
	return model
}

type SubscribeSrvConf struct {
	Namespace string `xml:"Namespace"`
	Name      string `xml:"Name"`
	Version   string `xml:"Version"`
}

type SubscribeConf struct {
	Services []SubscribeSrvConf `xml:"Service"`
}

func (c *SubscribeConf) GetIndexByName(name string) int {
	for idx := range c.Services {
		if strings.EqualFold(c.Services[idx].Name, name) {
			return idx
		}
	}
	return -1
}

func (this *ConfRoot) FillWithXml(data []byte) error {
	err := xml.Unmarshal(data, this)
	if err != nil {
		return err
	}

	this.Username = strings.TrimSpace(this.Username)
	this.Password = strings.TrimSpace(this.Password)
	//反序列化后的处理
	if this.Timeout <= 0 {
		this.Timeout = 2
	}

	if this.RegisterConf != nil {
		this.RegisterConf.Namespace = strings.TrimSpace(this.RegisterConf.Namespace)
		if len(this.RegisterConf.Namespace) == 0 {
			this.RegisterConf.Namespace = defaultRegisterOption.Namespace
		}
		if this.RegisterConf.TTL == 0 {
			this.RegisterConf.TTL = int(defaultRegisterOption.TTLSec)
		}
		if this.RegisterConf.Interval == 0 {
			this.RegisterConf.Interval = int(defaultRegisterOption.Interval / time.Second)
		}

		//PrivateIP
		ip, err := convertRegisterIP(this.RegisterConf.Global.PrivateIPString)
		if err != nil {
			return err
		}
		this.RegisterConf.Global.PrivateIP = ip

		//PublicIP
		ip, err = convertRegisterIP(this.RegisterConf.Global.PublicIPString)
		if err == nil {
			this.RegisterConf.Global.PublicIP = ip
		}

		if len(this.RegisterConf.Global.NodeId) == 0 {
			this.RegisterConf.Global.NodeId = uuid.NewV1().String()
		}

		this.Endpoints = arrayUtil.StringsTrimSpaceFilterEmpty(this.Endpoints)
	}

	if this.SubScribeConf != nil {
		for idx := range this.SubScribeConf.Services {
			if len(this.SubScribeConf.Services[idx].Namespace) == 0 {
				if this.RegisterConf != nil {
					this.SubScribeConf.Services[idx].Namespace = this.RegisterConf.Namespace
				} else {
					this.SubScribeConf.Services[idx].Namespace = DEFAULT_NAMESPACE
				}
			}
		}
	}

	return err
}

func convertRegisterIP(ipString string) (string, error) {
	arr := strings.Split(ipString, ":")
	var filterArr []string
	if len(arr) == 1 {
		filterArr = nil
	} else {
		filterArr = strings.Split(arr[1], "|")
	}

	ipArr, err := netUtil.GetIPv4(arr[0], filterArr)
	if err != nil {
		return "", err
	}
	if len(ipArr) == 0 {
		return "", fmt.Errorf("no ip found")
	}
	return ipArr[0], nil
}

func (this *ConfRoot) GetRegisterOptionFuncs() []RegisterOptionFunc {
	if this.RegisterConf == nil {
		return nil
	}

	register := this.RegisterConf
	registerOp := make([]RegisterOptionFunc, 0, 4)
	registerOp = append(registerOp, WithTTL(int64(register.TTL)))
	registerOp = append(registerOp, WithRegisterNamespace(register.Namespace))
	registerOp = append(registerOp, WithRegisterInterval(time.Duration(register.Interval)*time.Second))
	registerOp = append(registerOp, WithRegisterConnTimeout(time.Duration(this.Timeout)*time.Second))
	return registerOp
}

func (this *ConfRoot) GetRegisterModule() (*RegisterInfo, error) {
	if this.RegisterConf == nil {
		return nil, fmt.Errorf("register conf is nil")
	}

	register := this.RegisterConf
	if len(register.Global.Name) == 0 {
		return nil, fmt.Errorf("register global.name is empty")
	}
	if len(register.Global.NodeId) == 0 {
		return nil, fmt.Errorf("register global.nodeId is empty")
	}
	if len(register.Global.Version) == 0 {
		return nil, fmt.Errorf("register global.version is empty")
	}
	if len(register.Global.PrivateIP) == 0 {
		return nil, fmt.Errorf("register global.ip is empty")
	}

	srvInfo := new(RegisterInfo)
	srvInfo.Global.Name = register.Global.Name
	srvInfo.Global.NodeId = register.Global.NodeId
	srvInfo.Global.Version = register.Global.Version
	srvInfo.Global.PrivateIp = register.Global.PrivateIP
	srvInfo.Global.PublicIP = register.Global.PublicIP
	srvInfo.Global.State = register.Global.State

	srvInfo.SvcInfos = register.SvcInfos
	//if len(register.SvcInfos) > 0 {
	//	portMap := register.SvcInfos
	//	srvInfo.Port = make(map[string]int)
	//	for i := range portMap {
	//		srvInfo.Port[portMap[i].Name] = portMap[i].Port
	//	}
	//} else {
	//	srvInfo.Port = nil
	//}

	return srvInfo, nil
}

func (this *ConfRoot) GetSubscribeBasicInfos() ([]SubBasicInfo, error) {
	if this.SubScribeConf == nil {
		return nil, fmt.Errorf("subscribe conf is nil")
	}
	subSrvs := this.SubScribeConf.Services
	infos := make([]SubBasicInfo, 0, len(subSrvs))
	for i := range subSrvs {
		infos = append(infos, *NewSubSrvBasicInfo(subSrvs[i].Name, subSrvs[i].Version, subSrvs[i].Namespace))
	}

	return infos, nil
}
