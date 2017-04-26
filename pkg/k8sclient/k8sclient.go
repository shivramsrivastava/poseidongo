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

package k8sclient

import (
	"fmt"
	"path"

	"k8s.io/kubernetes/pkg/api"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/watch"
)

type k8sConfig struct {
	address string
	QPS     int
	burst   int
}

const NodeBufferSize = 100
const PodBufferSize = 100

type Pod struct {
	ID string
}

type Node struct {
	ID string
}

type Client struct {
	k8sApi *k8sClient.Client
	nodeCh chan *Node
	podCh  chan *Pod
}

func StartNodeWatcher() (chan *Node, chan struct{}) {
	nodeCh := make(chan *Node, NodeBufferSize)
	_, nodeInformer := framework.NewInformer(
		cache.NewListWatchFromClient(c, "nodes", api.NamespaceAll,
			fields.ParseSelectorOrDie("")),
		&api.Node{},
		0,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(nodeObj interface{}) {
				node := nodeObj.(*api.Node)
				if node.Spec.Unschedulable {
					return
				}
				nodeCh <- &Node{
					ID: node.Name,
				}
			},
			UpdateFunc: func(oldNodeObj, newNodeObj interface{}) {},
			DeleteFunc: func(nodeObj interface{}) {},
		},
	)
	stopCh := make(chan struct{})
	go nodeInformer.Run(stopCh)
	return nodeCh, stopCh
}

func StartPodWatcher() (chan *Pod, chan struct{}) {
	podCh := make(chan *Pod, PodBufferSize)

	podSelector := fields.ParseSelectorOrDie("spec.nodeName==")
	podInformer := framework.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				options.FieldSelector = podSelector
				return c.Pods(api.NamespaceAll).List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				options.FieldSelector = podSelector
				return c.Pods(api.NamespaceAll).Watch(options)
			},
		},
		&api.Pod{},
		0,
	)
	podInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc: func(podObj interface{}) {
			pod := podObj.(*api.Pod)
			podeCh <- &Pod{
				ID: path.Join(pod.Namespace, pod.Name),
			}
		},
		UpdateFunc: func(oldPodObj, newPodObj interface{}) {},
		DeleteFunc: func(podObj interface{}) {},
	})
	stopCh := make(chan struct{})
	go podInformer.Run(stopCh)
	return podCh, stopCh
}

func New(k8sConfig config) (*Client, error) {
	restClientConfig := &restClient.Config{
		Host:  fmt.Sprintf("http://%s", config.address),
		QPS:   Config.QPS,
		Burst: Config.burst,
	}
	c, err := k8sClient.New(restClientConfig)
	if err != nil {
		return nil, err
	}
	nodeCh, stopNodeCh = StartNodeWatcher()
	podCh, stopPodCh = StartPodWatcher()
	return &Client{
		k8sApi: c,
		nodeCh: nodeCh,
		podCh:  podCh,
	}, nil
}