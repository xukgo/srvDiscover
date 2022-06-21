package srvDiscover

import "bytes"

type Ekv struct {
	Key   []byte
	Value []byte
}

func InitEkv(k, v []byte) Ekv {
	return Ekv{k, v}
}

type Ekvs []Ekv

func (list Ekvs) Len() int {
	return len(list)
}
func (list Ekvs) Less(i, j int) bool {
	return bytes.Compare(list[i].Key, list[j].Key) < 0
}
func (list Ekvs) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}
