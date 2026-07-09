/*
Copyright 2026 Juicedata Inc

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

package ddschedulerextender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const nodeNameIndex = "spec.nodeName"

type Options struct {
	Address            string
	DefaultHeadroomCPU string
	DefaultHeadroomMem string
	DefaultHeadroomPod int64
	AnnotationPrefixes []string
	CacheSyncTimeout   time.Duration
}

type Server struct {
	address          string
	defaultHeadroom  Headroom
	annotationPrefix []string
	cacheSyncTimeout time.Duration

	informerFactory informers.SharedInformerFactory
	podInformer     coreinformers.PodInformer
	nodeInformer    coreinformers.NodeInformer
	podLister       corelisters.PodLister
	nodeLister      corelisters.NodeLister

	log logr.Logger
}

func NewServer(client kubernetes.Interface, opts Options) (*Server, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is nil")
	}
	if opts.Address == "" {
		opts.Address = ":9908"
	}
	if opts.CacheSyncTimeout == 0 {
		opts.CacheSyncTimeout = 30 * time.Second
	}
	defaultHeadroom, err := parseHeadroom(opts.DefaultHeadroomCPU, opts.DefaultHeadroomMem, opts.DefaultHeadroomPod)
	if err != nil {
		return nil, err
	}

	factory := informers.NewSharedInformerFactory(client, 0)
	podInformer := factory.Core().V1().Pods()
	nodeInformer := factory.Core().V1().Nodes()
	if err := podInformer.Informer().AddIndexers(cache.Indexers{
		nodeNameIndex: func(obj interface{}) ([]string, error) {
			pod, ok := obj.(*corev1.Pod)
			if !ok || pod.Spec.NodeName == "" {
				return nil, nil
			}
			return []string{pod.Spec.NodeName}, nil
		},
	}); err != nil {
		return nil, err
	}

	return &Server{
		address:          opts.Address,
		defaultHeadroom:  defaultHeadroom,
		annotationPrefix: opts.AnnotationPrefixes,
		cacheSyncTimeout: opts.CacheSyncTimeout,
		informerFactory:  factory,
		podInformer:      podInformer,
		nodeInformer:     nodeInformer,
		podLister:        podInformer.Lister(),
		nodeLister:       nodeInformer.Lister(),
		log:              klog.NewKlogr().WithName("scheduler-extender"),
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	s.informerFactory.Start(ctx.Done())
	syncCtx, cancel := context.WithTimeout(ctx, s.cacheSyncTimeout)
	defer cancel()
	if !cache.WaitForCacheSync(syncCtx.Done(), s.podInformer.Informer().HasSynced, s.nodeInformer.Informer().HasSynced) {
		return fmt.Errorf("timed out waiting for scheduler extender caches to sync")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/filter", s.handleFilter)
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleHealthz)

	server := &http.Server{
		Addr:              s.address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			s.log.Error(err, "failed to shut down scheduler extender")
		}
	}()

	s.log.Info("starting scheduler extender", "address", s.address)
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleFilter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var args ExtenderArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, fmt.Sprintf("decode filter request: %v", err), http.StatusBadRequest)
		return
	}

	result := s.Filter(&args)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		s.log.Error(err, "encode filter response")
	}
}

func (s *Server) Filter(args *ExtenderArgs) ExtenderFilterResult {
	if args == nil {
		return ExtenderFilterResult{Error: "filter args cannot be nil"}
	}

	headroom, err := headroomFromPod(&args.Pod, s.defaultHeadroom, s.annotationPrefix)
	if err != nil {
		return ExtenderFilterResult{Error: err.Error()}
	}
	if headroom.Empty() {
		return passAll(args)
	}

	candidates, returnNodeNames, err := s.candidateNodes(args)
	if err != nil {
		return ExtenderFilterResult{Error: err.Error()}
	}

	filtered := make([]corev1.Node, 0, len(candidates))
	filteredNames := make([]string, 0, len(candidates))
	failed := FailedNodesMap{}
	required := podRequest(&args.Pod).Add(headroom)

	for i := range candidates {
		node := &candidates[i]
		used, err := s.requestedOnNode(node.Name)
		if err != nil {
			failed[node.Name] = err.Error()
			continue
		}
		if ok, reason := fits(node, used, required); !ok {
			failed[node.Name] = reason
			continue
		}
		filtered = append(filtered, *node.DeepCopy())
		filteredNames = append(filteredNames, node.Name)
	}

	result := ExtenderFilterResult{
		FailedNodes: failed,
	}
	if returnNodeNames {
		result.NodeNames = &filteredNames
		return result
	}
	result.Nodes = &corev1.NodeList{Items: filtered}
	return result
}

func passAll(args *ExtenderArgs) ExtenderFilterResult {
	if args.NodeNames != nil {
		names := append([]string{}, (*args.NodeNames)...)
		return ExtenderFilterResult{
			NodeNames:   &names,
			FailedNodes: FailedNodesMap{},
		}
	}
	if args.Nodes == nil {
		return ExtenderFilterResult{
			Nodes:       &corev1.NodeList{},
			FailedNodes: FailedNodesMap{},
		}
	}
	return ExtenderFilterResult{
		Nodes:       args.Nodes.DeepCopy(),
		FailedNodes: FailedNodesMap{},
	}
}

func (s *Server) candidateNodes(args *ExtenderArgs) ([]corev1.Node, bool, error) {
	if args.NodeNames != nil {
		nodes := make([]corev1.Node, 0, len(*args.NodeNames))
		for _, nodeName := range *args.NodeNames {
			node, err := s.nodeLister.Get(nodeName)
			if err != nil {
				return nil, true, fmt.Errorf("get node %q from cache: %w", nodeName, err)
			}
			nodes = append(nodes, *node.DeepCopy())
		}
		return nodes, true, nil
	}
	if args.Nodes == nil {
		return nil, false, nil
	}
	return append([]corev1.Node{}, args.Nodes.Items...), false, nil
}

func (s *Server) requestedOnNode(nodeName string) (Headroom, error) {
	objects, err := s.podInformer.Informer().GetIndexer().ByIndex(nodeNameIndex, nodeName)
	if err != nil {
		return Headroom{}, err
	}
	var used Headroom
	for _, obj := range objects {
		pod, ok := obj.(*corev1.Pod)
		if !ok || !podConsumesResources(pod) {
			continue
		}
		used = used.Add(podRequest(pod))
	}
	return used, nil
}

func fits(node *corev1.Node, used Headroom, required Headroom) (bool, string) {
	allocatable := nodeAllocatable(node)
	shortages := []string{}
	if used.MilliCPU+required.MilliCPU > allocatable.MilliCPU {
		shortages = append(shortages, fmt.Sprintf("cpu requested=%dm used=%dm capacity=%dm", required.MilliCPU, used.MilliCPU, allocatable.MilliCPU))
	}
	if used.Memory+required.Memory > allocatable.Memory {
		shortages = append(shortages, fmt.Sprintf("memory requested=%d used=%d capacity=%d", required.Memory, used.Memory, allocatable.Memory))
	}
	if used.Pods+required.Pods > allocatable.Pods {
		shortages = append(shortages, fmt.Sprintf("pods requested=%d used=%d capacity=%d", required.Pods, used.Pods, allocatable.Pods))
	}
	if len(shortages) > 0 {
		return false, "insufficient headroom: " + strings.Join(shortages, ", ")
	}
	return true, ""
}
