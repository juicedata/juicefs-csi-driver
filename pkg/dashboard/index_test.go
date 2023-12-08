package dashboard

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestIndexAdd(t *testing.T) {
	i := newTimeIndexes[corev1.Pod]()
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "default",
			UID:       "1",
			CreationTimestamp: metav1.Time{
				Time: time.Now().Add(-1 * time.Hour),
			},
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pod2",
			Namespace:         "default",
			UID:               "2",
			CreationTimestamp: metav1.Now(),
		},
	}

	metaGetter := func(p *corev1.Pod) metav1.ObjectMeta {
		return p.ObjectMeta
	}
	resourceGetter := func(n types.NamespacedName) (*corev1.Pod, error) {
		if n.Name == "pod1" {
			return pod1, nil
		}
		if n.Name == "pod2" {
			return pod2, nil
		}
		return nil, nil
	}

	i.addIndex(pod2, metaGetter, resourceGetter)
	i.addIndex(pod1, metaGetter, resourceGetter)
	if i.length() != 2 {
		t.Errorf("expected length of 2, got %d", i.length())
	}
	if !reflect.DeepEqual(i.debug(), []types.NamespacedName{
		{
			Namespace: "default",
			Name:      "pod1",
		},
		{
			Namespace: "default",
			Name:      "pod2",
		},
	}) {
		t.Errorf("expected index to be [pod1, pod2], got %v", i.debug())
	}
	i.addIndex(pod1, metaGetter, resourceGetter)
	i.addIndex(pod1, metaGetter, resourceGetter)
	if i.length() != 2 {
		t.Errorf("expected length of 2, got %d", i.length())
	}
	if !reflect.DeepEqual(i.debug(), []types.NamespacedName{
		{
			Namespace: "default",
			Name:      "pod1",
		},
		{
			Namespace: "default",
			Name:      "pod2",
		},
	}) {
		t.Errorf("expected index to be [pod1, pod2], got %v", i.debug())
	}
}
