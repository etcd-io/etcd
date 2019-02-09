// Copyright 2015 Daniel Theophanes.
// Use of this source code is governed by a zlib-style
// license that can be found in the LICENSE file.

// Simple service that only works by printing a log message every few seconds.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kardianos/osext"
	"github.com/kardianos/service"
)

// Config is the runner app config structure.
type Config struct {
	Name, DisplayName, Description string

	Dir  string
	Exec string
	Args []string
	Env  []string

	Stderr, Stdout string
}

var logger service.Logger

type program struct {
	exit    chan struct{}
	service service.Service

	*Config

	cmd *exec.Cmd
}

func (p *program) Start(s service.Service) error {
	// Look for exec.
	// Verify home directory.
	fullExec, err := exec.LookPath(p.Exec)
	if err != nil {
		return fmt.Errorf("Failed to find executable %q: %v", p.Exec, err)
	}

	p.cmd = exec.Command(fullExec, p.Args...)
	p.cmd.Dir = p.Dir
	p.cmd.Env = append(os.Environ(), p.Env...)

	go p.run()
	return nil
}
func (p *program) run() {
	logger.Info("Starting ", p.DisplayName)
	defer func() {
		if service.Interactive() {
			p.Stop(p.service)
		} else {
			p.service.Stop()
		}
	}()

	if p.Stderr != "" {
		f, err := os.OpenFile(p.Stderr, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0777)
		if err != nil {
			logger.Warningf("Failed to open std err %q: %v", p.Stderr, err)
			return
		}
		defer f.Close()
		p.cmd.Stderr = f
	}
	if p.Stdout != "" {
		f, err := os.OpenFile(p.Stdout, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0777)
		if err != nil {
			logger.Warningf("Failed to open std out %q: %v", p.Stdout, err)
			return
		}
		defer f.Close()
		p.cmd.Stdout = f
	}

	err := p.cmd.Run()
	if err != nil {
		logger.Warningf("Error running: %v", err)
	}

	return
}
func (p *program) Stop(s service.Service) error {
	close(p.exit)
	logger.Info("Stopping ", p.DisplayName)
	if p.cmd.ProcessState.Exited() == false {
		p.cmd.Process.Kill()
	}
	if service.Interactive() {
		os.Exit(0)
	}
	return nil
}

func getConfigPath() (string, error) {
	fullexecpath, err := osext.Executable()
	if err != nil {
		return "", err
	}

	dir, execname := filepath.Split(fullexecpath)
	ext := filepath.Ext(execname)
	name := execname[:len(execname)-len(ext)]

	return filepath.Join(dir, name+".json"), nil
}

func getConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	conf := &Config{}

	r := json.NewDecoder(f)
	err = r.Decode(&conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func main() {
	svcFlag := flag.String("service", "", "Control the system service.")
	flag.Parse()

	configPath, err := getConfigPath()
	if err != nil {
		log.Fatal(err)
	}
	config, err := getConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	svcConfig := &service.Config{
		Name:        config.Name,
		DisplayName: config.DisplayName,
		Description: config.Description,
	}

	prg := &program{
		exit: make(chan struct{}),

		Config: config,
	}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	prg.service = s

	errs := make(chan error, 5)
	logger, err = s.Logger(errs)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			err := <-errs
			if err != nil {
				log.Print(err)
			}
		}
	}()

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
