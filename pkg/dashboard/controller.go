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
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/resources/pods"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/resources/pvcs"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/resources/pvs"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/resources/secrets"
)

func (api *API) StartManager(ctx context.Context, mgr manager.Manager) error {
	podsSvc, ok := api.podSvc.(*pods.CachePodService)
	if !ok {
		return fmt.Errorf("pod service is not cache service")
	}
	if err := podsSvc.SetupWithManager(mgr); err != nil {
		return err
	}

	pvSvc, ok := api.pvSvc.(*pvs.CachePVService)
	if !ok {
		return fmt.Errorf("pv service is not cache service")
	}
	if err := pvSvc.SetupWithManager(mgr); err != nil {
		return err
	}

	pvcSvc, ok := api.pvcSvc.(*pvcs.CachePVCService)
	if !ok {
		return fmt.Errorf("pvc service is not cache service")
	}
	if err := pvcSvc.SetupWithManager(mgr); err != nil {
		return err
	}

	secretSvc, ok := api.secretSvc.(*secrets.CacheSecretService)
	if !ok {
		return fmt.Errorf("secret service is not cache service")
	}
	if err := secretSvc.SetupWithManager(mgr); err != nil {
		return err
	}

	return mgr.Start(ctx)
}
