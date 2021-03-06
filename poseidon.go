// Poseidon
// Copyright (c) The Poseidon Authors.
// All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// THIS CODE IS PROVIDED ON AN *AS IS* BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING WITHOUT
// LIMITATION ANY IMPLIED WARRANTIES OR CONDITIONS OF TITLE, FITNESS FOR
// A PARTICULAR PURPOSE, MERCHANTABLITY OR NON-INFRINGEMENT.
//
// See the Apache Version 2.0 License for specific language governing
// permissions and limitations under the License.

package main

import (
	"flag"
	"time"

	"github.com/ICGog/poseidongo/pkg/firmament"
	"github.com/ICGog/poseidongo/pkg/k8sclient"
	"github.com/ICGog/poseidongo/pkg/stats"
	"github.com/golang/glog"
)

var (
	firmamentAddress   string
	kubeConfig         string
	statsServerAddress string
)

func init() {
	flag.StringVar(&firmamentAddress, "firmamentAddress", "127.0.0.1:9090", "Firmament scheduler address")
	flag.StringVar(&kubeConfig, "kubeConfig", "kubeconfig.cfg", "Path to the kubeconfig file")
	flag.StringVar(&statsServerAddress, "statsServerAddress", "127.0.0.1:9091", "Address on which the stats server listens")
	flag.Parse()
}

func schedule(fc firmament.FirmamentSchedulerClient) {
	for {
		deltas := firmament.Schedule(fc)
		glog.Infof("Scheduler returned %d deltas", len(deltas.GetDeltas()))
		for _, delta := range deltas.GetDeltas() {
			switch delta.GetType() {
			case firmament.SchedulingDelta_PLACE:
				podIdentifier, ok := k8sclient.TaskIDToPod[delta.GetTaskId()]
				if !ok {
					glog.Fatalf("Placed task %d without pod pairing", delta.GetTaskId())
				}
				nodeName, ok := k8sclient.ResIDToNode[delta.GetResourceId()]
				if !ok {
					glog.Fatalf("Placed task %d on resource %s without node pairing", delta.GetTaskId(), delta.GetResourceId())
				}
				k8sclient.BindPodToNode(podIdentifier.Name, podIdentifier.Namespace, nodeName)
			case firmament.SchedulingDelta_PREEMPT:
				glog.Fatalf("Pod preemption not currently supported.")
			case firmament.SchedulingDelta_MIGRATE:
				glog.Fatalf("Pod migration not currently supported.")
			case firmament.SchedulingDelta_NOOP:
			default:
				glog.Fatalf("Unexpected SchedulingDelta type %v", delta.GetType())
			}
		}
		// TODO(ionel): Temporary sleep statement because we currently call the scheduler even if there's no work do to.
		time.Sleep(10 * time.Second)
	}
}

func main() {
	glog.Info("Starting Poseidon...")
	fc, conn, err := firmament.New(firmamentAddress)
	defer conn.Close()
	if err != nil {
		panic(err)
	}
	go schedule(fc)
	go stats.StartgRPCStatsServer(statsServerAddress, firmamentAddress)
	k8sclient.New(kubeConfig, firmamentAddress)
}
