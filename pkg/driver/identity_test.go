/*
Copyright 2021 Juicedata Inc

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

package driver

import (
	"context"
	"reflect"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
)

func TestDriver_GetPluginInfo(t *testing.T) {
	type fields struct {
		controllerService controllerService
		nodeService       nodeService
		srv               *grpc.Server
		endpoint          string
	}
	type args struct {
		ctx context.Context
		req *csi.GetPluginInfoRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.GetPluginInfoResponse
		wantErr bool
	}{
		{
			name:   "test",
			fields: fields{},
			args: args{
				ctx: nil,
				req: &csi.GetPluginInfoRequest{},
			},
			want: &csi.GetPluginInfoResponse{
				Name:          DriverName,
				VendorVersion: "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				controllerService: tt.fields.controllerService,
				nodeService:       tt.fields.nodeService,
				srv:               tt.fields.srv,
				endpoint:          tt.fields.endpoint,
			}
			got, err := d.GetPluginInfo(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPluginInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetPluginInfo() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPluginCapabilities(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.GetPluginCapabilitiesRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.GetPluginCapabilitiesResponse
		wantErr bool
	}{
		{
			name:   "test",
			fields: fields{},
			args: args{
				req: &csi.GetPluginCapabilitiesRequest{},
			},
			want: &csi.GetPluginCapabilitiesResponse{
				Capabilities: []*csi.PluginCapability{{
					Type: &csi.PluginCapability_Service_{
						Service: &csi.PluginCapability_Service{
							Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
						},
					},
				}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{}
			got, err := d.GetPluginCapabilities(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ControllerGetCapabilities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ControllerGetCapabilities() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDriver_Probe(t *testing.T) {
	type fields struct {
		controllerService controllerService
		nodeService       nodeService
		srv               *grpc.Server
		endpoint          string
	}
	type args struct {
		ctx context.Context
		req *csi.ProbeRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ProbeResponse
		wantErr bool
	}{
		{
			name:   "test",
			fields: fields{},
			args: args{
				req: &csi.ProbeRequest{},
			},
			want:    &csi.ProbeResponse{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				controllerService: tt.fields.controllerService,
				nodeService:       tt.fields.nodeService,
				srv:               tt.fields.srv,
				endpoint:          tt.fields.endpoint,
			}
			got, err := d.Probe(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Probe() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Probe() got = %v, want %v", got, tt.want)
			}
		})
	}
}
