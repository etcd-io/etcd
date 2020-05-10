package pkg

import "os/exec"

func fn() {
	exec.Command("ls")
	exec.Command("ls arg1") // want `first argument to exec`
	exec.Command(`C:\Program Files\this\is\insane.exe`)
	exec.Command("/Library/Application Support/VMware Tools/vmware-tools-daemon")
}
