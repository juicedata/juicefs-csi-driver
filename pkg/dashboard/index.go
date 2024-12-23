/*
 Copyright 2023 Juicedata Inc

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

package dashboard

import (
	"container/list"
	"context"
	"sync"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

var indexLog = klog.NewKlogr().WithName("index")

type k8sResource interface {
	corev1.Pod | corev1.PersistentVolume | corev1.PersistentVolumeClaim | storagev1.StorageClass | batchv1.Job | corev1.Secret
}

type timeOrderedIndexes[T k8sResource] struct {
	sync.RWMutex
	list *list.List
}

func newTimeIndexes[T k8sResource]() *timeOrderedIndexes[T] {
	return &timeOrderedIndexes[T]{
		list: list.New(),
	}
}

func (i *timeOrderedIndexes[T]) iterate(ctx context.Context, descend bool) <-chan types.NamespacedName {
	ch := make(chan types.NamespacedName)
	go func() {
		i.RLock()
		defer i.RUnlock()
		if descend {
			for e := i.list.Back(); e != nil && ctx.Err() == nil; e = e.Prev() {
				ch <- e.Value.(types.NamespacedName)
			}
		} else {
			for e := i.list.Front(); e != nil && ctx.Err() == nil; e = e.Next() {
				ch <- e.Value.(types.NamespacedName)
			}
		}
		close(ch)
	}()
	return ch
}

func (i *timeOrderedIndexes[T]) length() int {
	i.RLock()
	defer i.RUnlock()
	return i.list.Len()
}

func (i *timeOrderedIndexes[T]) addIndex(resource *T, metaGetter func(*T) metav1.ObjectMeta, resourceGetter func(types.NamespacedName) (*T, error)) {
	i.Lock()
	defer i.Unlock()
	meta := metaGetter(resource)
	name := types.NamespacedName{
		Namespace: meta.Namespace,
		Name:      meta.Name,
	}
	for e := i.list.Back(); e != nil; e = e.Prev() {
		currentResource, err := resourceGetter(e.Value.(types.NamespacedName))
		if err != nil || currentResource == nil {
			indexLog.V(1).Info("failed to get resource", "namespacedName", e.Value.(types.NamespacedName), "error", err)
			i.list.Remove(e)
			continue
		}
		currentMeta := metaGetter(currentResource)
		if meta.UID == currentMeta.UID {
			return
		}
		if meta.CreationTimestamp.After(currentMeta.CreationTimestamp.Time) {
			i.list.InsertAfter(name, e)
			return
		}
	}
	i.list.PushFront(name)
}

func (i *timeOrderedIndexes[T]) removeIndex(name types.NamespacedName) {
	i.Lock()
	defer i.Unlock()
	for e := i.list.Front(); e != nil; e = e.Next() {
		if e.Value.(types.NamespacedName) == name {
			i.list.Remove(e)
			return
		}
	}
}

func (i *timeOrderedIndexes[T]) debug() []types.NamespacedName {
	i.RLock()
	defer i.RUnlock()
	var names []types.NamespacedName
	for e := i.list.Front(); e != nil; e = e.Next() {
		names = append(names, e.Value.(types.NamespacedName))
	}
	return names
}
