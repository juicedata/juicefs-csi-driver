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

package util

import (
	"reflect"
	"testing"
)

func TestStripSecretArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "community edition format",
			args: []string{"/usr/local/bin/juicefs", "format", "--storage=s3", "--bucket=https://bucket.example.com",
				"--access-key=AKEXAMPLE", "--secret-key=${secretkey}", "${metaurl}", "vol",
				"--session-token=${session-token}", "--trash-days=1"},
			want: []string{"/usr/local/bin/juicefs", "format", "--storage=s3", "--bucket=https://bucket.example.com",
				"--access-key=***", "--secret-key=${secretkey}", "${metaurl}", "vol",
				"--session-token=${session-token}", "--trash-days=1"},
		},
		{
			name: "enterprise edition auth",
			args: []string{"/usr/bin/juicefs", "auth", "vol", "--access-key=AK1", "--access-key2=AK2", "--bucket=b1",
				"--token=${token}", "--secret-key=${secretkey}", "--passphrase=${passphrase}", "--conf-dir=/root/.juicefs"},
			want: []string{"/usr/bin/juicefs", "auth", "vol", "--access-key=***", "--access-key2=***", "--bucket=b1",
				"--token=${token}", "--secret-key=${secretkey}", "--passphrase=${passphrase}", "--conf-dir=/root/.juicefs"},
		},
		{
			name: "credentials given as format options",
			args: []string{"--secret-key=SKEXAMPLE", "--session-token=STEXAMPLE", "--passphrase=PPEXAMPLE", "--token=TKEXAMPLE",
				"--secret-key2=SK2EXAMPLE", "--password=PWEXAMPLE", "--secretkey=SK3EXAMPLE", "--session-token2=ST2EXAMPLE",
				"--Secret-Key=SK4EXAMPLE", "--passphrase=a=b"},
			want: []string{"--secret-key=***", "--session-token=***", "--passphrase=***", "--token=***",
				"--secret-key2=***", "--password=***", "--secretkey=***", "--session-token2=***",
				"--Secret-Key=***", "--passphrase=***"},
		},
		{
			name: "user information in a bucket URL",
			args: []string{"--bucket=https://user:pass@bucket.example.com/path", "--bucket2=$'webdav://u:p@example.com/a;b'",
				"--bucket=https://bucket.example.com/user:pass@x", "--bucket=https://user:p@ss@bucket.example.com/",
				"--bucket=root:pass@db.example.com:5432/jfs", "--bucket=$'root:pass@(db.example.com:3306)/jfs'",
				"--bucket=https://bucket.example.com", "--bucket=//user:pass@bucket.example.com/b"},
			want: []string{"--bucket=https://***@bucket.example.com/path", "--bucket2=$'webdav://***@example.com/a;b'",
				"--bucket=https://bucket.example.com/user:pass@x", "--bucket=https://***@bucket.example.com/",
				"--bucket=***@db.example.com:5432/jfs", "--bucket=$'***@(db.example.com:3306)/jfs'",
				"--bucket=https://bucket.example.com", "--bucket=//***@bucket.example.com/b"},
		},
		{
			name: "escaped values and look-alikes",
			args: []string{"--access-key=$'a;b'", "--secret-key=${secretkey}x", "--access-key=", "access-key=AK", "-access-key=AK",
				"--accesskeyx=AK", "--access-key"},
			want: []string{"--access-key=***", "--secret-key=***", "--access-key=", "access-key=AK", "-access-key=AK",
				"--accesskeyx=AK", "--access-key"},
		},
		{
			name: "nothing to strip",
			args: []string{},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := append([]string{}, tt.args...)
			if got := StripSecretArgs(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StripSecretArgs() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.args, original) {
				t.Errorf("StripSecretArgs() changed its input to %v", tt.args)
			}
		})
	}
}
