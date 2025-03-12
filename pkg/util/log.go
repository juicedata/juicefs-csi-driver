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
	"encoding/json"

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
		return log
	}
	if name != "" {
		log = log.WithName(name)
	}
	return log
}

var stripKeys = []string{
	"token",
	"accesskey",
	"access-key",
	"accesskey2",
	"access-key2",
	"secretkey",
	"secret-key",
	"secretkey2",
	"secret-key2",
	"passphrase",
	"password",
}

func StripSecret(secret map[string]string) map[string]string {
	s := make(map[string]string)
	for k, v := range secret {
		s[k] = v
	}
	if len(s) == 0 {
		return s
	}
	for _, key := range stripKeys {
		if _, ok := s[key]; ok {
			s[key] = "***"
		}
	}
	if _, ok := s["initconfig"]; ok {
		var initconfig map[string]string
		_ = json.Unmarshal([]byte(s["initconfig"]), &initconfig)

		stripped := StripSecret(initconfig)
		b, _ := json.Marshal(stripped)
		s["initconfig"] = string(b)
	}
	return s
}
