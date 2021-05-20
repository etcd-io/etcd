
## Graviton-based self-hosted github action worker

### Step 1. Create an EC2 instance with Graviton

Create an AWS Graviton-based EC2 instance. For example,

```
# or download from https://github.com/aws/aws-k8s-tester/releases
cd ${HOME}/go/src/github.com/aws/aws-k8s-tester
go install -v ./cmd/ec2-utils

# create arm64 AL2 instance
AWS_K8S_TESTER_EC2_ON_FAILURE_DELETE=true \
AWS_K8S_TESTER_EC2_LOG_COLOR=true \
AWS_K8S_TESTER_EC2_REGION=us-west-2 \
AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE=true \
AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE_KEEP=true \
AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE=true \
AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS=false \
AWS_K8S_TESTER_EC2_ASGS='{"GetRef.Name-arm64-al2-cpu":{"name":"GetRef.Name-arm64-al2-cpu","remote-access-user-name":"ec2-user","ami-type":"AL2_arm_64","image-id-ssm-parameter":"/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-arm64-gp2","instance-types":["m6g.xlarge"],"volume-size":40,"asg-min-size":1,"asg-max-size":1,"asg-desired-capacity":1}}' \
AWS_K8S_TESTER_EC2_ROLE_CREATE=true \
AWS_K8S_TESTER_EC2_VPC_CREATE=true \
ec2-utils create instances --enable-prompt=true --auto-path
```

### Step 2. Install github action on the host

SSH into the instance, and install the github action self-hosted runner (see [install scripts](https://github.com/etcd-io/etcd/settings/actions/runners/new?arch=arm64&os=linux)).

### Step 3. Configure github action on the host

SSH into the instance, and configure the github action self-hosted runner.

First, we need disable ICU install (see [actions/runner issue on ARM64](https://github.com/actions/runner/issues/629)):

```
sudo yum install -y patch
```

And write this bash script:

```
#!/bin/bash -e

patch -p1 <<ICU_PATCH
diff -Naur a/bin/Runner.Listener.runtimeconfig.json b/bin/Runner.Listener.runtimeconfig.json
--- a/bin/Runner.Listener.runtimeconfig.json	2020-07-01 02:21:09.000000000 +0000
+++ b/bin/Runner.Listener.runtimeconfig.json	2020-07-28 00:02:38.748868613 +0000
@@ -8,7 +8,8 @@
       }
     ],
     "configProperties": {
-      "System.Runtime.TieredCompilation.QuickJit": true
+      "System.Runtime.TieredCompilation.QuickJit": true,
+      "System.Globalization.Invariant": true
     }
   }
-}
\ No newline at end of file
+}
diff -Naur a/bin/Runner.PluginHost.runtimeconfig.json b/bin/Runner.PluginHost.runtimeconfig.json
--- a/bin/Runner.PluginHost.runtimeconfig.json	2020-07-01 02:21:22.000000000 +0000
+++ b/bin/Runner.PluginHost.runtimeconfig.json	2020-07-28 00:02:59.358680003 +0000
@@ -8,7 +8,8 @@
       }
     ],
     "configProperties": {
-      "System.Runtime.TieredCompilation.QuickJit": true
+      "System.Runtime.TieredCompilation.QuickJit": true,
+      "System.Globalization.Invariant": true
     }
   }
-}
\ No newline at end of file
+}
diff -Naur a/bin/Runner.Worker.runtimeconfig.json b/bin/Runner.Worker.runtimeconfig.json
--- a/bin/Runner.Worker.runtimeconfig.json	2020-07-01 02:21:16.000000000 +0000
+++ b/bin/Runner.Worker.runtimeconfig.json	2020-07-28 00:02:19.159028531 +0000
@@ -8,7 +8,8 @@
       }
     ],
     "configProperties": {
-      "System.Runtime.TieredCompilation.QuickJit": true
+      "System.Runtime.TieredCompilation.QuickJit": true,
+      "System.Globalization.Invariant": true
     }
   }
-}
\ No newline at end of file
+}
ICU_PATCH
```

And now patch the github action runner:

```
cd ${HOME}/actions-runner
bash patch.sh
```

```
patching file bin/Runner.Listener.runtimeconfig.json
patching file bin/Runner.PluginHost.runtimeconfig.json
patching file bin/Runner.Worker.runtimeconfig.json
```

And now configure:

```
sudo yum install -y wget
INSTANCE_ID=$(wget -q -O - http://169.254.169.254/latest/meta-data/instance-id)
echo ${INSTANCE_ID}

# get token from https://github.com/etcd-io/etcd/settings/actions/runners/new?arch=arm64&os=linux
cd ${HOME}/actions-runner
./config.sh \
--work "_work" \
--name ${INSTANCE_ID} \
--labels self-hosted,linux,ARM64,graviton2 \
--url https://github.com/etcd-io/etcd \
--token ...
```

And run:

```
# run this as a process in the terminal
cd ${HOME}/actions-runner
./run.sh

# or run this as a systemd service
cd ${HOME}/actions-runner
sudo ./svc.sh install
sudo ./svc.sh start
sudo ./svc.sh status
```

### Step 4. Create github action configuration

See https://github.com/etcd-io/etcd/pull/12928.

