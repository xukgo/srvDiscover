/**
 * @Author: hermes
 * @Description:
 * @File:  RegisterInfo
 * @Date: 2020/5/11 13:34
 */

package srvDiscover

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/xukgo/gsaber/utils/stringUtil"
	"time"
)

type RegisterGlobalInfo struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	NodeId    string `json:"nodeId"`
	Version   string `json:"version"`
	PrivateIp string `json:"privateIP"`
	PublicIP  string `json:"publicIP"`
	Timestamp string `json:"timestamp"`
}

func (this *RegisterGlobalInfo) RefreshTimestamp(dt time.Time) {
	this.Timestamp = fmt.Sprintf("%d", dt.UnixNano()/int64(time.Millisecond))
}

type RegisterProfileInfo struct {
	Cpu    int `json:"cpu"`
	IO     int `json:"io"`
	Disk   int `json:"disk"`
	Memory int `json:"memory"`
	Socket int `json:"socket"`
}

type RegisterInfo struct {
	Global   RegisterGlobalInfo      `json:"global"`
	SvcInfos []RegisterSvcDefineConf `json:"SvcInfo"`
	Profile  RegisterProfileInfo     `json:"profile"`
	Private  json.RawMessage         `json:"private"`
}

func (this *RegisterInfo) FormatRegisterKey(namespace string) string {
	key := fmt.Sprintf("registry.%s.%s.%s", namespace, this.GetServiceName(), this.UniqueId())
	return key
}

func (this *RegisterInfo) Serialize() []byte {
	gson, _ := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(this)
	return gson
}

func (this *RegisterInfo) Deserialize(data []byte) error {
	return jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(data, this)
}

func (this *RegisterInfo) GetServiceName() string {
	return this.Global.Name
}

func (this *RegisterInfo) UniqueId() string {
	md5Str := fmt.Sprintf("%x", md5.Sum([]byte(this.Global.PrivateIp+this.Global.NodeId)))
	return md5Str
}

func (this *RegisterInfo) DeepClone(privateCopy bool) RegisterInfo {
	model := RegisterInfo{
		Global:   this.Global,
		Profile:  this.Profile,
		SvcInfos: this.SvcInfos,
	}

	if privateCopy && len(this.Private) > 0 {
		model.Private = this.Private
	}
	return model
}

func (this *RegisterInfo) GetSvcInfo(name string) *RegisterSvcDefineConf {
	for idx := range this.SvcInfos {
		if stringUtil.CompareIgnoreCase(this.SvcInfos[idx].Name, name) {
			return &this.SvcInfos[idx]
		}
	}
	return nil
}

func (this *RegisterInfo) GetPort(name string, defaultPort int) int {
	for idx := range this.SvcInfos {
		if stringUtil.CompareIgnoreCase(this.SvcInfos[idx].Name, name) {
			return this.SvcInfos[idx].Port
		}
	}
	return defaultPort
}

//func (this *RegisterInfo) MakeEmpty() {
//	*this = RegisterInfo{}
//}
