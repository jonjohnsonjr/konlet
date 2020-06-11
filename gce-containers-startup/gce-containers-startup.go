// Copyright 2017 Google Inc. All Rights Reserved.
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
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/command"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/metadata"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/runtime"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/utils"

	api "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/types"
)

const METADATA_SERVER = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/gce-container-declaration"

var (
	tokenFlag       = flag.String("token", "", "what token to use")
	runDetachedFlag = flag.Bool("detached", true, "should we run container detached")
	showWelcomeFlag = flag.Bool("show-welcome", true, "should we show welcome on SSH to host OS")
	openIptables    = flag.Bool("open-iptables", true, "should we open IP Tables")
)

func main() {
	defer exitHandler()

	log.Printf("Starting Konlet container startup agent")
	flag.Parse()

	metadataProvider := metadata.DefaultProvider{}

	var authProvider utils.AuthProvider
	if *tokenFlag == "" {
		authProvider = utils.ServiceAccountTokenProvider{}
	} else {
		authProvider = utils.ConstantTokenProvider{Token: *tokenFlag}
	}

	runner, err := runtime.GetDefaultRunner(command.Runner{}, metadataProvider)
	if err != nil {
		log.Panicf("Failed to initialize Konlet: %v", err)
	}
	err = ExecStartup(metadataProvider, authProvider, runner, *openIptables)
	if err != nil {
		log.Panicf("Error: %v", err)
	}
}

func ExecStartup(metadataProvider metadata.Provider, authProvider utils.AuthProvider, runner *runtime.ContainerRunner, openIptables bool) error {
	var (
		auth = ""
		err  error
	)

	spec := api.Container{
		Name:    "debugme",
		Image:   "us-west1-docker.pkg.dev/jonjohnson-test/test/busybox",
		Command: []string{"sh"},
		Env: []struct {
			Name  string
			Value string
		}{{
			Name:  "PATH",
			Value: "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		}},
		StdIn: true,
		Tty:   true,
	}

	if utils.UseGcpTokenForImage(spec.Image) {
		auth, err = authProvider.RetrieveAuthToken()
		if err != nil {
			return fmt.Errorf("Cannot get auth token: %v", err)
		}
	} else {
		log.Printf("Non-GCR registry used - Konlet will use empty auth")
	}

	if openIptables {
		err = utils.OpenIptables()
		if err != nil {
			return fmt.Errorf("Cannot update IPtables: %v", err)
		}
	}

	log.Printf("Launching user container '%s'", spec.Image)
	err = runner.RunContainer(auth, spec, *runDetachedFlag)
	if err != nil {
		return fmt.Errorf("Failed to start container: %v", err)
	}

	return nil
}

func exitHandler() {
	err := recover()
	if *showWelcomeFlag {
		utils.WriteWelcomeScript(err)
	}
	if err != nil {
		os.Exit(1)
	}
}
