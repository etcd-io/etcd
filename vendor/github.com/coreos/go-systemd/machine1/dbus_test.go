/*
Copyright 2015 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package machine1

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sd_dbus "github.com/coreos/go-systemd/dbus"
	"github.com/godbus/dbus"
)

const (
	machinePrefix = "machined-test-"
)

func mustCreateTestProcess(machineName string) (pid int) {
	testServiceName := machineName + ".service"
	systemdRun, err := exec.LookPath("systemd-run")
	if err != nil {
		panic(err.Error())
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		panic(err.Error())
	}
	cmd := exec.Command(systemdRun, "--unit="+testServiceName, sleep, "5000")
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("systemd-run failed: %q", out))
	}
	dbusConn, err := sd_dbus.New()
	if err != nil {
		panic(err.Error())
	}
	defer dbusConn.Close()
	mainPIDProperty, err := dbusConn.GetServiceProperty(testServiceName, "MainPID")
	if err != nil {
		panic(err.Error())
	}
	mainPID := mainPIDProperty.Value.Value().(uint32)
	return int(mainPID)
}

func TestMachine(t *testing.T) {
	machineNames := []string{
		machinePrefix + "register-" + generateRandomLabel(8),
		machinePrefix + "register-with-network-" + generateRandomLabel(8),
		machinePrefix + "create-" + generateRandomLabel(8),
		machinePrefix + "create-with-network-" + generateRandomLabel(8),
	}
	leaders := []int{
		mustCreateTestProcess(machineNames[0]),
		mustCreateTestProcess(machineNames[1]),
		mustCreateTestProcess(machineNames[2]),
		mustCreateTestProcess(machineNames[3]),
	}

	conn, newErr := New()
	if newErr != nil {
		t.Fatal(newErr)
	}

	regErr := conn.RegisterMachine(machineNames[0], nil, "go-systemd", "container", leaders[0], "")
	if regErr != nil {
		t.Fatal(regErr)
	}

	regWithNetworkErr := conn.RegisterMachineWithNetwork(machineNames[1], nil, "go-systemd", "container", leaders[1], "", nil)
	if regWithNetworkErr != nil {
		t.Fatal(regWithNetworkErr)
	}

	createErr := conn.CreateMachine(machineNames[2], nil, "go-systemd", "container", leaders[2], "", nil)
	if createErr != nil {
		t.Fatal(createErr)
	}

	createWithNetworkErr := conn.CreateMachineWithNetwork(machineNames[3], nil, "go-systemd", "container", leaders[3], "", nil, nil)
	if createWithNetworkErr != nil {
		t.Fatal(createWithNetworkErr)
	}

	machines := make([]dbus.ObjectPath, 0)
	for _, v := range machineNames {
		machine, getErr := conn.GetMachine(v)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if machine != "" {
			machines = append(machines, machine)
		}
	}
	if len(machines) != 4 {
		t.Fatalf("did not find all machine nameds %s", machineNames)
	}

	listMachines, getErr := conn.ListMachines()
	if getErr != nil {
		t.Fatal(getErr)
	}

	// listMachines includes also `.host`, so by default the length should be greater than 2
	if len(listMachines) <= 4 {
		t.Fatalf("did not find any machine")
	}

	for _, v := range machineNames {
		tErr := conn.TerminateMachine(v)
		if tErr != nil {
			t.Fatal(tErr)
		}
		var machine dbus.ObjectPath
		for i := 1; i <= 10; i++ {
			machine, getErr = conn.GetMachine(v)
			if len(machine) == 0 && getErr != nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
		if len(machine) != 0 {
			t.Fatalf("unexpectedly found machine named %s", v)
		} else if getErr == nil {
			t.Fatal("expected error but got nil")
		}
	}
}

func generateRandomLabel(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	s := make([]rune, n)
	rand.Seed(time.Now().UnixNano())
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func TestImages(t *testing.T) {
	imageName := machinePrefix + generateRandomLabel(8)
	imagePath := filepath.Join("/var/lib/machines", imageName)

	if _, err := os.Create(imagePath); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(imagePath)

	if err := os.Truncate(imagePath, 500*1024*1024); err != nil {
		t.Fatal(err)
	}

	conn, newErr := New()
	if newErr != nil {
		t.Fatal(newErr)
	}

	listImages, listErr := conn.ListImages()
	if listErr != nil {
		t.Fatal(listErr)
	}

	if len(listImages) < 1 {
		t.Fatalf("did not find any image")
	}
}
