package apps

import (
	"github.com/juicedata/juicefs-csi-driver/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Run() {
	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		klog.V(5).Infof("Could not create manager %v", err)
		os.Exit(1)
	}

	err = ctrl.NewControllerManagedBy(manager).
		For(&corev1.Pod{}).
		Complete(&controller.PodReconciler{
			Client: manager.GetClient(),
		})

	if err != nil {
		klog.V(5).Infof("Could not create controller: %v", err)
		os.Exit(1)
	}

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.V(5).Infof("Could not start manager: %v", err)
		os.Exit(1)
	}
}
