# etcd/tools/rw-benchmark

`etcd/tools/rw-benchmark` is the mixed read/write performance evaluation tool for etcd clusters.

## Execute

### Benchmark
To get a mixed read/write performance evaluation result:
```sh
# run with default configurations and specify the working directory
./rw-benchmark.sh -w ${WORKING_DIR}
```
`rw-benchmark.sh` will automatically use the etcd binary compiled under `etcd/bin/tools` directory.

Note: the result csv file will be saved to current working directory. The working directory is where etcd database is saved. The working directory is designed for scenarios where a different mounted disk is preferred.

### Plot Graphs
The tool `rw-benchmark` can generate an HTML page including all the line charts based on the benchmark result csv files. See usage below,
```sh
$ ./rw-benchmark  -h
rw-benchmark is a tool for visualize etcd read-write performance result.

Usage:
    rw-benchmark [options] result-file1.csv [result-file2.csv]

Additional options:
    -legend: Comma separated names of legends, such as "main,pr", defaults to "1" or "1,2" depending on the number of CSV files provided.
    -layout: The layout of the page, valid values: none, center and flex, defaults to "flex".
    -width: The width(pixel) of the each line chart, defaults to 600.
    -height: The height(pixel) of the each line chart, defaults to 300.
    -o: The HTML file name in which the benchmark data will be rendered, defaults to "rw_benchmark.html".
    -h: Print usage.
```

See examples below,
```sh
# To generate a HTML page with each chart including one pair of read & write
# benchmark results from one data csv file.
./rw-benchmark ${FIRST_CSV_FILE} 

# To generate a HTML page with each chart including two pair of read & write 
# benchmark results from two data csv files respectively.
./rw-benchmark ${FIRST_CSV_FILE} ${SECOND_CSV_FILE}

# Set the legend to "main,dev"
./rw-benchmark -legend "main,dev" ${FIRST_CSV_FILE} ${SECOND_CSV_FILE}

# Set the width and height of each line chart to 800 and 400px respectively
./rw-benchmark -width 800 -height 400 ${FIRST_CSV_FILE} ${SECOND_CSV_FILE}
```

The read QPS is displayed as <span style="color:blue">blue</span>, and write QPS is displayed as <span style="color:red">red</span>. 
The data in the second CSV file is rendered as dashed line if present. See example in [example/rw_benchmark.html](example/rw_benchmark.html).
Note each line in the line chart can be hidden or displayed by clicking on the related legend.
