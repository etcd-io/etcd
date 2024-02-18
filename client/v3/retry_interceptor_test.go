package clientv3

import (
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/v3/credentials"
	grpccredentials "google.golang.org/grpc/credentials"
	"testing"
)

type dummyAuthTokenBundle struct{}

func (d dummyAuthTokenBundle) TransportCredentials() grpccredentials.TransportCredentials {
	return nil
}

func (d dummyAuthTokenBundle) PerRPCCredentials() grpccredentials.PerRPCCredentials {
	return nil
}

func (d dummyAuthTokenBundle) NewWithMode(mode string) (grpccredentials.Bundle, error) {
	return nil, nil
}

func (d dummyAuthTokenBundle) UpdateAuthToken(token string) {
}

func TestClientShouldRefreshToken(t *testing.T) {
	type fields struct {
		authTokenBundle credentials.Bundle
	}
	type args struct {
		err      error
		callOpts *options
	}

	optsWithTrue := &options{
		retryAuth: true,
	}
	optsWithFalse := &options{
		retryAuth: false,
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "ErrUserEmpty and non nil authTokenBundle",
			fields: fields{
				authTokenBundle: &dummyAuthTokenBundle{},
			},
			args: args{rpctypes.ErrGRPCUserEmpty, optsWithTrue},
			want: true,
		},
		{
			name: "ErrUserEmpty and nil authTokenBundle",
			fields: fields{
				authTokenBundle: nil,
			},
			args: args{rpctypes.ErrGRPCUserEmpty, optsWithTrue},
			want: false,
		},
		{
			name: "ErrGRPCInvalidAuthToken and retryAuth",
			fields: fields{
				authTokenBundle: nil,
			},
			args: args{rpctypes.ErrGRPCInvalidAuthToken, optsWithTrue},
			want: true,
		},
		{
			name: "ErrGRPCInvalidAuthToken and !retryAuth",
			fields: fields{
				authTokenBundle: nil,
			},
			args: args{rpctypes.ErrGRPCInvalidAuthToken, optsWithFalse},
			want: false,
		},
		{
			name: "ErrGRPCAuthOldRevision and retryAuth",
			fields: fields{
				authTokenBundle: nil,
			},
			args: args{rpctypes.ErrGRPCAuthOldRevision, optsWithTrue},
			want: true,
		},
		{
			name: "ErrGRPCAuthOldRevision and !retryAuth",
			fields: fields{
				authTokenBundle: nil,
			},
			args: args{rpctypes.ErrGRPCAuthOldRevision, optsWithFalse},
			want: false,
		},
		{
			name: "Other error and retryAuth",
			fields: fields{
				authTokenBundle: nil,
			},
			args: args{rpctypes.ErrGRPCAuthFailed, optsWithTrue},
			want: false,
		},
		{
			name: "Other error and !retryAuth",
			fields: fields{
				authTokenBundle: nil,
			},
			args: args{rpctypes.ErrGRPCAuthFailed, optsWithFalse},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				authTokenBundle: tt.fields.authTokenBundle,
			}
			if got := c.shouldRefreshToken(tt.args.err, tt.args.callOpts); got != tt.want {
				t.Errorf("shouldRefreshToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
