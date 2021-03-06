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
	"time"

	gpumanager "github.com/GoogleCloudPlatform/container-engine-accelerators/pkg/gpu/nvidia"
	"github.com/golang/glog"
)

const (
	// Device plugin settings.
	kubeletEndpoint      = "kubelet.sock"
	pluginEndpointPrefix = "nvidiaGPU"
)

var (
	hostPathPrefix      = flag.String("host-path", "/home/kubernetes/bin/nvidia", "Path on the host that contains nvidia libraries. This will be mounted inside the container as '-container-path'")
	containerPathPrefix = flag.String("container-path", "/usr/local/nvidia", "Path on the container that mounts '-host-path'")
	pluginMountPath     = flag.String("plugin-directory", "/device-plugin", "The directory path to create plugin socket")
)

func main() {
	flag.Parse()
	glog.Infoln("device-plugin started")
	ngm := gpumanager.NewNvidiaGPUManager(*hostPathPrefix, *containerPathPrefix)
	// Keep on trying until success. This is required
	// because Nvidia drivers may not be installed initially.
	for {
		err := ngm.Start()
		if err == nil {
			break
		}
		// Use non-default level to avoid log spam.
		glog.V(3).Infof("nvidiaGPUManager.Start() failed: %v", err)
		time.Sleep(5 * time.Second)
	}
	ngm.Serve(*pluginMountPath, kubeletEndpoint, fmt.Sprintf("%s-%d.sock", pluginEndpointPrefix, time.Now().Unix()))
}
