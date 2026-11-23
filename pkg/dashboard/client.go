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

package dashboard

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const resourceReadTimeout = 10 * time.Second
const resourceReadTimeoutMessage = "Timed out reading Kubernetes resources. Check Dashboard RBAC permissions or API server connectivity."

type readTimeoutClient struct {
	client.Client
}

func (c *readTimeoutClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	ctx, cancel := context.WithTimeout(ctx, resourceReadTimeout)
	defer cancel()
	err := c.Client.Get(ctx, key, obj, opts...)
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		return apierrors.NewTimeoutError(resourceReadTimeoutMessage, 0)
	}
	return err
}

func (c *readTimeoutClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	ctx, cancel := context.WithTimeout(ctx, resourceReadTimeout)
	defer cancel()
	err := c.Client.List(ctx, list, opts...)
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		return apierrors.NewTimeoutError(resourceReadTimeoutMessage, 0)
	}
	return err
}
