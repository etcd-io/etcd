# etcd/tools/rw-heatmaps

`etcd/tools/rw-heatmaps` is the mixed read/write performance evaluation tool for etcd clusters.

## Installation

Install the tool by running the following command from the etcd source directory.

```sh
  $ go install -v ./tools/rw-heatmaps
```

The installation will place executables in the $GOPATH/bin. If $GOPATH environment variable is not set, the tool will be installed into the $HOME/go/bin. You can also find out the installed location by running the following command from the etcd source directory. Make sure that $PATH is set accordingly in your environment.

```sh
  $ go list -f "{{.Target}}" ./tools/rw-heatmaps
```

Alternatively, instead of installing the tool, you can use it by simply running the following command from the etcd source directory.

```sh
  $ go run ./tools/rw-heatmaps
```

## Execute

### Benchmark

To get a mixed read/write performance evaluation result:
```sh
  # run with default configurations and specify the working directory
  $ ./rw-benchmark.sh -w ${WORKING_DIR}
```
`rw-benchmark.sh` will automatically use the etcd binary compiled under `etcd/bin/` directory.

Note: the result CSV file will be saved to current working directory. The working directory is where etcd database is saved. The working directory is designed for scenarios where a different mounted disk is preferred.

### Plot Graphs

To generate two images (read and write) based on the benchmark result CSV file:

```sh
  # to generate a pair of read & write images from one data csv file
  $ rw-heatmaps ${CSV_FILE} -t ${IMAGE_TITLE} -o ${OUTPUT_IMAGE_NAME}
```

To generate two images (read and write) showing the performance difference from two result CSV files:

```sh
  # to generate a pair of read & write images from one data csv file
  $ rw-heatmaps ${CSV_FILE1} ${CSV_FILE2} -t ${IMAGE_TITLE} -o ${OUTPUT_IMAGE_NAME}
```

To see the available options use the `--help` option.

```sh
  $ rw-heatmaps --help

rw-heatmaps is a tool to generate read/write heatmaps images for etcd3.

Usage:
  rw-heatmaps [input file(s) in csv format] [flags]

Flags:
  -h, --help                       help for rw-heatmaps
  -f, --output-format string       output image file format (default "jpg")
  -o, --output-image-file string   output image filename (required)
  -t, --title string               plot graph title (required)
      --zero-centered              plot the improvement graph with white color represents 0.0 (default true)
```
