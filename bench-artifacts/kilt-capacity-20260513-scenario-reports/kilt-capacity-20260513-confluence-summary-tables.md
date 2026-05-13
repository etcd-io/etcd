# KILT Capacity 20260513 Summary Tables

Source: `bench-artifacts/kilt-capacity-20260513-scenario-reports/full/kilt-capacity-20260513-full-summary.md`

## Put Ramp Summary

| Rate | Requests | Req/sec | Avg | P50 | P90 | P99 | P999 | Max | WAL P99 | Backend P99 | Pending Max | DB Growth | CPU Max |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 500/s | 150,000 | 500.00 | 2.21 ms | 1.94 ms | 3.44 ms | 4.80 ms | 6.63 ms | 22.28 ms | 3.78 ms | 7.48 ms | 2 | 119.96 MiB | 28.9% |
| 1,000/s | 300,000 | 999.99 | 3.38 ms | 3.11 ms | 5.31 ms | 7.88 ms | 11.22 ms | 43.62 ms | 3.83 ms | 7.77 ms | 3 | 238.20 MiB | 33.3% |
| 2,000/s | 600,000 | 1,999.40 | 5.43 ms | 5.13 ms | 8.03 ms | 11.38 ms | 19.64 ms | 216.35 ms | 3.88 ms | 7.86 ms | 7 | 474.42 MiB | 15.4% |
| 4,000/s | 1,200,000 | 3,999.99 | 6.76 ms | 6.66 ms | 9.49 ms | 13.12 ms | 18.74 ms | 229.90 ms | 3.91 ms | 7.92 ms | 17 | 947.49 MiB | 18.9% |
| 8,000/s | 2,400,000 | 8,000.42 | 7.91 ms | 7.81 ms | 10.86 ms | 16.10 ms | 22.44 ms | 170.83 ms | 3.93 ms | 11.63 ms | 59 | 1.85 GiB | 25.3% |
| 16,000/s | 4,800,000 | 16,001.71 | 9.73 ms | 9.04 ms | 14.53 ms | 23.83 ms | 41.66 ms | 242.79 ms | 3.96 ms | 15.43 ms | 347 | 3.70 GiB | 44.3% |

## Put Network Summary

| Rate | Net TX | Net RX | TX Bandwidth | RX Bandwidth | TX PPS | RX PPS | Packet Drops |
|---|---:|---:|---:|---:|---:|---:|---:|
| 500/s | 229.17 MiB | 51.02 MiB | 781.90 KiB/s | 174.07 KiB/s | 2,532.09/s | 1,532.32/s | - |
| 1,000/s | 456.64 MiB | 97.07 MiB | 1.52 MiB/s | 331.22 KiB/s | 5,034.19/s | 3,034.36/s | - |
| 2,000/s | 911.45 MiB | 189.29 MiB | 3.04 MiB/s | 645.63 KiB/s | 10,031.99/s | 6,037.23/s | - |
| 4,000/s | 1.78 GiB | 373.70 MiB | 6.07 MiB/s | 1.24 MiB/s | 20,032.31/s | 12,040.09/s | - |
| 8,000/s | 3.55 GiB | 746.22 MiB | 12.12 MiB/s | 2.48 MiB/s | 40,009.87/s | 24,030.52/s | - |
| 16,000/s | 7.11 GiB | 1.45 GiB | 24.21 MiB/s | 4.95 MiB/s | 79,772.32/s | 48,005.60/s | - |

## Txn-Put Ramp Summary

| Rate | Requests | Req/sec | Avg | P50 | P90 | P99 | P999 | Max | WAL P99 | Backend P99 | Pending Max | DB Growth | CPU Max |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 500/s | 150,000 | 500.00 | 2.35 ms | 2.04 ms | 3.59 ms | 5.35 ms | 9.74 ms | 45.80 ms | 3.96 ms | 15.43 ms | 2 | 159.96 MiB | 11.1% |
| 1,000/s | 300,000 | 999.99 | 3.80 ms | 3.49 ms | 5.92 ms | 9.22 ms | 12.91 ms | 32.28 ms | 3.95 ms | 15.33 ms | 4 | 318.25 MiB | 25.5% |
| 2,000/s | 217,069 | 723.56 | 5.91 ms | 5.77 ms | 8.58 ms | 12.14 ms | 17.79 ms | 37.25 ms | 3.94 ms | 15.21 ms | 8 | 254.99 MiB | 13.2% |
| 4,000/s | 1,200,000 | 3,999.97 | 7.18 ms | 6.94 ms | 9.75 ms | 13.79 ms | 24.85 ms | 481.88 ms | 3.94 ms | 14.14 ms | 87 | 1.24 GiB | 16.0% |
| 8,000/s | 2,400,000 | 8,000.09 | 7.98 ms | 7.79 ms | 11.19 ms | 16.86 ms | 23.52 ms | 80.38 ms | 3.94 ms | 14.29 ms | 45 | 2.41 GiB | 24.2% |
| 16,000/s | 1,255,384 | 4,182.96 | 10.54 ms | 9.56 ms | 15.56 ms | 25.61 ms | 174.25 ms | 284.49 ms | 3.94 ms | 14.50 ms | 129 | 1.31 GiB | 53.5% |

## Txn-Put Network Summary

| Rate | Net TX | Net RX | TX Bandwidth | RX Bandwidth | TX PPS | RX PPS | Packet Drops |
|---|---:|---:|---:|---:|---:|---:|---:|
| 500/s | 278.91 MiB | 54.80 MiB | 887.60 KiB/s | 174.39 KiB/s | 2,457.10/s | 1,524.59/s | - |
| 1,000/s | 572.53 MiB | 115.89 MiB | 1.57 MiB/s | 325.02 KiB/s | 4,389.36/s | 2,745.99/s | - |
| 2,000/s | 1.30 GiB | 183.57 MiB | 3.08 MiB/s | 436.21 KiB/s | 7,327.36/s | 4,147.07/s | - |
| 4,000/s | 2.17 GiB | 403.60 MiB | 4.65 MiB/s | 865.09 KiB/s | 13,098.79/s | 8,080.66/s | - |
| 8,000/s | 4.36 GiB | 871.49 MiB | 6.39 MiB/s | 1.25 MiB/s | 17,967.17/s | 11,104.98/s | - |
| 16,000/s | 8.31 GiB | 1.14 GiB | 11.57 MiB/s | 1.59 MiB/s | 30,205.51/s | 16,393.58/s | - |

## Watch-Latency Ramp Summary

| Rate | Operation | Requests | Req/sec | Avg | P50 | P90 | P99 | P999 | Max | WAL P99 | Backend P99 | Pending Max | DB Growth | CPU Max |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 500/s | watch-latency-put | 150,000 | 474.99 | 2.10 ms | 1.81 ms | 3.37 ms | 5.61 ms | 7.31 ms | 215.70 ms | 3.94 ms | 14.02 ms | 1 | 118.73 MiB | 21.1% |
| 500/s | watch-latency-watch | 7,500,000 | 23,706.29 | 0.08 ms | 0.01 ms | 0.17 ms | 1.02 ms | 2.52 ms | 213.90 ms | 3.94 ms | 14.02 ms | 1 | 118.73 MiB | 21.1% |
| 1,000/s | watch-latency-put | 300,000 | 502.50 | 1.99 ms | 1.76 ms | 3.22 ms | 3.94 ms | 4.97 ms | 67.86 ms | 3.94 ms | 13.80 ms | 1 | 237.37 MiB | 12.8% |
| 1,000/s | watch-latency-watch | 15,000,000 | 25,075.18 | 0.10 ms | 0.05 ms | 0.21 ms | 1.16 ms | 2.46 ms | 13.69 ms | 3.94 ms | 13.80 ms | 1 | 237.37 MiB | 12.8% |
| 2,000/s | watch-latency-put | 600,000 | 494.32 | 2.02 ms | 1.78 ms | 3.27 ms | 4.04 ms | 5.17 ms | 216.59 ms | 3.93 ms | 13.68 ms | 1 | 474.77 MiB | 15.8% |
| 2,000/s | watch-latency-watch | 30,000,000 | 24,668.48 | 0.10 ms | 0.04 ms | 0.21 ms | 1.19 ms | 2.52 ms | 223.04 ms | 3.93 ms | 13.68 ms | 1 | 474.77 MiB | 15.8% |

## Watch-Latency Network Summary

| Rate | Net TX | Net RX | TX Bandwidth | RX Bandwidth | TX PPS | RX PPS | Packet Drops |
|---|---:|---:|---:|---:|---:|---:|---:|
| 500/s | 291.76 MiB | 4.91 GiB | 942.97 KiB/s | 15.87 MiB/s | 6,812.37/s | 6,554.38/s | - |
| 1,000/s | 579.90 MiB | 9.81 GiB | 991.14 KiB/s | 16.77 MiB/s | 7,113.13/s | 6,834.70/s | - |
| 2,000/s | 1.13 GiB | 19.63 GiB | 974.44 KiB/s | 16.50 MiB/s | 6,988.39/s | 6,670.73/s | - |

## Mixed Ramp Summary

| Rate | Operation | Requests | Req/sec | Avg | P50 | P90 | P99 | P999 | Max | WAL P99 | Backend P99 | Pending Max | DB Growth | CPU Max |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 500/s | watch-latency-put | 150,000 | 490.78 | 2.04 ms | 1.79 ms | 3.33 ms | 4.23 ms | 5.43 ms | 72.58 ms | 3.91 ms | 13.43 ms | 1 | 126.68 MiB | 13.4% |
| 500/s | watch-latency-range | 15,000 | 48.99 | 2.39 ms | 2.35 ms | 3.17 ms | 4.28 ms | 7.51 ms | 9.43 ms | 3.91 ms | 13.43 ms | 1 | 126.68 MiB | 13.4% |
| 500/s | watch-latency-watch | 7,500,000 | 24,492.54 | 0.11 ms | 0.04 ms | 0.22 ms | 1.46 ms | 2.74 ms | 17.88 ms | 3.91 ms | 13.43 ms | 1 | 126.68 MiB | 13.4% |
| 1,000/s | watch-latency-put | 300,000 | 498.66 | 2.00 ms | 1.77 ms | 3.24 ms | 4.03 ms | 5.19 ms | 24.53 ms | 3.91 ms | 13.36 ms | 1 | 245.34 MiB | 13.1% |
| 1,000/s | watch-latency-range | 30,000 | 49.77 | 2.25 ms | 2.21 ms | 3.05 ms | 4.12 ms | 16.68 ms | 20.34 ms | 3.91 ms | 13.36 ms | 1 | 245.34 MiB | 13.1% |
| 1,000/s | watch-latency-watch | 15,000,000 | 24,884.41 | 0.11 ms | 0.05 ms | 0.22 ms | 1.40 ms | 2.66 ms | 20.33 ms | 3.91 ms | 13.36 ms | 1 | 245.34 MiB | 13.1% |
| 2,000/s | watch-latency-put | 600,000 | 489.18 | 2.04 ms | 1.80 ms | 3.27 ms | 4.09 ms | 5.44 ms | 83.93 ms | 3.90 ms | 13.24 ms | 1 | 482.67 MiB | 20.3% |
| 2,000/s | watch-latency-range | 60,000 | 48.82 | 1.93 ms | 1.82 ms | 2.63 ms | 3.69 ms | 22.49 ms | 34.70 ms | 3.90 ms | 13.24 ms | 1 | 482.67 MiB | 20.3% |
| 2,000/s | watch-latency-watch | 30,000,000 | 24,410.03 | 0.09 ms | 0.01 ms | 0.17 ms | 1.23 ms | 2.54 ms | 19.04 ms | 3.90 ms | 13.24 ms | 1 | 482.67 MiB | 20.3% |

## Mixed Network Summary

| Rate | Net TX | Net RX | TX Bandwidth | RX Bandwidth | TX PPS | RX PPS | Packet Drops |
|---|---:|---:|---:|---:|---:|---:|---:|
| 500/s | 302.14 MiB | 5.79 GiB | 947.05 KiB/s | 18.14 MiB/s | 6,609.07/s | 6,605.91/s | - |
| 1,000/s | 592.85 MiB | 11.57 GiB | 973.39 KiB/s | 19.00 MiB/s | 6,856.13/s | 6,881.82/s | - |
| 2,000/s | 1.16 GiB | 23.15 GiB | 971.43 KiB/s | 18.96 MiB/s | 6,948.27/s | 6,822.43/s | - |

## Range Ramp Summary

| Rate | Requests | Req/sec | Avg | P50 | P90 | P99 | P999 | Max | WAL P99 | Backend P99 | Pending Max | DB Growth | CPU Max |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 500/s | 150,000 | 500.00 | 2.62 ms | 2.28 ms | 4.31 ms | 6.13 ms | 7.37 ms | 9.96 ms | 3.89 ms | 12.98 ms | 0 | 0 B | 11.8% |
| 1,000/s | 300,000 | 1,000.00 | 2.38 ms | 2.12 ms | 3.88 ms | 5.42 ms | 6.55 ms | 9.26 ms | 3.89 ms | 12.98 ms | 0 | 0 B | 25.0% |
| 2,000/s | 600,000 | 2,000.02 | 2.44 ms | 2.18 ms | 3.75 ms | 5.52 ms | 7.39 ms | 19.16 ms | 3.89 ms | 12.98 ms | 0 | 0 B | 27.9% |
| 4,000/s | 1,200,000 | 4,000.20 | 2.77 ms | 2.20 ms | 4.88 ms | 8.47 ms | 14.56 ms | 59.51 ms | 3.89 ms | 12.98 ms | 0 | 0 B | 25.3% |
| 8,000/s | 2,400,000 | 7,926.87 | 67.47 ms | 2.34 ms | 218.33 ms | 1037.46 ms | 1932.41 ms | 52692.97 ms | 3.89 ms | 12.98 ms | 0 | 0 B | 44.1% |

## Range Network Summary

| Rate | Net TX | Net RX | TX Bandwidth | RX Bandwidth | TX PPS | RX PPS | Packet Drops |
|---|---:|---:|---:|---:|---:|---:|---:|
| 500/s | 97.49 MiB | 8.84 GiB | 332.53 KiB/s | 30.14 MiB/s | 3,733.14/s | 5,469.27/s | - |
| 1,000/s | 193.52 MiB | 17.67 GiB | 660.27 KiB/s | 60.28 MiB/s | 7,445.08/s | 10,972.00/s | - |
| 2,000/s | 387.57 MiB | 35.78 GiB | 1.29 MiB/s | 122.05 MiB/s | 14,981.13/s | 23,123.42/s | - |
| 4,000/s | 772.79 MiB | 70.65 GiB | 2.57 MiB/s | 241.02 MiB/s | 29,906.32/s | 43,965.27/s | - |
| 8,000/s | 1.54 GiB | 141.18 GiB | 5.19 MiB/s | 476.93 MiB/s | 59,884.08/s | 80,683.42/s | - |
