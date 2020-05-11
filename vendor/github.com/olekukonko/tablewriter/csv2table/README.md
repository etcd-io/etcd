ASCII Table Writer Tool
=========

Generate ASCII table on the fly via command line ...  Installation is simple as

#### Get Tool

    go get  github.com/olekukonko/tablewriter/csv2table

#### Install Tool

    go install  github.com/olekukonko/tablewriter/csv2table


#### Usage

    csv2table -f test.csv

#### Support for Piping

    cat test.csv | csv2table -p=true

#### Output

```
+------------+-----------+---------+
| FIRST NAME | LAST NAME |   SSN   |
+------------+-----------+---------+
|    John    |   Barry   |  123456 |
|   Kathy    |   Smith   |  687987 |
|    Bob     | McCornick | 3979870 |
+------------+-----------+---------+
```

#### Another Piping with Header set to `false`

    echo dance,with,me | csv2table -p=true -h=false

#### Output

    +-------+------+-----+
    | dance | with | me  |
    +-------+------+-----+
