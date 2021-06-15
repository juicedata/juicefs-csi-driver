package reconcile

import (
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"reflect"
)

type Status struct {
	Events []common.Event
	Mount  *mountv1.JuiceMount
	Status *mountv1.JuiceMountStatus
}

func (s *Status) Apply() ([]common.Event, *mountv1.JuiceMount) {
	pre, crt := s.Mount.Status, s.Status
	if reflect.DeepEqual(pre, crt) {
		return s.Events, nil
	}
	mount := s.Mount
	mount.Status = *crt
	return s.Events, mount
}

func NewStatus(mount *mountv1.JuiceMount) *Status {
	return &Status{
		Mount:    mount,
		Status:   mount.Status.DeepCopy(),
	}
}
