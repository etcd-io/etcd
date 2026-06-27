# What is it?

`license-bill-of-materials` uses `go list` tool over a Go workspace to collect the dependencies
of a package or command, detect their license if any and match them against
well-known templates.

The output record format follows the JSON representation of the Go structs:

```go
type projectAndLicenses struct {
	Project  string    `json:"project"`
	Licenses []license `json:"licenses,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type license struct {
	Type       string  `json:"type,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}
```

The output might have three arrays of records:

- Matched/Guessed license projects
- Error projects

Miscategorized and error projects can be overridden with a file by using the `--override-file` flag.

Example file

```
[
	{
		"project": "k8s.io/kubernetes",
		"license": "Apache License 2.0",
	}
]
```

Example output of Kubernetes API server:

```bash
$ license-bill-of-materials k8s.io/kubernetes/cmd/kube-apiserver
```

```json
[
	{
		"project": "k8s.io/kubernetes",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "cloud.google.com/go",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9988925802879292
			}
		]
	},
	{
		"project": "github.com/Azure/azure-sdk-for-go",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9988925802879292
			}
		]
	},
	{
		"project": "github.com/Azure/go-autorest/autorest",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9680365296803652
			}
		]
	},
	{
		"project": "github.com/PuerkitoBio/purell",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9916666666666667
			}
		]
	},
	{
		"project": "github.com/PuerkitoBio/urlesc",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "github.com/aws/aws-sdk-go",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/beorn7/perks/quantile",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 0.9891304347826086
			}
		]
	},
	{
		"project": "github.com/coreos/etcd",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/coreos/go-oidc",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/coreos/go-systemd",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9966703662597114
			}
		]
	},
	{
		"project": "github.com/coreos/pkg",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/davecgh/go-spew/spew",
		"licenses": [
			{
				"type": "ISC License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/dgrijalva/jwt-go",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 0.9891304347826086
			}
		]
	},
	{
		"project": "github.com/docker/distribution",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/docker/engine-api/types",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9657534246575342
			}
		]
	},
	{
		"project": "github.com/docker/go-connections/nat",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9657534246575342
			}
		]
	},
	{
		"project": "github.com/docker/go-units",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9657534246575342
			}
		]
	},
	{
		"project": "github.com/docker/spdystream",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9680365296803652
			},
			{
				"type": "Open Software License 3.0",
				"confidence": 0.4606413994169096
			}
		]
	},
	{
		"project": "github.com/elazarl/go-bindata-assetfs",
		"licenses": [
			{
				"type": "BSD 2-clause \"Simplified\" License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/emicklei/go-restful",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/emicklei/go-restful-swagger12",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/evanphx/json-patch",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.979253112033195
			}
		]
	},
	{
		"project": "github.com/exponent-io/jsonpath",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/ghodss/yaml",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.8357142857142857
			}
		]
	},
	{
		"project": "github.com/go-ini/ini",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9966703662597114
			}
		]
	},
	{
		"project": "github.com/go-openapi/analysis",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/go-openapi/jsonpointer",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/go-openapi/jsonreference",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/go-openapi/loads",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/go-openapi/spec",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			},
			{
				"type": "The Unlicense",
				"confidence": 0.3422459893048128
			}
		]
	},
	{
		"project": "github.com/go-openapi/swag",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/gogo/protobuf",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9090909090909091
			}
		]
	},
	{
		"project": "github.com/golang/glog",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9966703662597114
			}
		]
	},
	{
		"project": "github.com/golang/groupcache/lru",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9966703662597114
			}
		]
	},
	{
		"project": "github.com/golang/protobuf",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.92
			}
		]
	},
	{
		"project": "github.com/google/gofuzz",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/gophercloud/gophercloud",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9680365296803652
			}
		]
	},
	{
		"project": "github.com/grpc-ecosystem/go-grpc-prometheus",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/grpc-ecosystem/grpc-gateway",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.979253112033195
			}
		]
	},
	{
		"project": "github.com/hashicorp/golang-lru",
		"licenses": [
			{
				"type": "Mozilla Public License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/hawkular/hawkular-client-go/metrics",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/howeyc/gopass",
		"licenses": [
			{
				"type": "ISC License",
				"confidence": 0.9850746268656716
			}
		]
	},
	{
		"project": "github.com/imdario/mergo",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "github.com/influxdata/influxdb",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/jmespath/go-jmespath",
		"licenses": [
			{
				"type": "The Unlicense",
				"confidence": 0.35294117647058826
			}
		]
	},
	{
		"project": "github.com/jonboulle/clockwork",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/juju/ratelimit",
		"licenses": [
			{
				"type": "GNU Lesser General Public License v3.0",
				"confidence": 0.9409937888198758
			}
		]
	},
	{
		"project": "github.com/mailru/easyjson",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 0.9891304347826086
			}
		]
	},
	{
		"project": "github.com/matttproud/golang_protobuf_extensions/pbutil",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9988925802879292
			}
		]
	},
	{
		"project": "github.com/mesos/mesos-go",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/mitchellh/mapstructure",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/mxk/go-flowrate/flowrate",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9707112970711297
			}
		]
	},
	{
		"project": "github.com/pborman/uuid",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "github.com/pkg/errors",
		"licenses": [
			{
				"type": "BSD 2-clause \"Simplified\" License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/prometheus/client_golang/prometheus",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/prometheus/client_model/go",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/prometheus/common",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/prometheus/procfs",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/rackspace/gophercloud",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9680365296803652
			}
		]
	},
	{
		"project": "github.com/robfig/cron",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 0.9946524064171123
			}
		]
	},
	{
		"project": "github.com/rubiojr/go-vhd/vhd",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/samuel/go-zookeeper/zk",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9916666666666667
			}
		]
	},
	{
		"project": "github.com/spf13/cobra",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 0.9573241061130334
			}
		]
	},
	{
		"project": "github.com/spf13/pflag",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "github.com/ugorji/go/codec",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 0.9946524064171123
			}
		]
	},
	{
		"project": "github.com/vmware/govmomi",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/vmware/govmomi/vim25/xml",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "github.com/vmware/photon-controller-go-sdk/photon",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "github.com/xanzy/go-cloudstack/cloudstack",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	},
	{
		"project": "golang.org/x/crypto",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "golang.org/x/net",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "golang.org/x/oauth2",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "golang.org/x/text",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "google.golang.org/api",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9663865546218487
			}
		]
	},
	{
		"project": "google.golang.org/api/googleapi/internal/uritemplates",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 0.9891304347826086
			}
		]
	},
	{
		"project": "google.golang.org/grpc",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.979253112033195
			}
		]
	},
	{
		"project": "gopkg.in/gcfg.v1",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9752066115702479
			}
		]
	},
	{
		"project": "gopkg.in/inf.v0",
		"licenses": [
			{
				"type": "BSD 3-clause \"New\" or \"Revised\" License",
				"confidence": 0.9752066115702479
			}
		]
	},
	{
		"project": "gopkg.in/natefinch/lumberjack.v2",
		"licenses": [
			{
				"type": "MIT License",
				"confidence": 1
			}
		]
	},
	{
		"project": "gopkg.in/yaml.v2",
		"licenses": [
			{
				"type": "GNU Lesser General Public License v3.0",
				"confidence": 0.9528301886792453
			},
			{
				"type": "MIT License",
				"confidence": 0.8975609756097561
			}
		]
	},
	{
		"project": "k8s.io/client-go",
		"licenses": [
			{
				"type": "Apache License 2.0",
				"confidence": 1
			}
		]
	}
]
```

# Where does it come from?

Both the code and reference data were directly ported from:

  [https://github.com/benbalter/licensee](https://github.com/benbalter/licensee)
