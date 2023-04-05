# etcd Robustness Testing

Purpose of etcd robustness tests is to validate that etcd upholds
[API guarantees] and [watch guarantees] under any condition or failure.

Robustness tests achieve that comparing etcd cluster behavior against a simplified model.
Multiple test encompass different etcd cluster setups, client traffic types and failures experienced by cluster.
During a single test we create a cluster and inject failures while sending and recording client traffic.
Correctness is validated by running collected history of client operations against the etcd model and a set of validators.
Upon failure tests generate a report that can be used to attribute whether failure was caused by bug in etcd or test framework. 

[API guarantees]: https://etcd.io/docs/latest/learning/api_guarantees/
[watch guarantees]: https://etcd.io/docs/latest/learning/api/#watch-streams

## Running locally 

1. Build etcd with failpoints
    ```bash
    make gofail-enable
    make build
    make gofail-disable
    ```
2. Run the tests

    ```bash
    make test-robustness
    ```
   
    Optionally you can pass environment variables:
    * `GO_TEST_FLAGS` - to pass additional arguments to `go test`. 
      It is recommended to run tests multiple times with failfast enabled. this can be done by setting `GO_TEST_FLAGS='--count=100 --failfast'`.
    * `EXPECT_DEBUG=true` - to get logs from the cluster.
    * `RESULTS_DIR` - to change location where results report will be saved.

## Analysing failure

If robustness tests fails we want to analyse the report to confirm if the issue is on etcd side. Location of this report
is included in test logs. One of log lines should look like:
```
    history.go:34: Model is not linearizable
    logger.go:130: 2023-03-18T12:18:03.244+0100 INFO    Saving member data dir  {"member": "TestRobustnessIssue14370-test-0", "path": "/tmp/TestRobustness_Issue14370/TestRobustnessIssue14370-test-0"}
    logger.go:130: 2023-03-18T12:18:03.244+0100 INFO    Saving watch responses  {"path": "/tmp/TestRobustness_Issue14370/TestRobustnessIssue14370-test-0/responses.json"}
    logger.go:130: 2023-03-18T12:18:03.247+0100 INFO    Saving watch events     {"path": "/tmp/TestRobustness_Issue14370/TestRobustnessIssue14370-test-0/events.json"}
    logger.go:130: 2023-03-18T12:18:03.248+0100 INFO    Saving operation history        {"path": "/tmp/TestRobustness_Issue14370/full-history.json"}
    logger.go:130: 2023-03-18T12:18:03.252+0100 INFO    Saving operation history        {"path": "/tmp/TestRobustness_Issue14370/patched-history.json"}
    logger.go:130: 2023-03-18T12:18:03.256+0100 INFO    Saving visualization    {"path": "/tmp/TestRobustness_Issue14370/history.html"}
```

Report includes multiple types of files:
* Member db files, can be used to verify disk/memory corruption.
* Watch responses saved as json, can be used to validate [watch guarantees].
* Operation history saved as both html visualization and a json, can be used to validate [API guarantees].

### Example analysis of linearization issue

Let's reproduce and analyse robustness test report for issue [#14370].
To reproduce the issue by yourself run `make test-robustness-issue14370`.
After a couple of tries robustness tests should fail with a log `Model is not linearizable` and save report locally.

Lineralization issues are easiest to analyse via history visualization. 
Open `/tmp/TestRobustness_Issue14370/history.html` file in your browser.
Jump to the error in linearization by clicking `[ jump to first error ]` on the top of the page.

You should see a graph similar to the one on the image below.
![issue14370](./issue14370.png)

Last correct request (connected with grey line) is a `Put` request that succeeded and got revision `168`.
All following requests are invalid (connected with red line) as they have revision `167`. 
Etcd guarantee that revision is non-decreasing, so this shows a bug in etcd as there is no way revision should decrease.
This is consistent with the root cause of [#14370] as it was issue with process crash causing last write to be lost.

[#14370]: https://github.com/etcd-io/etcd/issues/14370

### Example analysis of watch issue

Let's reproduce and analyse robustness test report for issue [#15271].
To reproduce the issue by yourself run `make test-robustness-issue15271`.
After a couple of tries robustness tests should fail with a logs `Broke watch guarantee` and save report locally.

Watch issues are easiest to analyse by reading the recorded watch history.
Watch history is recorded for each member separated in different subdirectory under `/tmp/TestRobustness_Issue15271/`
Open `responses.json` for member mentioned in log `Broke watch guarantee`. 
For example for member `TestRobustnessIssue15271-test-1` open `/tmp/TestRobustness_Issue15271/TestRobustnessIssue15271-test-1/responses.json`. 

Each line consists of json blob corresponding to single watch response observed by client.
Look for lines with `mod_revision` equal to revision mentioned in the first log with `Broke watch guarantee`
You should see two lines where the `mod_revision` decreases like ones below:
```
{"Header":{"cluster_id":12951239930360520062,"member_id":16914881897345358027,"revision":2574,"raft_term":2},"Events":[{"kv":{"key":"Ng==","create_revision":2303,"mod_revision":2574,"version":46,"value":"Mjg5OA=="}}],"CompactRevision":0,"Canceled":false,"Created":false}
{"Header":{"cluster_id":12951239930360520062,"member_id":16914881897345358027,"revision":7708,"raft_term":2},"Events":[{"kv":{"key":"NQ==","create_revision":5,"mod_revision":91,"version":10,"value":"MTAy"}}, ... }
```

Up to the first line the `mod_revision` of events within responses only increased up to a value of `2574`.
However, the following line includes an event with `mod_revision` equal `91`.
If you follow the `mod_revision` throughout the file you should notice that watch replayed revisions second time.
This is incorrect and breaks `Ordered` and `Unique` [watch guarantees].
This is consistent with the root cause of [#14370] where member reconnecting to cluster will incorrectly resend revisions.

[#15271]: https://github.com/etcd-io/etcd/issues/15271
