/*
 Copyright 2024 Juicedata Inc

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

package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
)

type ExecCli struct {
	*exec.ExecOptions
	clientSet *kubernetes.Clientset
}

func NewExecCli(clientSet *kubernetes.Clientset) *ExecCli {
	eCli := ExecCli{
		ExecOptions: &exec.ExecOptions{
			StreamOptions: exec.StreamOptions{
				IOStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
			},
			Executor: &exec.DefaultRemoteExecutor{},
		},
		clientSet: clientSet,
	}
	return &eCli
}

func (e *ExecCli) Completion() *ExecCli {
	e.PodClient = e.clientSet.CoreV1()
	e.Executor = &exec.DefaultRemoteExecutor{}
	conf, err := KubernetesConfigFlags.ToRESTConfig()
	if err != nil {
		panic(err)
	}
	if err = setKubernetesDefaults(conf); err != nil {
		panic(err)
	}
	e.Config = conf
	return e
}

func (e *ExecCli) SetNamespace(ns string) *ExecCli {
	e.Namespace = ns
	return e
}

func (e *ExecCli) SetPod(podName string) *ExecCli {
	e.PodName = podName
	return e
}

func (e *ExecCli) Container(cn string) *ExecCli {
	e.ContainerName = cn
	return e
}

func (e *ExecCli) Commands(commands []string) *ExecCli {
	e.Command = commands
	return e
}

func setKubernetesDefaults(config *rest.Config) error {
	config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}

	if config.APIPath == "" {
		config.APIPath = "/api"
	}
	if config.NegotiatedSerializer == nil {
		// This codec factory ensures the resources are not converted. Therefore, resources
		// will not be round-tripped through internal versions. Defaulting does not happen
		// on the client.
		config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	}
	return rest.SetKubernetesDefaults(config)
}
