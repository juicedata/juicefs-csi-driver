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
	"regexp"
	"strings"

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

// stripArgKeys are the command options whose value StripSecretArgs masks: the
// keys StripSecret masks, and the session tokens format-options may carry.
var stripArgKeys = append([]string{"session-token", "session-token2"}, stripKeys...)

// urlArgKeys are the command options whose value is a URL or address that may
// hold a user and password.
var urlArgKeys = []string{"bucket", "bucket2"}

// urlUserInfo matches the user information of a URL or address, with or
// without a scheme or "//", inside $'...' quoting or not, up to the last "@"
// before the next "/".
var urlUserInfo = regexp.MustCompile(`^(\$')?((?:[A-Za-z][A-Za-z0-9+.-]*:)?//)?[^/]*@`)

// StripSecretArgs returns a copy of the arguments of a juicefs command, such as
// the ones GenFormatCmd and GenAuthCmd return, for logging: the value of every
// --<key>=<value> option whose key StripSecret masks, or a session token, is
// replaced with ***, and the user information of a --bucket or --bucket2 URL
// with ***. A value that is a reference to an environment variable, ${name},
// is kept.
func StripSecretArgs(args []string) []string {
	stripped := make([]string, 0, len(args))
	for _, arg := range args {
		stripped = append(stripped, stripArg(arg))
	}
	return stripped
}

func stripArg(arg string) string {
	if !strings.HasPrefix(arg, "--") {
		return arg
	}
	key, value, ok := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
	if !ok || value == "" || isEnvReference(value) {
		return arg
	}
	key = strings.ToLower(key)
	prefix := arg[:len(arg)-len(value)]
	if ContainsString(stripArgKeys, key) {
		return prefix + "***"
	}
	if ContainsString(urlArgKeys, key) {
		return prefix + urlUserInfo.ReplaceAllString(value, "${1}${2}***@")
	}
	return arg
}

// isEnvReference reports whether value is ${name}, which the shell expands from
// the environment.
func isEnvReference(value string) bool {
	return strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") && !strings.ContainsAny(value[2:len(value)-1], "${} ")
}
