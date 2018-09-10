// Copyright 2017 CoreOS, Inc.
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

package misc

import (
	"encoding/json"
	"fmt"

	"github.com/coreos/mantle/kola/cluster"
	"github.com/coreos/mantle/kola/register"
	"github.com/coreos/mantle/platform"
	"github.com/coreos/mantle/platform/conf"
	"github.com/coreos/mantle/platform/machine/qemu"
)

var (
	raidRootUserData = conf.ContainerLinuxConfig(`storage:
  disks:
    - device: "/dev/disk/by-id/virtio-secondary"
      wipe_table: true
      partitions:
       - label: root1
         number: 1
         size: 256MiB
         type_guid: be9067b9-ea49-4f15-b4f6-f36f8c9e1818
       - label: root2
         number: 2
         size: 256MiB
         type_guid: be9067b9-ea49-4f15-b4f6-f36f8c9e1818
  raid:
    - name: "rootarray"
      level: "raid1"
      devices:
        - "/dev/disk/by-partlabel/root1"
        - "/dev/disk/by-partlabel/root2"
  filesystems:
    - name: "ROOT"
      mount:
        device: "/dev/md/rootarray"
        format: "ext4"
        label: ROOT
    - name: "NOT_ROOT"
      mount:
        device: "/dev/disk/by-id/virtio-primary-disk-part9"
        format: "ext4"
        label: wasteland
        wipe_filesystem: true`)
)

func init() {
	register.Register(&register.Test{
		// This test needs additional disks which is only supported on qemu since Ignition
		// does not support deleting partitions without wiping the partition table and the
		// disk doesn't have room for new partitions.
		// TODO(ajeddeloh): change this to delete partition 9 and replace it with 9 and 10
		// once Ignition supports it.
		Run:         RootOnRaid,
		ClusterSize: 0,
		Platforms:   []string{"qemu"},
		Name:        "coreos.disk.raid.root",
	})
	register.Register(&register.Test{
		Run:         DataOnRaid,
		ClusterSize: 1,
		Name:        "coreos.disk.raid.data",
		UserData: conf.ContainerLinuxConfig(`storage:
  raid:
    - name: "DATA"
      level: "raid1"
      devices:
        - "/dev/disk/by-partlabel/OEM-CONFIG"
        - "/dev/disk/by-partlabel/USR-B"
  filesystems:
    - name: "DATA"
      mount:
        device: "/dev/md/DATA"
        format: "ext4"
        label: DATA
systemd:
  units:
    - name: "var-lib-data.mount"
      enable: true
      contents: |
          [Mount]
          What=/dev/md/DATA
          Where=/var/lib/data
          Type=ext4
          
          [Install]
          WantedBy=local-fs.target`),
	})
}

func checksPrint(header string, cmd string, stdout []byte, stderr []byte, err error) {
	fmt.Printf("## %s:\n# cmd\n%v\n\n# err\n%v\n\n# stdout\n%v\n\n# stderr\n%v\n\n",
		header, cmd, err, string(stdout), string(stderr))
}

func selinuxChecks(m platform.Machine) {
	var cmd string
	var stdout []byte
	var stderr []byte
	var err error
	fname := "RootOnRaid"

	cmd = "mount"
	stdout, stderr, err = m.SSH(cmd)
	checksPrint(fname, cmd, stdout, stderr, err)

	cmd = `sestatus`
	stdout, stderr, err = m.SSH(cmd)
	checksPrint(fname, cmd, stdout, stderr, err)

	cmd = "getenforce"
	stdout, stderr, err = m.SSH(cmd)
	checksPrint(fname, cmd, stdout, stderr, err)

	cmd = "id; pwd"
	stdout, stderr, err = m.SSH(cmd)
	checksPrint(fname, cmd, stdout, stderr, err)

	cmd = "cat /proc/self/status"
	stdout, stderr, err = m.SSH(cmd)
	checksPrint(fname, cmd, stdout, stderr, err)

	cmd = "ls -lZ /"
	stdout, stderr, err = m.SSH(cmd)
	checksPrint(fname, cmd, stdout, stderr, err)

	cmd = "ls -lZ /root"
	stdout, stderr, err = m.SSH(cmd)
	checksPrint(fname+" (should fail, Permission denied)", cmd, stdout, stderr, err)

	cmd = "sudo ls -lZ /root"
	stdout, stderr, err = m.SSH(cmd)
	checksPrint(fname, cmd, stdout, stderr, err)
}

func RootOnRaid(c cluster.TestCluster) {
	options := qemu.MachineOptions{
		AdditionalDisks: []qemu.Disk{
			{Size: "520M", Serial: "secondary"},
		},
	}
	m, err := c.Cluster.(*qemu.Cluster).NewMachineWithOptions(raidRootUserData, options)
	if err != nil {
		c.Fatal(err)
	}

	selinuxChecks(m)

	checkIfMountpointIsRaid(c, m, "/")

	// reboot it to make sure it comes up again
	err = m.Reboot()
	if err != nil {
		c.Fatalf("could not reboot machine: %v", err)
	}

	checkIfMountpointIsRaid(c, m, "/")
}

func DataOnRaid(c cluster.TestCluster) {
	m := c.Machines()[0]

	checkIfMountpointIsRaid(c, m, "/var/lib/data")

	// reboot it to make sure it comes up again
	err := m.Reboot()
	if err != nil {
		c.Fatalf("could not reboot machine: %v", err)
	}

	checkIfMountpointIsRaid(c, m, "/var/lib/data")
}

type lsblkOutput struct {
	Blockdevices []blockdevice `json:"blockdevices"`
}

type blockdevice struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Mountpoint *string       `json:"mountpoint"`
	Children   []blockdevice `json:"children"`
}

// checkIfMountpointIsRaid will check if a given machine has a device of type
// raid1 mounted at the given mountpoint. If it does not, the test is failed.
func checkIfMountpointIsRaid(c cluster.TestCluster, m platform.Machine, mountpoint string) {
	output := c.MustSSH(m, "lsblk --json")

	l := lsblkOutput{}
	err := json.Unmarshal(output, &l)
	if err != nil {
		c.Fatalf("couldn't unmarshal lsblk output: %v", err)
	}

	foundRoot := checkIfMountpointIsRaidWalker(c, l.Blockdevices, mountpoint)
	if !foundRoot {
		c.Fatalf("didn't find root mountpoint in lsblk output")
	}
}

// checkIfMountpointIsRaidWalker will iterate over bs and recurse into its
// children, looking for a device mounted at / with type raid1. true is returned
// if such a device is found. The test is failed if a device of a different type
// is found to be mounted at /.
func checkIfMountpointIsRaidWalker(c cluster.TestCluster, bs []blockdevice, mountpoint string) bool {
	for _, b := range bs {
		if b.Mountpoint != nil && *b.Mountpoint == mountpoint {
			if b.Type != "raid1" {
				c.Fatalf("device %q is mounted at %q with type %q (was expecting raid1)", b.Name, mountpoint, b.Type)
			}
			return true
		}
		foundRoot := checkIfMountpointIsRaidWalker(c, b.Children, mountpoint)
		if foundRoot {
			return true
		}
	}
	return false
}
