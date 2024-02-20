package validator

import (
	"context"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
)

type SecretValidator struct {
	jfs juicefs.Interface
}

var _ Validator[corev1.Secret] = &SecretValidator{}

func NewSecretValidator(jfs juicefs.Interface) *SecretValidator {
	return &SecretValidator{
		jfs: jfs,
	}
}

func (s *SecretValidator) Validate(ctx context.Context, secret corev1.Secret) error {
	secretsMap := make(map[string]string)
	if configs, ok := secret.Data["configs"]; ok {
		err := s.ValidateConfigs(string(configs[:]))
		if err != nil {
			return err
		}
	}
	if envs, ok := secret.Data["envs"]; ok {
		err := s.ValidateEnvs(string(envs[:]))
		if err != nil {
			return err
		}
	}

	for k, v := range secret.Data {
		secretsMap[k] = string(v[:])
	}
	jfsSetting, err := s.jfs.Settings(ctx, "", secretsMap, nil, nil)
	if err != nil {
		return err
	}

	tempConfDir, err := os.MkdirTemp(os.TempDir(), "juicefs-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempConfDir)
	jfsSetting.ClientConfPath = tempConfDir
	if jfsSetting.IsCe {
		metaUrl := secretsMap["metaurl"]
		if metaUrl == "" {
			return fmt.Errorf("metaurl is empty")
		}
		if err := s.jfs.Status(ctx, metaUrl); err != nil {
			return err
		}
	} else {
		_, err := s.jfs.AuthFs(ctx, secretsMap, jfsSetting, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *SecretValidator) ValidateConfigs(configs string) error {
	configMap := make(map[string]string)
	if err := config.ParseYamlOrJson(configs, &configMap); err != nil {
		return err
	}

	for k, v := range configMap {
		if v == "" {
			return fmt.Errorf("config %s is empty", k)
		}
		if !strings.HasPrefix(v, "/") {
			return fmt.Errorf("config %s is not absolute path", k)
		}
	}

	return nil
}

func (s *SecretValidator) ValidateEnvs(envs string) error {
	envsMap := make(map[string]string)
	return config.ParseYamlOrJson(envs, &envsMap)
}
