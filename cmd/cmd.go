package cmd

import (
	"encoding/json"
	"errors"
	"net"
)

type Arkcmd struct {
	Name string   `json:"name"`
	Cmd  string   `json:"cmd"`
	Opts []string `json:"opts"`
}

func (c *Arkcmd) SendCmd(conn net.Conn) error {
	defer conn.Close()
	bufc, _ := json.Marshal(c)
	_, err := conn.Write(bufc)
	if err != nil {
		return err
	}
	buf := make([]byte, 8)
	n, err := conn.Read(buf[:])
	if err != nil {
		return err
	}
	ret := string(buf[0:n])
	if ret != "OK" {
		return errors.New(ret)
	}
	return nil
}

func GetPFcmds(run_dir string) map[string]*Arkcmd {
	pfcmds := make(map[string]*Arkcmd)
	pfcmds["check"] = &Arkcmd{"CheckPF", "/sbin/pfctl", []string{"-nf", run_dir + "pf.conf"}}
	pfcmds["backup"] = &Arkcmd{"BackupPF", "/bin/mv", []string{"/etc/pf.conf", "/etc/pf.conf.prev"}}
	pfcmds["move"] = &Arkcmd{"MovePF", "/bin/mv", []string{run_dir + "pf.conf", "/etc/"}}
	pfcmds["apply"] = &Arkcmd{"ApplyPF", "/sbin/pfctl", []string{"-f", "/etc/pf.conf"}}
	pfcmds["revert"] = &Arkcmd{"RevertPF", "/bin/mv", []string{"/etc/pf.conf.prev", "/etc/pf.conf"}}
	return pfcmds
}
