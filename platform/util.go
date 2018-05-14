// Copyright 2015 CoreOS, Inc.
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

package platform

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// Manhole connects os.Stdin, os.Stdout, and os.Stderr to an interactive shell
// session on the Machine m. Manhole blocks until the shell session has ended.
// If os.Stdin does not refer to a TTY, Manhole returns immediately with a nil
// error.
func Manhole(m Machine) error {
	fd := int(os.Stdin.Fd())
	if !terminal.IsTerminal(fd) {
		return nil
	}

	tstate, _ := terminal.MakeRaw(fd)
	defer terminal.Restore(fd, tstate)

	client, err := m.SSHClient()
	if err != nil {
		return fmt.Errorf("SSH client failed: %v", err)
	}

	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session failed: %v", err)
	}

	defer session.Close()

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	modes := ssh.TerminalModes{
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}

	cols, lines, err := terminal.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}

	if err = session.RequestPty(os.Getenv("TERM"), lines, cols, modes); err != nil {
		return fmt.Errorf("failed to request pseudo terminal: %s", err)
	}

	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %s", err)
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to wait for session: %s", err)
	}

	return nil
}

// Enable SELinux on a machine (skip on machines without SELinux support)
func EnableSelinux(m Machine) error {
	_, stderr, err := m.SSH("if type -P setenforce; then sudo setenforce 1; fi")
	if err != nil {
		return fmt.Errorf("Unable to enable SELinux: %s: %s", err, stderr)
	}
	return nil
}

// Reboots a machine, stopping ssh first.
// Afterwards run CheckMachine to verify the system is back and operational.
func StartReboot(m Machine) error {
	// stop sshd so that commonMachineChecks will only work if the machine
	// actually rebooted
	out, stderr, err := m.SSH("sudo systemctl stop sshd.socket && sudo reboot")
	if _, ok := err.(*ssh.ExitMissingError); ok {
		// A terminated session is perfectly normal during reboot.
		err = nil
	}
	if err != nil {
		return fmt.Errorf("issuing reboot command failed: %s: %s: %s", out, err, stderr)
	}
	return nil
}

// RebootMachine will reboot a given machine, provided the machine's journal.
func RebootMachine(m Machine, j *Journal) error {
	if err := StartReboot(m); err != nil {
		return fmt.Errorf("machine %q failed to begin rebooting: %v", m.ID(), err)
	}
	return StartMachine(m, j)
}

// StartMachine will start a given machine, provided the machine's journal.
func machineChecksPrint(header string, cmd string, stdout []byte, stderr []byte, err error) {
	fmt.Printf("## %s:\n# cmd\n%v\n\n# err\n%v\n\n# stdout\n%v\n\n# stderr\n%v\n\n",
		   header, cmd, err, string(stdout), string(stderr))
}

func machineChecks(m Machine) {
	var cmd string
	var stdout []byte
	var stderr []byte
	var err error

	if false {debug.PrintStack()}

	cmd = "lsblk --json"
	stdout, stderr, err = m.SSH(cmd)
	machineChecksPrint("machineChecks", cmd, stdout, stderr, err)

	cmd = `getenforce`
	stdout, stderr, err = m.SSH(cmd)
	machineChecksPrint("machineChecks", cmd, stdout, stderr, err)

	cmd = `sestatus`
	stdout, stderr, err = m.SSH(cmd)
	machineChecksPrint("machineChecks", cmd, stdout, stderr, err)

	//cmd = `sudo mount`
	//stdout, stderr, err = m.SSH(cmd)
	//machineChecksPrint("machineChecks", cmd, stdout, stderr, err)

	cmd = `sudo ls -lZ /etc`
	stdout, stderr, err = m.SSH(cmd)
	machineChecksPrint("machineChecks", cmd, stdout, stderr, err)

	//cmd = "systemctl status"
	//stdout, stderr, err = m.SSH(cmd)
	//machineChecksPrint("machineChecks", cmd, stdout, stderr, err)

	//cmd = `type -P setenforce`
	//stdout, stderr, err = m.SSH(cmd)
	//machineChecksPrint("machineChecks", cmd, stdout, stderr, err)

	cmd = `sudo setenforce 1`
	stdout, stderr, err = m.SSH(cmd)
	machineChecksPrint("machineChecks", cmd, stdout, stderr, err)

	cmd = `sestatus`
	stdout, stderr, err = m.SSH(cmd)
	machineChecksPrint("machineChecks", cmd, stdout, stderr, err)
}

func StartMachine(m Machine, j *Journal) error {
	if err := j.Start(context.TODO(), m); err != nil {
		return fmt.Errorf("machine %q failed to start: %v", m.ID(), err)
	}
	if err := CheckMachine(context.TODO(), m); err != nil {
		return fmt.Errorf("machine %q failed basic checks: %v", m.ID(), err)
	}
	machineChecks(m)
	if !m.RuntimeConf().NoEnableSelinux {
		if err := EnableSelinux(m); err != nil {
			return fmt.Errorf("machine %q failed to enable selinux: %v", m.ID(), err)
		}
	}
	return nil
}
