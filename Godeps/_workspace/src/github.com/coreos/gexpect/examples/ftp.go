package main

import gexpect "github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/gexpect"
import "log"

func main() {
	log.Printf("Testing Ftp... ")

	child, err := gexpect.Spawn("ftp ftp.openbsd.org")
	if err != nil {
		panic(err)
	}
	child.Expect("Name")
	child.SendLine("anonymous")
	child.Expect("Password")
	child.SendLine("pexpect@sourceforge.net")
	child.Expect("ftp> ")
	child.SendLine("cd /pub/OpenBSD/3.7/packages/i386")
	child.Expect("ftp> ")
	child.SendLine("bin")
	child.Expect("ftp> ")
	child.SendLine("prompt")
	child.Expect("ftp> ")
	child.SendLine("pwd")
	child.Expect("ftp> ")
	log.Printf("Success\n")
}
