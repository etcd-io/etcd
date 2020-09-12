// Copyright 2016 The etcd Authors
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

package command

import (
	"fmt"
	"strconv"

	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/v3/qos"
	"go.etcd.io/etcd/v3/qos/qospb"

	"github.com/spf13/cobra"
)

// NewQoSCommand returns the cobra command for "qos".
func NewQoSCommand() *cobra.Command {
	qc := &cobra.Command{
		Use:   "qos <subcommand>",
		Short: "qos related commands",
	}
	qc.AddCommand(NewQoSEnableCommand())
	qc.AddCommand(NewQoSDisableCommand())
	qc.AddCommand(NewQoSAddCommand())
	qc.AddCommand(NewQoSDeleteCommand())
	qc.AddCommand(NewQoSUpdateCommand())
	qc.AddCommand(NewQoSGetCommand())
	qc.AddCommand(NewQoSListCommand())

	return qc
}

var helpdoc = `
add,update the QoS Rule into the store.
rule name is the unique name of a qos rule,you can get/update/delete qos rule by name.
rule type supports gRPCMethod,slowquery,traffic,custom.
subject supports gRPCMethod(such as Range,Put,Txn,Authenticate) and RangeKeyNum,DBUsedByte.
key prefix,if it is not empty, the rule will only take effect when the user key matches it..
qps indicates the maximum number of requests allowed to pass.
ratelimiter supports TokenBucket,MaxInflight,etc.
priority indicates the priority of this rule,when a request satisfies multiple rules, the rule with the highest priority is selected.
threshold indicates rule will take affect when subject(RangeKeyNum,DBUsedByte) > threshold,it runs more fastly than condition expr.
condition supports complex expression.it supports following key words,RangeKeyNum,DBUsedByte,gRPCMethod,key and prefix function.
RangeKeyNum: the total number of keys traversed by the query request
DBUsedByte: the bytes of backend db used.

for example:
$ etcdctl qos add/update rule-gRPCMethod-1 gRPCMethod Range /prefix 100 TokenBucket 1 0 ""
$ etcdctl qos add/update rule-slowquery-1 slowquery RangeKeyNum "" 100 MaxInflight 9 1000 ""
$ etcdctl qos add/update rule-custom-1 custom "" "" 100 TokenBucket 2 0  'DBUsedByte > 4096 && prefix(key,"/config") && gRPCMethod == "Put"'
`

// NewQoSAddCommand returns the cobra command for "qos add".
func NewQoSAddCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "add <rule name> <rule type> <subject> <key prefix> <qps> <ratelimiter> <priority(0-9)> <threshold> <condition>",
		Short: "adds a qos rule into the cluster",
		Long:  helpdoc,
		Run:   QoSAddCommandFunc,
	}

	return cc
}

// NewQoSDeleteCommand returns the cobra command for "qos delete".
func NewQoSDeleteCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "delete <rule-name>",
		Short: "deletes a qos rule from the cluster",

		Run: QoSDeleteCommandFunc,
	}

	return cc
}

// NewQoSUpdateCommand returns the cobra command for "qos update".
func NewQoSUpdateCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "update <rule name> <rule type> <subject> <key prefix> <qps> <ratelimiter> <priority(0-9)> <threshold> <condition>",
		Short: "updates a qos rule in the cluster",
		Long:  helpdoc,
		Run:   QoSUpdateCommandFunc,
	}

	return cc
}

// NewQoSListCommand returns the cobra command for "qos list".
func NewQoSListCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "list",
		Short: "Lists all qos rules in the cluster",
		Run:   QoSListCommandFunc,
	}

	return cc
}

// NewQoSEnableCommand enable qos feature.
func NewQoSEnableCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "enable",
		Short: "enable qos feature",
		Run:   QoSEnableCommandFunc,
	}
	return cc
}

// NewQoSDisableCommand disable qos feature.
func NewQoSDisableCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "disable",
		Short: "disable qos feature",
		Run:   QoSDisableCommandFunc,
	}
	return cc
}

// NewQoSGetCommand get qos rule.
func NewQoSGetCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "get <rule name>",
		Short: "get qos rule",
		Run:   QoSGetRuleCommandFunc,
	}
	return cc
}

// QoSEnableCommandFunc executes the "qos enable" command.
func QoSEnableCommandFunc(cmd *cobra.Command, args []string) {
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).QoS.QoSEnable(ctx, &pb.QoSEnableRequest{})
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.QoSEnable(*resp)
}

// QoSDisableCommandFunc executes the "qos disable" command.
func QoSDisableCommandFunc(cmd *cobra.Command, args []string) {
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).QoS.QoSDisable(ctx, &pb.QoSDisableRequest{})
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.QoSDisable(*resp)
}

// QoSAddCommandFunc executes the "qos add" command.
func QoSAddCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) <= 5 {
		err := fmt.Errorf("qos add requires at least four argument,please see help doc(-h)")
		ExitWithError(ExitBadArgs, err)
	}
	rule := parseQoSRuleFromArgv(args)
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).QoS.QoSRuleAdd(ctx, &pb.QoSRuleAddRequest{QosRule: rule})
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.QoSRuleAdd(*resp)
}

// QoSGetRuleCommandFunc executes the "qos get" command.
func QoSGetRuleCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		err := fmt.Errorf("qos get requires exactly one argument")
		ExitWithError(ExitBadArgs, err)
	}
	ruleName := args[0]
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).QoS.QoSRuleGet(ctx, &pb.QoSRuleGetRequest{RuleName: ruleName})
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.QoSRuleGet(*resp)
}

// QoSDeleteCommandFunc executes the "qos delete" command.
func QoSDeleteCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		err := fmt.Errorf("qos delete requires exactly one argument")
		ExitWithError(ExitBadArgs, err)
	}
	ruleName := args[0]
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).QoS.QoSRuleDelete(ctx, &pb.QoSRuleDeleteRequest{RuleName: ruleName})
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.QoSRuleDelete(*resp)
}

// QoSUpdateCommandFunc executes the "qos update" command.
func QoSUpdateCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) <= 5 {
		err := fmt.Errorf("qos update requires at least five argument,please see help doc(-h)")
		ExitWithError(ExitBadArgs, err)
	}
	rule := parseQoSRuleFromArgv(args)
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).QoS.QoSRuleUpdate(ctx, &pb.QoSRuleUpdateRequest{QosRule: rule})
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.QoSRuleUpdate(*resp)
}

// QoSListCommandFunc executes the "qos list" command.
func QoSListCommandFunc(cmd *cobra.Command, args []string) {
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).QoS.QoSRuleList(ctx, &pb.QoSRuleListRequest{})
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.QoSRuleList(*resp)

}

func parseInt(arg string) int {
	num, err := strconv.Atoi(arg)
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}
	return num
}

func parseQoSRuleFromArgv(args []string) *qospb.QoSRule {
	//add/update <rule name> <rule type> <subject> <key prefix> <qps> <ratelimiter> <priority(0-9)> <threshold> <condition>
	ruleName := args[0]
	if ruleType := qos.QoSRuleType(args[1]); ruleType != qos.RuleTypeGRPCMethod &&
		ruleType != qos.RuleTypeSlowQuery &&
		ruleType != qos.RuleTypeTraffic &&
		ruleType != qos.RuleTypeCustom {
		err := fmt.Errorf("rule type %s is invalid,please see qos help doc", args[1])
		ExitWithError(ExitBadArgs, err)
	}
	subjectName := args[2]
	prefix := args[3]
	qps := parseInt(args[4])
	rule := &qospb.QoSRule{RuleName: ruleName,
		RuleType: args[1],
		Subject:  &qospb.Subject{Name: subjectName, Prefix: prefix},
		Qps:      uint64(qps),
	}
	if len(args) > 5 {
		rule.Ratelimiter = args[5]
	} else {
		rule.Ratelimiter = qos.RateLimiterTokenBucket
	}
	if len(args) > 6 {
		rule.Priority = uint64(parseInt(args[6]))
		if rule.Priority < 0 || rule.Priority > 9 {
			err := fmt.Errorf("rule priority %d is invalid,it should be in [0,9]", rule.Priority)
			ExitWithError(ExitBadArgs, err)
		}
	}
	if len(args) > 7 {
		rule.Threshold = uint64(parseInt(args[7]))
	}
	if len(args) > 8 {
		rule.Condition = args[8]
	}
	return rule
}
