package cmd

import (
	"context"
	//"crypto/rand"
	"fmt"
	//"math/big"
	mrand "math/rand"
	"sync"
	"time"

	mp_dp_proto "lwi-channel-common/pkg/mp-dp-proto"

	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"
)

const twoPcKeySizeBytes = 256

var (
	twoPcTotal        int
	twoPcChannelStart int
	twoPcChannelCount int
	twoPcVersionStart uint64
	twoPcDialTimeout  time.Duration

	twoPcRngMu sync.Mutex
	twoPcRng   = mrand.New(mrand.NewSource(time.Now().UnixNano()))
)

var TwoPcPutCmd = &cobra.Command{
	Use:   "2pc-put",
	Short: "Write random 2pc_round protobuf messages to etcd",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTwoPcPut(cmd.Context())
	},
}

func init() {
	TwoPcPutCmd.Flags().IntVar(&twoPcTotal, "total", 1000, "number of puts to perform")
	TwoPcPutCmd.Flags().IntVar(&twoPcChannelStart, "channel-start", 1, "starting channel id")
	TwoPcPutCmd.Flags().IntVar(&twoPcChannelCount, "channel-count", 5, "number of channel ids to cycle through")
	TwoPcPutCmd.Flags().Uint64Var(&twoPcVersionStart, "version-start", 1, "starting version")
	TwoPcPutCmd.Flags().DurationVar(&twoPcDialTimeout, "dial-timeout", 5*time.Second, "etcd dial timeout")

	RootCmd.AddCommand(TwoPcPutCmd)
}

func runTwoPcPut(ctx context.Context) error {
	if twoPcTotal <= 0 {
		return fmt.Errorf("total must be > 0")
	}
	if twoPcChannelCount <= 0 {
		return fmt.Errorf("channel-count must be > 0")
	}
	if len(endpoints) == 0 {
		return fmt.Errorf("no endpoints configured")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: twoPcDialTimeout,
		DialOptions: noProxyDialOptions(),
	})
	if err != nil {
		return fmt.Errorf("create etcd client: %w", err)
	}
	defer cli.Close()

	revisionTracker := newCASRevisionTracker()
	for i := 0; i < twoPcTotal; i++ {
		channelID := twoPcChannelStart + (i % twoPcChannelCount)
		version := twoPcVersionStart + uint64(i)

		payload, key, err := generateTwoPcRound(channelID, version)
		if err != nil {
			return err
		}

		if err = revisionTracker.put(ctx, cli, key, string(payload)); err != nil {
			return fmt.Errorf("cas put key %q: %w", key, err)
		}
	}

	return nil
}

func generateTwoPcRound(channelID int, version uint64) ([]byte, string, error) {
	op := twoPcRandomOperation()
	phase := twoPcRandomPhase()
	protocol := twoPcRandomProtocol()

	msg := &mp_dp_proto.TwoPcRound{
		SpecVersion:       ptr(version),
		ChannelInstanceId: ptr(twoPcRandomString(32)),
		Operation:         &op,
		Phase:             &phase,
		DpStates:          twoPcDPStatesForKiltNodes(),
		Vip:               ptr(kiltEtcdNodeIP(channelID + int(version))),
		VipPort:           ptr(uint32(1000 + twoPcRandIntn(50000))),
		VipProtocol:       &protocol,
		Backends: []*mp_dp_proto.Backend{
			twoPcRandomBackend(),
			twoPcRandomBackend(),
		},
		HcPolicies: []*mp_dp_proto.HealthPolicy{
			twoPcRandomHealthPolicy(),
		},
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, "", fmt.Errorf("marshal TwoPcRound: %w", err)
	}

	key := twoPcBuildKey(channelID, version)
	if len(key) != twoPcKeySizeBytes {
		return nil, "", fmt.Errorf("generated key length %d, want %d", len(key), twoPcKeySizeBytes)
	}

	return payload, key, nil
}

func twoPcDPStatesForKiltNodes() map[string]mp_dp_proto.DPState {
	states := make(map[string]mp_dp_proto.DPState, len(kiltEtcdNodes))
	for _, node := range kiltEtcdNodes {
		states[node.name] = twoPcRandomDPState()
	}
	return states
}

func twoPcBuildKey(channelID int, version uint64) string {
	prefix := benchmarkKey(fmt.Sprintf("%d/STATE/%d/", channelID, version))
	if len(prefix) >= twoPcKeySizeBytes {
		return prefix[:twoPcKeySizeBytes]
	}
	return prefix + twoPcRandomString(twoPcKeySizeBytes-len(prefix))
}

func twoPcRandomOperation() mp_dp_proto.Operation {
	switch twoPcRandIntn(4) {
	case 0:
		return mp_dp_proto.Operation_OPERATION_ENSURE_CHANNEL
	case 1:
		return mp_dp_proto.Operation_OPERATION_DELETE_CHANNEL
	case 2:
		return mp_dp_proto.Operation_OPERATION_ENSURE_BACKEND
	default:
		return mp_dp_proto.Operation_OPERATION_DELETE_BACKEND
	}
}

func twoPcRandomPhase() mp_dp_proto.Phase {
	switch twoPcRandIntn(5) {
	case 0:
		return mp_dp_proto.Phase_PHASE_VOTING
	case 1:
		return mp_dp_proto.Phase_PHASE_COMMIT
	case 2:
		return mp_dp_proto.Phase_PHASE_ABORT
	case 3:
		return mp_dp_proto.Phase_PHASE_ACTIVE
	default:
		return mp_dp_proto.Phase_PHASE_FAILED
	}
}

func twoPcRandomDPState() mp_dp_proto.DPState {
	switch twoPcRandIntn(4) {
	case 0:
		return mp_dp_proto.DPState_DP_STATE_NIL
	case 1:
		return mp_dp_proto.DPState_DP_STATE_YES
	case 2:
		return mp_dp_proto.DPState_DP_STATE_NO
	default:
		return mp_dp_proto.DPState_DP_STATE_ACK
	}
}

func twoPcRandomProtocol() mp_dp_proto.Protocol {
	if twoPcRandIntn(2) == 0 {
		return mp_dp_proto.Protocol_CHANNEL_PROTOCOL_TCP
	}
	return mp_dp_proto.Protocol_CHANNEL_PROTOCOL_UDP
}

func twoPcRandomBackend() *mp_dp_proto.Backend {
	node := kiltEtcdNodes[twoPcRandIntn(len(kiltEtcdNodes))]
	return &mp_dp_proto.Backend{
		ChannelTaskInstanceId: ptr(node.name + "-" + twoPcRandomString(8)),
		HcPolicyId:            ptr(twoPcRandomString(8)),
		Ip:                    ptr(node.ip),
		Port:                  ptr(uint32(1000 + twoPcRandIntn(50000))),
		Weight:                ptr(uint32(1 + twoPcRandIntn(127))),
		AcceptNewConnections:  ptr(twoPcRandIntn(2) == 0),
	}
}

func twoPcRandomHealthPolicy() *mp_dp_proto.HealthPolicy {
	return &mp_dp_proto.HealthPolicy{
		PolicyId:          ptr(twoPcRandomString(8)),
		IntervalMsecs:     ptr(uint32(5000)),
		TimeoutMsecs:      ptr(uint32(1000)),
		HealthyTries:      ptr(uint32(3)),
		UnhealthyTries:    ptr(uint32(3)),
		FailOpenThreshold: ptr(uint32(50)),
		PolicyType: &mp_dp_proto.HealthPolicy_Http{
			Http: &mp_dp_proto.HealthPolicyHttp{
				UriPath: ptr("/health"),
				Port:    ptr(uint32(8080)),
			},
		},
	}
}

func twoPcRandomString(n int) string {
	if n <= 0 {
		return ""
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)

	twoPcRngMu.Lock()
	defer twoPcRngMu.Unlock()

	for i := range b {
		b[i] = charset[twoPcRng.Intn(len(charset))]
	}

	return string(b)
}

func twoPcRandIntn(max int) int {
	if max <= 0 {
		return 0
	}

	twoPcRngMu.Lock()
	defer twoPcRngMu.Unlock()

	return twoPcRng.Intn(max)
}

func ptr[T any](v T) *T {
	return &v
}
