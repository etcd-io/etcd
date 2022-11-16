// Copyright 2022 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flagutil_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"go.etcd.io/etcd/client/pkg/v3/flagutil"
)

func TestDuration_String(t *testing.T) {
	tests := []struct {
		name string
		d    flagutil.Duration
		want string
	}{
		{
			name: "Nothing",
			d:    flagutil.Duration{},
			want: "0s",
		},
		{
			name: "Microseconds",
			d:    flagutil.ToDuration(2 * time.Microsecond),
			want: "2µs",
		},
		{
			name: "Milliseconds",
			d:    flagutil.ToDuration(2 * time.Millisecond),
			want: "2ms",
		},
		{
			name: "Seconds",
			d:    flagutil.ToDuration(2 * time.Second),
			want: "2s",
		},
		{
			name: "Minutes",
			d:    flagutil.ToDuration(2 * time.Minute),
			want: "2m0s",
		},
		{
			name: "Minutes-With-Lower-Units",
			d:    flagutil.ToDuration(220 * time.Second),
			want: "3m40s",
		},
		{
			name: "Hours",
			d:    flagutil.ToDuration(2 * time.Hour),
			want: "2h0m0s",
		},
		{
			name: "Hours-With-Lower-Units",
			d:    flagutil.ToDuration(3678 * time.Second),
			want: "1h1m18s",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.String(); got != tt.want {
				t.Errorf("Duration.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDuration_Type(t *testing.T) {
	tests := []struct {
		name string
		d    flagutil.Duration
		want string
	}{
		{
			name: "Nothing",
			d:    flagutil.Duration{},
			want: "duration",
		},
		{
			name: "Valid-Duration",
			d:    flagutil.ToDuration(2 * time.Second),
			want: "duration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.Type(); got != tt.want {
				t.Errorf("Duration.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDuration_Set(t *testing.T) {
	tests := []struct {
		name    string
		d       *flagutil.Duration
		input   string
		output  string
		wantErr bool
	}{
		{
			name:    "No-Input",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "",
			output:  "0s",
			wantErr: true,
		},
		{
			name:    "Microseconds",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "10us",
			output:  "10µs",
			wantErr: false,
		},
		{
			name:    "Milliseconds",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "10ms",
			output:  "10ms",
			wantErr: false,
		},
		{
			name:    "Seconds",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "10s",
			output:  "10s",
			wantErr: false,
		},
		{
			name:    "Minutes",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "10m",
			output:  "10m0s",
			wantErr: false,
		},
		{
			name:    "Minutes-With-Lower-Units",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "10m20s",
			output:  "10m20s",
			wantErr: false,
		},
		{
			name:    "Seconds-to-Minutes",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "620s",
			output:  "10m20s",
			wantErr: false,
		},
		{
			name:    "Hours",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "10h",
			output:  "10h0m0s",
			wantErr: false,
		},
		{
			name:    "Hours-With-Lower-Units",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "10h20m30s",
			output:  "10h20m30s",
			wantErr: false,
		},
		{
			name:    "Seconds-to-Hours",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "3600s",
			output:  "1h0m0s",
			wantErr: false,
		},
		{
			name:    "Minutes-to-Hours",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "60m",
			output:  "1h0m0s",
			wantErr: false,
		},
		{
			name:    "Without-Units",
			d:       flagutil.NewDuration(new(time.Duration), nil),
			input:   "3600",
			output:  "1h0m0s",
			wantErr: false,
		},
		{
			name:    "Replace-Duration",
			d:       ptr(flagutil.ToDuration(10 * time.Second)),
			input:   "20s",
			output:  "20s",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.d.Set(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("Duration.Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.d.String() != tt.output {
				t.Errorf("Duration expected = %s, got %s", tt.output, tt.d.String())
			}
		})
	}
}

func TestDurationFlag(t *testing.T) {
	type args struct {
		p         *flagutil.Duration
		name      string
		value     flagutil.Duration
		usage     string
		descUnits bool
	}
	tests := []struct {
		name string
		args args
		want *pflag.Flag
	}{
		{
			name: "With-Args",
			args: args{
				p:         flagutil.NewDuration(new(time.Duration), nil),
				name:      "input",
				value:     flagutil.ToDuration(10 * time.Second),
				usage:     "input flag",
				descUnits: false,
			},
			want: &pflag.Flag{
				Name:     "input",
				Usage:    "input flag",
				Value:    ptr(flagutil.ToDuration(10 * time.Second)),
				DefValue: "10s",
			},
		},
		{
			name: "Default-Inputs",
			args: args{},
			want: &pflag.Flag{
				Name:     "",
				Usage:    "",
				Value:    pflag.Value(ptr(flagutil.Duration{})),
				DefValue: "0s",
			},
		},
		{
			name: "Enable-Desc-Units",
			args: args{
				p:         flagutil.NewDuration(new(time.Duration), nil),
				name:      "input",
				value:     flagutil.ToDuration(10 * time.Second),
				usage:     "input flag",
				descUnits: true,
			},
			want: &pflag.Flag{
				Name:     "input",
				Usage:    "input flag (default units: seconds)",
				Value:    ptr(flagutil.ToDuration(10 * time.Second)),
				DefValue: "10s",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagutil.DurationFlag(tt.args.p, tt.args.name, tt.args.value, tt.args.usage, tt.args.descUnits); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DurationFlag() = %+v, want %+v", *got, *tt.want)
			}
		})
	}
}

func ptr[T any](f T) *T {
	return &f
}

func TestDuration_Dur(t *testing.T) {
	tests := []struct {
		name string
		d    flagutil.Duration
		want time.Duration
	}{
		{
			name: "No-Underlying-Time-Duration",
			d:    flagutil.Duration{},
			want: 0,
		},
		{
			name: "With-Valid-Time-Duration",
			d:    flagutil.ToDuration(10 * time.Second),
			want: 10 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.Dur(); got != tt.want {
				t.Errorf("Duration.Dur() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddDefaultUnitsDesc(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Empty-Input",
			input: "",
			want:  " (default units: seconds)",
		},
		{
			name:  "With-Valid-Desc",
			input: "This is the tag",
			want:  "This is the tag (default units: seconds)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagutil.AddDefaultUnitsDesc(tt.input); got != tt.want {
				t.Errorf("AddDefaultUnitsDesc() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDuration(t *testing.T) {
	type args struct {
		duration        *time.Duration
		defaultDuration *time.Duration
	}
	tests := []struct {
		name string
		args args
		want *flagutil.Duration
	}{
		{
			name: "Zero-Duration",
			args: args{
				duration:        new(time.Duration),
				defaultDuration: nil,
			},
			want: ptr(flagutil.ToDuration(time.Duration(0))),
		},
		{
			name: "Just-Duration-Object",
			args: args{
				duration:        new(time.Duration),
				defaultDuration: nil,
			},
			want: ptr(flagutil.ToDuration(time.Duration(0))),
		},
		{
			name: "Duration-With-Default-Duration",
			args: args{
				duration:        new(time.Duration),
				defaultDuration: ptr(200 * time.Second),
			},
			want: ptr(flagutil.ToDuration(200 * time.Second)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagutil.NewDuration(tt.args.duration, tt.args.defaultDuration); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
