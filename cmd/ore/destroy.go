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

package main

import (
	"fmt"
	"os"

	"github.com/coreos/mantle/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/mantle/auth"
	"github.com/coreos/mantle/platform"
)

var (
	cmdDestroy = &cobra.Command{
		Use:   "destroy-instances -prefix=<prefix> ",
		Short: "destroy cluster on GCE",
		Long:  "Destroy GCE instances based on name prefix.",
		Run:   runDestroy,
	}
	destroyProject string
	destroyZone    string
	destroyPrefix  string
)

func init() {
	cmdDestroy.Flags().StringVar(&destroyProject, "project", "coreos-gce-testing", "found in developers console")
	cmdDestroy.Flags().StringVar(&destroyZone, "zone", "us-central1-a", "gce zone")
	cmdDestroy.Flags().StringVar(&destroyPrefix, "prefix", "", "prefix of name for instances to destroy")
	root.AddCommand(cmdDestroy)
}

func runDestroy(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "Unrecognized args in ore list cmd: %v\n", args)
		os.Exit(2)
	}

	// avoid wiping out all instances in project or mishaps with short destroyPrefixes
	if destroyPrefix == "" || len(destroyPrefix) < 2 {
		fmt.Fprintf(os.Stderr, "Please specify a prefix of length 2 or greater with -prefix\n")
		os.Exit(1)
	}

	client, err := auth.GoogleClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		os.Exit(1)
	}

	vms, err := platform.GCEListVMs(client, destroyProject, destroyZone, destroyPrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed listing vms: %v\n", err)
		os.Exit(1)
	}
	var count int
	for _, vm := range vms {
		err := platform.GCEDestroyVM(client, destroyProject, destroyZone, vm.ID())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed destroying vm: %v\n", err)
			os.Exit(1)
		}
		count++
	}
	fmt.Printf("%v instance(s) scheduled for deletion\n", count)
}
