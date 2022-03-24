package srvDiscover

import (
	"os"
	"testing"
	"time"
)

func Test_initConf(t *testing.T) {
	fi, err := os.Open("SrvDiscover.xml")
	if err != nil {
		t.FailNow()
	}
	defer fi.Close()

	repo := new(Repo)
	err = repo.InitFromReader(fi)
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
