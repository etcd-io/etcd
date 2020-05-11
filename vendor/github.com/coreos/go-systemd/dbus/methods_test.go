// Copyright 2015, 2018 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbus

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus"
)

type TrUnitProp struct {
	name  string
	props []Property
}

func setupConn(t *testing.T) *Conn {
	conn, err := New()
	if err != nil {
		t.Fatal(err)
	}

	return conn
}

func findFixture(target string, t *testing.T) string {
	abs, err := filepath.Abs("../fixtures/" + target)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func setupUnit(target string, conn *Conn, t *testing.T) {
	// Blindly stop the unit in case it is running
	conn.StopUnit(target, "replace", nil)

	// Blindly remove the symlink in case it exists
	targetRun := filepath.Join("/run/systemd/system/", target)
	os.Remove(targetRun)
}

func linkUnit(target string, conn *Conn, t *testing.T) {
	abs := findFixture(target, t)
	fixture := []string{abs}

	changes, err := conn.LinkUnitFiles(fixture, true, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(changes) < 1 {
		t.Fatalf("Expected one change, got %v", changes)
	}

	runPath := filepath.Join("/run/systemd/system/", target)
	if changes[0].Filename != runPath {
		t.Fatal("Unexpected target filename")
	}
}

func getUnitStatus(units []UnitStatus, name string) *UnitStatus {
	for _, u := range units {
		if u.Name == name {
			return &u
		}
	}
	return nil
}

func getUnitStatusSingle(conn *Conn, name string) *UnitStatus {
	units, err := conn.ListUnits()
	if err != nil {
		return nil
	}
	return getUnitStatus(units, name)
}

func getUnitFile(units []UnitFile, name string) *UnitFile {
	for _, u := range units {
		if path.Base(u.Path) == name {
			return &u
		}
	}
	return nil
}

func runStartTrUnit(t *testing.T, conn *Conn, trTarget TrUnitProp) error {
	reschan := make(chan string)
	_, err := conn.StartTransientUnit(trTarget.name, "replace", trTarget.props, reschan)
	if err != nil {
		return err
	}

	job := <-reschan
	if job != "done" {
		return fmt.Errorf("Job is not done: %s", job)
	}

	return nil
}

func runStopUnit(t *testing.T, conn *Conn, trTarget TrUnitProp) error {
	reschan := make(chan string)
	_, err := conn.StopUnit(trTarget.name, "replace", reschan)
	if err != nil {
		return err
	}

	// wait for StopUnit job to complete
	<-reschan

	return nil
}

// Ensure that basic unit starting and stopping works.
func TestStartStopUnit(t *testing.T) {
	target := "start-stop.service"
	conn := setupConn(t)
	defer conn.Close()

	setupUnit(target, conn, t)
	linkUnit(target, conn, t)

	// 2. Start the unit
	reschan := make(chan string)
	_, err := conn.StartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job := <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	units, err := conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit := getUnitStatus(units, target)

	if unit == nil {
		t.Fatalf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		t.Fatalf("Test unit not active")
	}

	// 3. Stop the unit
	_, err = conn.StopUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	// wait for StopUnit job to complete
	<-reschan

	units, err = conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit = getUnitStatus(units, target)

	if unit != nil {
		t.Fatalf("Test unit found in list, should be stopped")
	}
}

// Ensure that basic unit restarting works.
func TestRestartUnit(t *testing.T) {
	target := "start-stop.service"
	conn := setupConn(t)
	defer conn.Close()

	setupUnit(target, conn, t)
	linkUnit(target, conn, t)

	// Start the unit
	reschan := make(chan string)
	_, err := conn.StartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job := <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	units, err := conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit := getUnitStatus(units, target)
	if unit == nil {
		t.Fatalf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		t.Fatalf("Test unit not active")
	}

	// Restart the unit
	reschan = make(chan string)
	_, err = conn.RestartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job = <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	// Stop the unit
	_, err = conn.StopUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	// wait for StopUnit job to complete
	<-reschan

	units, err = conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit = getUnitStatus(units, target)
	if unit != nil {
		t.Fatalf("Test unit found in list, should be stopped")
	}

	// Try to restart the unit.
	// It should still succeed, even if the unit is inactive.
	reschan = make(chan string)
	_, err = conn.TryRestartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	// wait for StopUnit job to complete
	<-reschan

	units, err = conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit = getUnitStatus(units, target)
	if unit != nil {
		t.Fatalf("Test unit found in list, should be stopped")
	}
}

// Ensure that basic unit reloading works.
func TestReloadUnit(t *testing.T) {
	target := "reload.service"
	conn := setupConn(t)
	defer conn.Close()

	err := conn.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	subSet := conn.NewSubscriptionSet()
	evChan, errChan := subSet.Subscribe()

	subSet.Add(target)

	setupUnit(target, conn, t)
	linkUnit(target, conn, t)

	// Start the unit
	reschan := make(chan string)
	_, err = conn.StartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job := <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	units, err := conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit := getUnitStatus(units, target)
	if unit == nil {
		t.Fatalf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		t.Fatalf("Test unit not active")
	}

	// Reload the unit
	reschan = make(chan string)

	_, err = conn.ReloadUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job = <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(3 * time.Second)
		close(timeout)
	}()

	// Wait for the event, expecting the target UnitStatus meets all of the
	// following conditions:
	//  * target is non-nil
	//  * target's ActiveState is active.
waitevent:
	for {
		select {
		case changes := <-evChan:
			tch, ok := changes[target]
			if !ok {
				continue waitevent
			}
			if tch != nil && tch.Name == target && tch.ActiveState == "active" {
				break waitevent
			}
		case err = <-errChan:
			t.Fatal(err)
		case <-timeout:
			t.Fatal("Reached timeout")
		}
	}
}

// Ensure that basic unit reload-or-restarting works.
func TestReloadOrRestartUnit(t *testing.T) {
	target := "reload.service"
	conn := setupConn(t)
	defer conn.Close()

	setupUnit(target, conn, t)
	linkUnit(target, conn, t)

	// Start the unit
	reschan := make(chan string)
	_, err := conn.StartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job := <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	units, err := conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit := getUnitStatus(units, target)
	if unit == nil {
		t.Fatalf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		t.Fatalf("Test unit not active")
	}

	// Reload or restart the unit
	reschan = make(chan string)
	_, err = conn.ReloadOrRestartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job = <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	// Stop the unit
	_, err = conn.StopUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	// wait for StopUnit job to complete
	<-reschan

	units, err = conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit = getUnitStatus(units, target)
	if unit != nil && unit.ActiveState == "active" {
		t.Fatalf("Test unit still active, should be inactive.")
	}

	// Reload or try to restart the unit
	// It should still succeed, even if the unit is inactive.
	reschan = make(chan string)
	_, err = conn.ReloadOrTryRestartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job = <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}
}

// Ensure that ListUnitsByNames works.
func TestListUnitsByNames(t *testing.T) {
	target1 := "systemd-journald.service"
	target2 := "unexisting.service"

	conn := setupConn(t)
	defer conn.Close()

	units, err := conn.ListUnitsByNames([]string{target1, target2})

	if err != nil {
		t.Skip(err)
	}

	unit := getUnitStatus(units, target1)

	if unit == nil {
		t.Fatalf("%s unit not found in list", target1)
	} else if unit.ActiveState != "active" {
		t.Fatalf("%s unit should be active but it is %s", target1, unit.ActiveState)
	}

	unit = getUnitStatus(units, target2)

	if unit == nil {
		t.Fatalf("Unexisting test unit not found in list")
	} else if unit.ActiveState != "inactive" {
		t.Fatalf("Test unit should be inactive")
	}
}

// Ensure that ListUnitsByPatterns works.
func TestListUnitsByPatterns(t *testing.T) {
	target1 := "systemd-journald.service"
	target2 := "unexisting.service"

	conn := setupConn(t)
	defer conn.Close()

	units, err := conn.ListUnitsByPatterns([]string{}, []string{"systemd-journald*", target2})

	if err != nil {
		t.Skip(err)
	}

	unit := getUnitStatus(units, target1)

	if unit == nil {
		t.Fatalf("%s unit not found in list", target1)
	} else if unit.ActiveState != "active" {
		t.Fatalf("Test unit should be active")
	}

	unit = getUnitStatus(units, target2)

	if unit != nil {
		t.Fatalf("Unexisting test unit found in list")
	}
}

// Ensure that ListUnitsFiltered works.
func TestListUnitsFiltered(t *testing.T) {
	target := "systemd-journald.service"

	conn := setupConn(t)
	defer conn.Close()

	units, err := conn.ListUnitsFiltered([]string{"active"})

	if err != nil {
		t.Fatal(err)
	}

	unit := getUnitStatus(units, target)

	if unit == nil {
		t.Fatalf("%s unit not found in list", target)
	} else if unit.ActiveState != "active" {
		t.Fatalf("Test unit should be active")
	}

	units, err = conn.ListUnitsFiltered([]string{"inactive"})

	if err != nil {
		t.Fatal(err)
	}

	unit = getUnitStatus(units, target)

	if unit != nil {
		t.Fatalf("Inactive unit should not be found in list")
	}
}

// Ensure that ListUnitFilesByPatterns works.
func TestListUnitFilesByPatterns(t *testing.T) {
	target1 := "systemd-journald.service"
	target2 := "exit.target"

	conn := setupConn(t)
	defer conn.Close()

	units, err := conn.ListUnitFilesByPatterns([]string{"static"}, []string{"systemd-journald*", target2})

	if err != nil {
		t.Skip(err)
	}

	unit := getUnitFile(units, target1)

	if unit == nil {
		t.Fatalf("%s unit not found in list", target1)
	} else if unit.Type != "static" {
		t.Fatalf("Test unit file should be static")
	}

	units, err = conn.ListUnitFilesByPatterns([]string{"disabled"}, []string{"systemd-journald*", target2})

	if err != nil {
		t.Fatal(err)
	}

	unit = getUnitFile(units, target2)

	if unit == nil {
		t.Fatalf("%s unit not found in list", target2)
	} else if unit.Type != "disabled" {
		t.Fatalf("%s unit file should be disabled", target2)
	}
}

func TestListUnitFiles(t *testing.T) {
	target1 := "systemd-journald.service"
	target2 := "exit.target"

	conn := setupConn(t)
	defer conn.Close()

	units, err := conn.ListUnitFiles()

	if err != nil {
		t.Fatal(err)
	}

	unit := getUnitFile(units, target1)

	if unit == nil {
		t.Fatalf("%s unit not found in list", target1)
	} else if unit.Type != "static" {
		t.Fatalf("Test unit file should be static")
	}

	unit = getUnitFile(units, target2)

	if unit == nil {
		t.Fatalf("%s unit not found in list", target2)
	} else if unit.Type != "disabled" {
		t.Fatalf("%s unit file should be disabled", target2)
	}
}

// Enables a unit and then immediately tears it down
func TestEnableDisableUnit(t *testing.T) {
	target := "enable-disable.service"
	conn := setupConn(t)
	defer conn.Close()

	setupUnit(target, conn, t)
	abs := findFixture(target, t)
	runPath := filepath.Join("/run/systemd/system/", target)

	// 1. Enable the unit
	install, changes, err := conn.EnableUnitFiles([]string{abs}, true, true)
	if err != nil {
		t.Fatal(err)
	}

	if install {
		t.Log("Install was true")
	}

	if len(changes) < 1 {
		t.Fatalf("Expected one change, got %v", changes)
	}

	if changes[0].Filename != runPath {
		t.Fatal("Unexpected target filename")
	}

	// 2. Disable the unit
	dChanges, err := conn.DisableUnitFiles([]string{target}, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(dChanges) != 1 {
		t.Fatalf("Changes should include the path, %v", dChanges)
	}
	if dChanges[0].Filename != runPath {
		t.Fatalf("Change should include correct filename, %+v", dChanges[0])
	}
	if dChanges[0].Destination != "" {
		t.Fatalf("Change destination should be empty, %+v", dChanges[0])
	}
}

// TestSystemState tests if system state is one of the valid states
func TestSystemState(t *testing.T) {
	conn := setupConn(t)
	defer conn.Close()

	prop, err := conn.SystemState()
	if err != nil {
		t.Fatal(err)
	}

	if prop.Name != "SystemState" {
		t.Fatalf("unexpected property name: %v", prop.Name)
	}

	val := prop.Value.Value().(string)

	switch val {
	case "initializing":
	case "starting":
	case "running":
	case "degraded":
	case "maintenance":
	case "stopping":
	case "offline":
	case "unknown":
		// valid systemd state - do nothing

	default:
		t.Fatalf("unexpected property value: %v", val)
	}
}

// TestGetUnitProperties reads the `-.mount` which should exist on all systemd
// systems and ensures that one of its properties is valid.
func TestGetUnitProperties(t *testing.T) {
	conn := setupConn(t)
	defer conn.Close()

	unit := "-.mount"

	info, err := conn.GetUnitProperties(unit)
	if err != nil {
		t.Fatal(err)
	}

	desc, _ := info["Description"].(string)

	prop, err := conn.GetUnitProperty(unit, "Description")
	if err != nil {
		t.Fatal(err)
	}

	if prop.Name != "Description" {
		t.Fatal("unexpected property name")
	}

	val := prop.Value.Value().(string)
	if !reflect.DeepEqual(val, desc) {
		t.Fatal("unexpected property value")
	}
}

// TestGetUnitPropertiesRejectsInvalidName attempts to get the properties for a
// unit with an invalid name. This test should be run with --test.timeout set,
// as a fail will manifest as GetUnitProperties hanging indefinitely.
func TestGetUnitPropertiesRejectsInvalidName(t *testing.T) {
	conn := setupConn(t)
	defer conn.Close()

	unit := "//invalid#$^/"

	_, err := conn.GetUnitProperties(unit)
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}

	_, err = conn.GetUnitProperty(unit, "Wants")
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

// TestGetServiceProperty reads the `systemd-udevd.service` which should exist
// on all systemd systems and ensures that one of its property is valid.
func TestGetServiceProperty(t *testing.T) {
	conn := setupConn(t)
	defer conn.Close()

	service := "systemd-udevd.service"

	prop, err := conn.GetServiceProperty(service, "Type")
	if err != nil {
		t.Fatal(err)
	}

	if prop.Name != "Type" {
		t.Fatal("unexpected property name")
	}

	if _, ok := prop.Value.Value().(string); !ok {
		t.Fatal("invalid property value")
	}
}

// TestSetUnitProperties changes a cgroup setting on the `-.mount`
// which should exist on all systemd systems and ensures that the
// property was set.
func TestSetUnitProperties(t *testing.T) {
	conn := setupConn(t)
	defer conn.Close()

	unit := "-.mount"

	if err := conn.SetUnitProperties(unit, true, Property{"CPUShares", dbus.MakeVariant(uint64(1023))}); err != nil {
		t.Fatal(err)
	}

	info, err := conn.GetUnitTypeProperties(unit, "Mount")
	if err != nil {
		t.Fatal(err)
	}

	value, _ := info["CPUShares"].(uint64)
	if value != 1023 {
		t.Fatal("CPUShares of unit is not 1023:", value)
	}
}

// Ensure that oneshot transient unit starting and stopping works.
func TestStartStopTransientUnitAll(t *testing.T) {
	testCases := []struct {
		trTarget  TrUnitProp
		trDep     TrUnitProp
		checkFunc func(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error
	}{
		{
			trTarget: TrUnitProp{
				name: "testing-transient.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
				},
			},
			trDep:     TrUnitProp{"", nil},
			checkFunc: checkTransientUnit,
		},
		{
			trTarget: TrUnitProp{
				name: "testing-transient-oneshot.service",
				props: []Property{
					PropExecStart([]string{"/bin/true"}, false),
					PropType("oneshot"),
					PropRemainAfterExit(true),
				},
			},
			trDep:     TrUnitProp{"", nil},
			checkFunc: checkTransientUnitOneshot,
		},
		{
			trTarget: TrUnitProp{
				name: "testing-transient-requires.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
					PropRequires("testing-transient-requiresdep.service"),
				},
			},
			trDep: TrUnitProp{
				name: "testing-transient-requiresdep.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
				},
			},
			checkFunc: checkTransientUnitRequires,
		},
		{
			trTarget: TrUnitProp{
				name: "testing-transient-requires-ov.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
					PropRequires("testing-transient-requiresdep-ov.service"),
				},
			},
			trDep: TrUnitProp{
				name: "testing-transient-requiresdep-ov.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
				},
			},
			checkFunc: checkTransientUnitRequiresOv,
		},
		{
			trTarget: TrUnitProp{
				name: "testing-transient-requisite.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
					PropRequisite("testing-transient-requisitedep.service"),
				},
			},
			trDep: TrUnitProp{
				name: "testing-transient-requisitedep.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
				},
			},
			checkFunc: checkTransientUnitRequisite,
		},
		{
			trTarget: TrUnitProp{
				name: "testing-transient-requisite-ov.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
					PropRequisiteOverridable("testing-transient-requisitedep-ov.service"),
				},
			},
			trDep: TrUnitProp{
				name: "testing-transient-requisitedep-ov.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
				},
			},
			checkFunc: checkTransientUnitRequisiteOv,
		},
		{
			trTarget: TrUnitProp{
				name: "testing-transient-wants.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
					PropWants("testing-transient-wantsdep.service"),
				},
			},
			trDep: TrUnitProp{
				name: "testing-transient-wantsdep.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
				},
			},
			checkFunc: checkTransientUnitWants,
		},
		{
			trTarget: TrUnitProp{
				name: "testing-transient-bindsto.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
					PropBindsTo("testing-transient-bindstodep.service"),
				},
			},
			trDep: TrUnitProp{
				name: "testing-transient-bindstodep.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
				},
			},
			checkFunc: checkTransientUnitBindsTo,
		},
		{
			trTarget: TrUnitProp{
				name: "testing-transient-conflicts.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
					PropConflicts("testing-transient-conflictsdep.service"),
				},
			},
			trDep: TrUnitProp{
				name: "testing-transient-conflictsdep.service",
				props: []Property{
					PropExecStart([]string{"/bin/sleep", "400"}, false),
				},
			},
			checkFunc: checkTransientUnitConflicts,
		},
	}

	for i, tt := range testCases {
		if err := tt.checkFunc(t, tt.trTarget, tt.trDep); err != nil {
			t.Errorf("case %d: failed test with unit %s. err: %v", i, tt.trTarget.name, err)
		}
	}
}

// Ensure that basic transient unit starting and stopping works.
func checkTransientUnit(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the unit
	err := runStartTrUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit == nil {
		return fmt.Errorf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		return fmt.Errorf("Test unit not active")
	}

	// Stop the unit
	err = runStopUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit = getUnitStatusSingle(conn, trTarget.name)
	if unit != nil {
		return fmt.Errorf("Test unit found in list, should be stopped")
	}

	return nil
}

func checkTransientUnitOneshot(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the unit
	err := runStartTrUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit == nil {
		return fmt.Errorf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		return fmt.Errorf("Test unit not active")
	}

	// Stop the unit
	err = runStopUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit = getUnitStatusSingle(conn, trTarget.name)
	if unit != nil {
		return fmt.Errorf("Test unit found in list, should be stopped")
	}

	return nil
}

// Ensure that transient unit with Requires starting and stopping works.
func checkTransientUnitRequires(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the dependent unit
	err := runStartTrUnit(t, conn, trDep)
	if err != nil {
		return err
	}

	// Start the target unit
	err = runStartTrUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit == nil {
		return fmt.Errorf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		return fmt.Errorf("Test unit not active")
	}

	// Stop the unit
	err = runStopUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit = getUnitStatusSingle(conn, trTarget.name)
	if unit != nil {
		return fmt.Errorf("Test unit found in list, should be stopped")
	}

	// Stop the dependent unit
	err = runStopUnit(t, conn, trDep)
	if err != nil {
		return err
	}

	unit = getUnitStatusSingle(conn, trDep.name)
	if unit != nil {
		return fmt.Errorf("Test unit found in list, should be stopped")
	}

	return nil
}

// Ensure that transient unit with RequiresOverridable starting and stopping works.
func checkTransientUnitRequiresOv(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the dependent unit
	err := runStartTrUnit(t, conn, trDep)
	if err != nil {
		return err
	}

	// Start the target unit
	err = runStartTrUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit == nil {
		return fmt.Errorf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		return fmt.Errorf("Test unit not active")
	}

	// Stop the unit
	err = runStopUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit = getUnitStatusSingle(conn, trTarget.name)
	if unit != nil {
		return fmt.Errorf("Test unit found in list, should be stopped")
	}

	// Stop the dependent unit
	err = runStopUnit(t, conn, trDep)
	if err != nil {
		return err
	}

	unit = getUnitStatusSingle(conn, trDep.name)
	if unit != nil {
		return fmt.Errorf("Test unit found in list, should be stopped")
	}

	return nil
}

// Ensure that transient unit with Requisite starting and stopping works.
// It's expected for target unit to fail, as its child is not started at all.
func checkTransientUnitRequisite(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the target unit
	err := runStartTrUnit(t, conn, trTarget)
	if err == nil {
		return fmt.Errorf("Unit %s is expected to fail, but succeeded", trTarget.name)
	}

	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit != nil && unit.ActiveState == "active" {
		return fmt.Errorf("Test unit %s is active, should be inactive", trTarget.name)
	}

	return nil
}

// Ensure that transient unit with RequisiteOverridable starting and stopping works.
// It's expected for target unit to fail, as its child is not started at all.
func checkTransientUnitRequisiteOv(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the target unit
	err := runStartTrUnit(t, conn, trTarget)
	if err == nil {
		return fmt.Errorf("Unit %s is expected to fail, but succeeded", trTarget.name)
	}

	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit != nil && unit.ActiveState == "active" {
		return fmt.Errorf("Test unit %s is active, should be inactive", trTarget.name)
	}

	return nil
}

// Ensure that transient unit with Wants starting and stopping works.
// It's expected for target to successfully start, even when its child is not started.
func checkTransientUnitWants(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the target unit
	err := runStartTrUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit == nil {
		return fmt.Errorf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		return fmt.Errorf("Test unit not active")
	}

	// Stop the unit
	err = runStopUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit = getUnitStatusSingle(conn, trTarget.name)
	if unit != nil {
		return fmt.Errorf("Test unit found in list, should be stopped")
	}

	return nil
}

// Ensure that transient unit with BindsTo starting and stopping works.
// Stopping its child should result in stopping target unit.
func checkTransientUnitBindsTo(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the dependent unit
	err := runStartTrUnit(t, conn, trDep)
	if err != nil {
		return err
	}

	// Start the target unit
	err = runStartTrUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit == nil {
		t.Fatalf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		t.Fatalf("Test unit not active")
	}

	// Stop the dependent unit
	err = runStopUnit(t, conn, trDep)
	if err != nil {
		return err
	}

	unit = getUnitStatusSingle(conn, trDep.name)
	if unit != nil {
		t.Fatalf("Test unit found in list, should be stopped")
	}

	// Then the target unit should be gone
	unit = getUnitStatusSingle(conn, trTarget.name)
	if unit != nil {
		t.Fatalf("Test unit found in list, should be stopped")
	}

	return nil
}

// Ensure that transient unit with Conflicts starting and stopping works.
func checkTransientUnitConflicts(t *testing.T, trTarget TrUnitProp, trDep TrUnitProp) error {
	conn := setupConn(t)
	defer conn.Close()

	// Start the dependent unit
	err := runStartTrUnit(t, conn, trDep)
	if err != nil {
		return err
	}

	// Start the target unit
	err = runStartTrUnit(t, conn, trTarget)
	if err != nil {
		return err
	}

	isTargetActive := false
	unit := getUnitStatusSingle(conn, trTarget.name)
	if unit != nil && unit.ActiveState == "active" {
		isTargetActive = true
	}

	isReqDepActive := false
	unit = getUnitStatusSingle(conn, trDep.name)
	if unit != nil && unit.ActiveState == "active" {
		isReqDepActive = true
	}

	if isTargetActive && isReqDepActive {
		return fmt.Errorf("Conflicts didn't take place")
	}

	// Stop the target unit
	if isTargetActive {
		err = runStopUnit(t, conn, trTarget)
		if err != nil {
			return err
		}

		unit = getUnitStatusSingle(conn, trTarget.name)
		if unit != nil {
			return fmt.Errorf("Test unit %s found in list, should be stopped", trTarget.name)
		}
	}

	// Stop the dependent unit
	if isReqDepActive {
		err = runStopUnit(t, conn, trDep)
		if err != nil {
			return err
		}

		unit = getUnitStatusSingle(conn, trDep.name)
		if unit != nil {
			return fmt.Errorf("Test unit %s found in list, should be stopped", trDep.name)
		}
	}

	return nil
}

// Ensure that putting running programs into scopes works
func TestStartStopTransientScope(t *testing.T) {
	conn := setupConn(t)
	defer conn.Close()

	cmd := exec.Command("/bin/sleep", "400")
	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()

	props := []Property{
		PropPids(uint32(cmd.Process.Pid)),
	}
	target := fmt.Sprintf("testing-transient-%d.scope", cmd.Process.Pid)

	// Start the unit
	reschan := make(chan string)
	_, err = conn.StartTransientUnit(target, "replace", props, reschan)
	if err != nil {
		t.Fatal(err)
	}

	job := <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	units, err := conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit := getUnitStatus(units, target)

	if unit == nil {
		t.Fatalf("Test unit not found in list")
	} else if unit.ActiveState != "active" {
		t.Fatalf("Test unit not active")
	}

	// maybe check if pid is really a member of the just created scope
	//   systemd uses the following api which does not use dbus, but directly
	//   accesses procfs for cgroup information.
	//     int sd_pid_get_unit(pid_t pid, char **session)
}

// Ensure that basic unit gets killed by SIGTERM
func TestKillUnit(t *testing.T) {
	target := "start-stop.service"
	conn := setupConn(t)
	defer conn.Close()

	err := conn.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	subSet := conn.NewSubscriptionSet()
	evChan, errChan := subSet.Subscribe()

	subSet.Add(target)

	setupUnit(target, conn, t)
	linkUnit(target, conn, t)

	// Start the unit
	reschan := make(chan string)
	_, err = conn.StartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job := <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	// send SIGTERM
	conn.KillUnit(target, int32(syscall.SIGTERM))

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(3 * time.Second)
		close(timeout)
	}()

	// Wait for the event, expecting the target UnitStatus meets one of the
	// following conditions:
	//  * target is nil, meaning the unit has completely gone.
	//  * target is non-nil, and its ActiveState is not active.
waitevent:
	for {
		select {
		case changes := <-evChan:
			tch, ok := changes[target]
			if !ok {
				continue waitevent
			}
			if tch == nil || (tch != nil && tch.Name == target && tch.ActiveState != "active") {
				break waitevent
			}
		case err = <-errChan:
			t.Fatal(err)
		case <-timeout:
			t.Fatal("Reached timeout")
		}
	}
}

// Ensure that a failed unit gets reset
func TestResetFailedUnit(t *testing.T) {
	target := "start-failed.service"
	conn := setupConn(t)
	defer conn.Close()

	setupUnit(target, conn, t)
	linkUnit(target, conn, t)

	// Start the unit
	reschan := make(chan string)
	_, err := conn.StartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	job := <-reschan
	if job != "failed" {
		t.Fatal("Job is not failed:", job)
	}

	units, err := conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	unit := getUnitStatus(units, target)
	if unit == nil {
		t.Fatalf("Test unit not found in list")
	}

	// reset the failed unit
	err = conn.ResetFailedUnit(target)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure that the target unit is actually gone
	units, err = conn.ListUnits()
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, u := range units {
		if u.Name == target {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("Test unit still found in list. units = %v", units)
	}
}

func TestConnJobListener(t *testing.T) {
	target := "start-stop.service"
	conn := setupConn(t)
	defer conn.Close()

	setupUnit(target, conn, t)
	linkUnit(target, conn, t)

	jobSize := len(conn.jobListener.jobs)

	reschan := make(chan string)
	_, err := conn.StartUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	<-reschan

	_, err = conn.StopUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	<-reschan

	currentJobSize := len(conn.jobListener.jobs)
	if jobSize != currentJobSize {
		t.Fatal("JobListener jobs leaked")
	}
}

// Enables a unit and then masks/unmasks it
func TestMaskUnmask(t *testing.T) {
	target := "mask-unmask.service"
	conn := setupConn(t)
	defer conn.Close()

	setupUnit(target, conn, t)
	abs := findFixture(target, t)
	runPath := filepath.Join("/run/systemd/system/", target)

	// 1. Enable the unit
	install, changes, err := conn.EnableUnitFiles([]string{abs}, true, true)
	if err != nil {
		t.Fatal(err)
	}

	if install {
		t.Log("Install was true")
	}

	if len(changes) < 1 {
		t.Fatalf("Expected one change, got %v", changes)
	}

	if changes[0].Filename != runPath {
		t.Fatal("Unexpected target filename")
	}

	// 2. Mask the unit
	mChanges, err := conn.MaskUnitFiles([]string{target}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if mChanges[0].Filename != runPath {
		t.Fatalf("Change should include correct filename, %+v", mChanges[0])
	}
	if mChanges[0].Destination != "" {
		t.Fatalf("Change destination should be empty, %+v", mChanges[0])
	}

	// 3. Unmask the unit
	uChanges, err := conn.UnmaskUnitFiles([]string{target}, true)
	if err != nil {
		t.Fatal(err)
	}
	if uChanges[0].Filename != runPath {
		t.Fatalf("Change should include correct filename, %+v", uChanges[0])
	}
	if uChanges[0].Destination != "" {
		t.Fatalf("Change destination should be empty, %+v", uChanges[0])
	}

}

// Test a global Reload
func TestReload(t *testing.T) {
	conn := setupConn(t)
	defer conn.Close()

	err := conn.Reload()
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnitName(t *testing.T) {
	for _, unit := range []string{
		"",
		"foo.service",
		"foobar",
		"woof@woof.service",
		"0123456",
		"account_db.service",
		"got-dashes",
	} {
		got := unitName(unitPath(unit))
		if got != unit {
			t.Errorf("bad result for unitName(%s): got %q, want %q", unit, got, unit)
		}
	}
}
