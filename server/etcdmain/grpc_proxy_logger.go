// Copyright 2025 The etcd Authors
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

package etcdmain

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"slices"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

// grpcProxyLogger implements the go-grpc-middleware v2 Reporter interface
type grpcProxyLogger struct {
	logger *zap.Logger
	fields []zap.Field
}

var _ interceptors.Reporter = (*grpcProxyLogger)(nil)

const (
	responseCallType = "response"
	requestCallType  = "request"
)

func (r *grpcProxyLogger) PostCall(_ error, _ time.Duration) {
	// no op - no post-call payload logging
}

func (r *grpcProxyLogger) PostMsgReceive(payload any, err error, _ time.Duration) {
	callType := requestCallType
	f := logFieldsFromPayload(payload, err, callType)
	r.logger.Info(fmt.Sprintf("received payload logged as grpc.%s.content field", callType), slices.Concat(f, r.fields)...)
}

func (r *grpcProxyLogger) PostMsgSend(payload any, err error, _ time.Duration) {
	callType := responseCallType
	f := logFieldsFromPayload(payload, err, callType)
	r.logger.Info(fmt.Sprintf("returned response payload logged as grpc.%s.content field", callType), slices.Concat(f, r.fields)...)
}

func logFieldsFromPayload(payload any, err error, callType string) []zap.Field {
	fields := []zap.Field{}
	if err != nil {
		fields = append(fields, zap.NamedError(fmt.Sprintf("grpc.%s.error", callType), err))
	}
	p, ok := payload.(proto.Message)
	if !ok {
		fields = append(fields, zap.NamedError("msg.type.error", fmt.Errorf("payload is not a github.com/golang/protobuf/proto message")))
	}
	msg, pErr := protoToJSON(p)
	if pErr != nil {
		fields = append(fields, zap.NamedError("msg.proto.error", fmt.Errorf("error when converting payload to logging json: %w", pErr)))
	}
	fields = append(fields, zap.String(fmt.Sprintf("grpc.%s.content", callType), msg))
	return fields
}

func protoToJSON(msg proto.Message) (string, error) {
	if reflect.ValueOf(msg).IsNil() {
		return "", nil
	}
	marshaler := jsonpb.Marshaler{}
	return marshaler.MarshalToString(msg)
}

func reportable(lg *zap.Logger) interceptors.CommonReportableFunc {
	return func(ctx context.Context, c interceptors.CallMeta) (interceptors.Reporter, context.Context) {
		fields := []zap.Field{
			zap.String("grpc.service", path.Dir(c.FullMethod())[1:]),
			zap.String("grpc.method", path.Base(c.FullMethod())),
		}
		if peer, ok := peer.FromContext(ctx); ok {
			fields = append(fields, zap.String("peer.address", peer.Addr.String()))
		}
		return &grpcProxyLogger{
			logger: lg,
			fields: fields,
		}, ctx
	}
}
