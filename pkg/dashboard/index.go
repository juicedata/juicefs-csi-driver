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
	"log"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type k8sResource interface {
	corev1.Pod | corev1.PersistentVolume | corev1.PersistentVolumeClaim | storagev1.StorageClass
}

type timeOrderedIndexes[T k8sResource] struct {
	list *list.List
}

func newTimeIndexes[T k8sResource]() *timeOrderedIndexes[T] {
	return &timeOrderedIndexes[T]{list.New()}
}

func (i *timeOrderedIndexes[T]) iterate(ctx context.Context, descend bool) <-chan types.NamespacedName {
	ch := make(chan types.NamespacedName)
	go func() {
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
	return i.list.Len()
}

func (i *timeOrderedIndexes[T]) addIndex(name types.NamespacedName, resource *T, maps ...map[types.NamespacedName]*T) {
	for e := i.list.Back(); e != nil; e = e.Prev() {
		currentName := e.Value.(types.NamespacedName)
		var (
			currentResource *T
		)
		for _, m := range maps {
			if r, exist := m[currentName]; exist {
				currentResource = r
				break
			}
		}
		if currentResource == nil {
			i.list.Remove(e)
			continue
		}
		meta := getMeta(*resource)
		currentMeta := getMeta(*currentResource)
		if meta.UID == currentMeta.UID {
			break
		}
		if meta.CreationTimestamp.After(currentMeta.CreationTimestamp.Time) {
			i.list.InsertAfter(name, e)
			return
		}
	}
	i.list.PushFront(name)
}

func (i *timeOrderedIndexes[T]) removeIndex(name types.NamespacedName) {
	for e := i.list.Front(); e != nil; e = e.Next() {
		if e.Value.(types.NamespacedName) == name {
			i.list.Remove(e)
			break
		}
	}
}

func (i *timeOrderedIndexes[T]) debug() []types.NamespacedName {
	var names []types.NamespacedName
	for e := i.list.Front(); e != nil; e = e.Next() {
		names = append(names, e.Value.(types.NamespacedName))
	}
	return names
}

func getMeta(r any) metav1.ObjectMeta {
	switch resource := r.(type) {
	case corev1.Pod:
		return resource.ObjectMeta
	case corev1.PersistentVolume:
		return resource.ObjectMeta
	case corev1.PersistentVolumeClaim:
		return resource.ObjectMeta
	case storagev1.StorageClass:
		return resource.ObjectMeta
	default:
		log.Panicf("unsupported resouce type by time indexes: %s", reflect.TypeOf(r).String())
		return metav1.ObjectMeta{}
	}
}
