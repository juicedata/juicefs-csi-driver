package driver

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (d *Driver) juicefsAuth(source string, secrets map[string]string) ([]byte, error) {
	if secrets == nil || secrets["token"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Nil secrets or empty token")
	}

	token := secrets["token"]
	args := []string{"auth", source, "--token", token}
	keys := []string{"accesskey", "secretkey", "accesskey2", "secretkey2"}
	for _, k := range keys {
		v := secrets[k]
		args = append(args, "--"+k)
		if v != "" {
			args = append(args, v)
		} else {
			args = append(args, "''")
		}
	}
	return d.exec.Run(jfsCmd, args...)
}
