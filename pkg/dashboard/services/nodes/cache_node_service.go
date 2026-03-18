/*
 Copyright 2025 Juicedata Inc

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

package nodes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
)

var (
	nodeLog = klog.NewKlogr().WithName("NodeService/Cache")
)

type CacheNodeService struct {
	*nodeService

	nodeIndexes *utils.TimeOrderedIndexes[corev1.Node]
}

func (c *CacheNodeService) ListAllNodes(ctx context.Context) ([]corev1.Node, error) {
	nodes := make([]corev1.Node, 0, c.nodeIndexes.Length())
	for name := range c.nodeIndexes.Iterate(ctx, false) {
		var node corev1.Node
		if err := c.client.Get(ctx, name, &node); err == nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (c *CacheNodeService) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	node := &corev1.Node{}
	if err := c.client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			c.nodeIndexes.RemoveIndex(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		nodeLog.Error(err, "get node failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, err
	}
	if node.DeletionTimestamp != nil {
		c.nodeIndexes.RemoveIndex(req.NamespacedName)
		nodeLog.V(1).Info("node marked for deletion, index removed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	c.nodeIndexes.AddIndex(
		node,
		func(n *corev1.Node) metav1.ObjectMeta { return n.ObjectMeta },
		func(name types.NamespacedName) (*corev1.Node, error) {
			var n corev1.Node
			err := c.client.Get(ctx, name, &n)
			return &n, err
		},
	)
	nodeLog.V(1).Info("node reconciled/indexed", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *CacheNodeService) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("node", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.Node{}, &handler.TypedEnqueueRequestForObject[*corev1.Node]{}, predicate.TypedFuncs[*corev1.Node]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Node]) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Node]) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Node]) bool {
			node := deleteEvent.Object
			indexes := c.nodeIndexes
			if indexes != nil {
				indexes.RemoveIndex(types.NamespacedName{
					Name: node.GetName(),
				})
				nodeLog.V(1).Info("node deleted", "name", node.GetName())
				return false
			}
			return true
		},
	}))
}
