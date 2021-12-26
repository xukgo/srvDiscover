package srvDiscover

import (
	"testing"
	"time"
)

func Test_initConf(t *testing.T) {
	repo := new(Repo)
	err := repo.InitFromPath("SrvDiscover.xml")
	if err != nil {
		t.FailNow()
	}
	err = repo.InitClient()
	if err != nil {
		t.FailNow()
	}
	err = repo.StartRegister(nil)
	if err != nil {
		t.FailNow()
	}
	err = repo.StartSubscribe()
	if err != nil {
		t.FailNow()
	}

	for {
		time.Sleep(time.Hour)
	}
}
