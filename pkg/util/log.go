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

package util

import (
	"context"

	"k8s.io/klog/v2"
)

const LogKey = "jfs-logger"

type LoggerType string

func WithLog(parentCtx context.Context, log klog.Logger) context.Context {
	return context.WithValue(parentCtx, LoggerType(LogKey), log)
}

func GenLog(ctx context.Context, log klog.Logger, name string) klog.Logger {
	if ctx.Value(LoggerType(LogKey)) != nil {
		log = ctx.Value(LoggerType(LogKey)).(klog.Logger)
	}
	if name != "" {
		log = log.WithName(name)
	}
	return log
}
