module wcfvol/etcd

require (
	cloud.google.com/go v0.35.1 // indirect
	dmitri.shuralyov.com/app/changes v0.0.0-20181114035150-5af16e21babb // indirect
	dmitri.shuralyov.com/service/change v0.0.0-20190124041518-7c09d163af9b // indirect
	git.apache.org/thrift.git v0.12.0 // indirect
	github.com/Shopify/sarama v1.20.1 // indirect
	github.com/StackExchange/wmi v0.0.0-20181212234831-e0a55b97c705 // indirect
	github.com/bgentry/speakeasy v0.1.0
	github.com/blacktear23/go-proxyprotocol v0.0.0-20180807104634-af7a81e8dd0d // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/coreos/bbolt v1.3.0 // indirect
	github.com/coreos/etcd v3.3.11+incompatible // indirect
	github.com/coreos/go-semver v0.2.0
	github.com/coreos/go-systemd v0.0.0-20181031085051-9002847aa142
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/cznic/sortutil v0.0.0-20181122101858-f5f958428db8 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.7.0 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logfmt/logfmt v0.4.0 // indirect
	github.com/go-ole/go-ole v1.2.2 // indirect
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/gogo/protobuf v1.2.0
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff
	github.com/golang/lint v0.0.0-20181217174547-8f45f776aaf1 // indirect
	github.com/golang/protobuf v1.2.0
	github.com/google/btree v0.0.0-20180813153112-4030bb1f1f0c
	github.com/google/pprof v0.0.0-20190109223431-e84dfd68c163 // indirect
	github.com/google/uuid v1.1.0
	github.com/googleapis/gax-go v2.0.2+incompatible // indirect
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/gorilla/websocket v1.4.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20181110185634-c63ab54fda8f // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway v1.7.0
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jonboulle/clockwork v0.1.0
	github.com/juju/errors v0.0.0-20181118221551-089d3ea4e4d5 // indirect
	github.com/juju/loggo v0.0.0-20180524022052-584905176618 // indirect
	github.com/juju/testing v0.0.0-20180920084828-472a3e8b2073 // indirect
	github.com/klauspost/cpuid v1.2.0 // indirect
	github.com/kr/pty v1.1.3
	github.com/layeh/gopher-json v0.0.0-20190114024228-97fed8db8427 // indirect
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.4 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/microcosm-cc/bluemonday v1.0.2 // indirect
	github.com/myesui/uuid v1.0.0 // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/openzipkin/zipkin-go v0.1.5 // indirect
	github.com/pingcap/check v0.0.0-20190102082844-67f458068fc8 // indirect
	github.com/pingcap/kvproto v0.0.0-20190121084144-be0b43ee9241 // indirect
	github.com/pingcap/parser v0.0.0-20190123063514-f8c3dff115d5 // indirect
	github.com/pingcap/pd v2.1.2+incompatible // indirect
	github.com/pingcap/tidb v0.0.0-20181123114736-1e0876fe810a
	github.com/pingcap/tidb-tools v2.1.2+incompatible // indirect
	github.com/pingcap/tipb v0.0.0-20190107072121-abbec73437b7 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.0.0-20190115171406-56726106282f
	github.com/prometheus/common v0.1.0 // indirect
	github.com/prometheus/procfs v0.0.0-20190117184657-bf6a532e95b1 // indirect
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	github.com/shirou/gopsutil v2.18.12+incompatible // indirect
	github.com/shurcooL/go v0.0.0-20190121191506-3fef8c783dec // indirect
	github.com/shurcooL/gofontwoff v0.0.0-20181114050219-180f79e6909d // indirect
	github.com/shurcooL/highlight_diff v0.0.0-20181222201841-111da2e7d480 // indirect
	github.com/shurcooL/highlight_go v0.0.0-20181215221002-9d8641ddf2e1 // indirect
	github.com/shurcooL/home v0.0.0-20190120230144-cf17a69b0cc5 // indirect
	github.com/shurcooL/htmlg v0.0.0-20190120222857-1e8a37b806f3 // indirect
	github.com/shurcooL/httpfs v0.0.0-20181222201310-74dc9339e414 // indirect
	github.com/shurcooL/issues v0.0.0-20190119024938-0d39520a96b7 // indirect
	github.com/shurcooL/issuesapp v0.0.0-20181229001453-b8198a402c58 // indirect
	github.com/shurcooL/notifications v0.0.0-20181111060504-bcc2b3082a7a // indirect
	github.com/shurcooL/octicon v0.0.0-20181222203144-9ff1a4cf27f4 // indirect
	github.com/shurcooL/reactions v0.0.0-20181222204718-145cd5e7f3d1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/shurcooL/webdavfs v0.0.0-20181215192745-5988b2d638f6 // indirect
	github.com/sirupsen/logrus v1.3.0 // indirect
	github.com/soheilhy/cmux v0.1.4
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5
	github.com/uber/jaeger-client-go v2.15.1-0.20190116124224-6733ee486c78+incompatible // indirect
	github.com/uber/jaeger-lib v2.0.0+incompatible // indirect
	github.com/ugorji/go/codec v0.0.0-20181209151446-772ced7fd4c2
	github.com/urfave/cli v1.20.0
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2
	github.com/zalando/skipper v0.10.155 // indirect
	go.etcd.io/bbolt v1.3.0
	go.etcd.io/etcd v3.3.11+incompatible
	go.opencensus.io v0.19.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.9.1
	go4.org v0.0.0-20181109185143-00e24f1b2599 // indirect
	golang.org/x/build v0.0.0-20190124033707-36bd60da6b75 // indirect
	golang.org/x/crypto v0.0.0-20190123085648-057139ce5d2b
	golang.org/x/exp v0.0.0-20190123073158-f1c91bc264ca // indirect
	golang.org/x/net v0.0.0-20190119204137-ed066c81e75e
	golang.org/x/oauth2 v0.0.0-20190115181402-5dab4167f31c // indirect
	golang.org/x/sys v0.0.0-20190124100055-b90733256f2e // indirect
	golang.org/x/time v0.0.0-20181108054448-85acf8d2951c
	golang.org/x/tools v0.0.0-20190124004107-78ee07aa9465 // indirect
	google.golang.org/genproto v0.0.0-20190123001331-8819c946db44 // indirect
	google.golang.org/grpc v1.18.0
	gopkg.in/cheggaaa/pb.v1 v1.0.27
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/yaml.v2 v2.2.2
	honnef.co/go/tools v0.0.0-20190123181848-3f36ca0168d8 // indirect
	sourcegraph.com/sourcegraph/appdash v0.0.0-20190107175209-d9ea5c54f7dc // indirect
	sourcegraph.com/sqs/pbtypes v1.0.0 // indirect
)
