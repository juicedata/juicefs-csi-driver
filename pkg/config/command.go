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

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
)

// args: the real args running in csi
// cmdArgs: the args in mount pod which using stripped value with env
func GenAuthCmd(secrets map[string]string, setting *JfsSetting) (args []string, cmdArgs []string, err error) {
	if secrets == nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return nil, nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}

	args = []string{"auth", security.EscapeBashStr(secrets["name"])}             // the real args running in csi
	cmdArgs = []string{CliPath, "auth", security.EscapeBashStr(secrets["name"])} // the args in mount pod which using stripped value with env

	keysCompatible := map[string]string{
		"accesskey":  "access-key",
		"accesskey2": "access-key2",
		"secretkey":  "secret-key",
		"secretkey2": "secret-key2",
	}
	// compatible
	for compatibleKey, realKey := range keysCompatible {
		if value, ok := secrets[compatibleKey]; ok {
			log.Info("transform key", "compatibleKey", compatibleKey, "realKey", realKey)
			secrets[realKey] = value
			delete(secrets, compatibleKey)
		}
	}

	keys := []string{
		"access-key",
		"access-key2",
		"bucket",
		"bucket2",
		"subdir",
	}
	keysStripped := []string{
		"token",
		"secret-key",
		"secret-key2",
		"passphrase",
	}
	strippedkey := map[string]string{
		"secret-key":  "secretkey",
		"secret-key2": "secretkey2",
	}
	for _, k := range keys {
		if secrets[k] != "" {
			v := security.EscapeBashStr(secrets[k])
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
			args = append(args, fmt.Sprintf("--%s=%s", k, v))
		}
	}
	for _, k := range keysStripped {
		if secrets[k] != "" {
			argKey := k
			if v, ok := strippedkey[k]; ok {
				argKey = v
			}
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=${%s}", k, argKey))
			args = append(args, fmt.Sprintf("--%s=%s", k, security.EscapeBashStr(secrets[k])))
		}
	}
	if v, ok := os.LookupEnv("JFS_NO_UPDATE_CONFIG"); ok && v == "enabled" {
		cmdArgs = append(cmdArgs, "--no-update")
		args = append(args, "--no-update")
		if secrets["bucket"] == "" {
			return nil, nil, fmt.Errorf("bucket argument is required when --no-update option is provided")
		}
		if ByProcess && secrets["initconfig"] != "" {
			conf := secrets["name"] + ".conf"
			confPath := filepath.Join(setting.ClientConfPath, conf)
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				err = os.WriteFile(confPath, []byte(secrets["initconfig"]), 0644)
				if err != nil {
					return nil, nil, fmt.Errorf("create config file %q failed: %v", confPath, err)
				}
				log.Info("Create config file success", "confPath", confPath)
			}
		}
	}
	if setting.FormatOptions != "" {
		options, err := setting.ParseFormatOptions()
		if err != nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "Parse format options error: %v", err)
		}
		args = append(args, setting.RepresentFormatOptions(options)...)
		stripped := setting.StripFormatOptions(options, []string{"session-token"})
		cmdArgs = append(cmdArgs, stripped...)
	}

	if setting.ClientConfPath != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--conf-dir=%s", setting.ClientConfPath))
		args = append(args, fmt.Sprintf("--conf-dir=%s", setting.ClientConfPath))
	}

	log.Info("AuthFs cmd", "args", cmdArgs)
	return
}

func GenFormatCmd(secrets map[string]string, noUpdate bool, setting *JfsSetting) (args []string, cmdArgs []string, err error) {
	if secrets == nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return nil, nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}

	if secrets["metaurl"] == "" {
		return nil, nil, status.Errorf(codes.InvalidArgument, "Empty metaurl")
	}

	args = []string{"format"}
	cmdArgs = []string{CeCliPath, "format"}
	if noUpdate {
		cmdArgs = append(cmdArgs, "--no-update")
		args = append(args, "--no-update")
	}
	keys := []string{
		"storage",
		"bucket",
		"access-key",
		"block-size",
		"compress",
		"trash-days",
		"capacity",
		"inodes",
		"shards",
	}
	keysStripped := map[string]string{"secret-key": "secretkey"}
	for _, k := range keys {
		if secrets[k] != "" {
			v := security.EscapeBashStr(secrets[k])
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
			args = append(args, fmt.Sprintf("--%s=%s", k, v))
		}
	}
	for k, v := range keysStripped {
		if secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=${%s}", k, v))
			args = append(args, fmt.Sprintf("--%s=%s", k, security.EscapeBashStr(secrets[k])))
		}
	}
	cmdArgs = append(cmdArgs, "${metaurl}", secrets["name"])
	args = append(args, security.EscapeBashStr(secrets["metaurl"]), security.EscapeBashStr(secrets["name"]))

	if setting.FormatOptions != "" {
		options, err := setting.ParseFormatOptions()
		if err != nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "Parse format options error: %v", err)
		}
		args = append(args, setting.RepresentFormatOptions(options)...)
		stripped := setting.StripFormatOptions(options, []string{"session-token"})
		cmdArgs = append(cmdArgs, stripped...)
	}

	log.Info("ce format cmd", "args", cmdArgs)
	return
}
