# etcd/tools/rw-heatmaps

`etcd/tools/rw-heatmaps` is the mixed read/write performance evaluation tool for etcd clusters.

## Execute

### Benchmark
To get a mixed read/write performance evaluation result:
```sh
# run with default configurations and specify the working directory
./rw-benchmark.sh -w ${WORKING_DIR}
```
`rw-benchmark.sh` will automatically use the etcd binary compiled under `etcd/bin/` directory.

Note: the result csv file will be saved to current working directory. The working directory is where etcd database is saved. The working directory is designed for scenarios where a different mounted disk is preferred.

### Plot Graphs
To generate two images (read and write) based on the benchmark result csv file:
```sh
# to generate a pair of read & write images from one data csv file
./plot_data.py ${FIRST_CSV_FILE} -t ${IMAGE_TITLE} -o ${OUTPUT_IMAGE_NAME}


# to generate a pair of read & write images by comparing two data csv files
./plot_data.py ${FIRST_CSV_FILE} ${SECOND_CSV_FILE} -t ${IMAGE_TITLE} -o ${OUTPUT_IMAGE_NAME}
```
