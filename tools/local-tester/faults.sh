#!/bin/bash

PROCFILE="tools/local-tester/Procfile"

function wait_time {
	expr $RANDOM % 10 + 1
}

function cycle {
	for a; do
		echo "cycling $a"
		goreman -f $PROCFILE run stop $a || echo "could not stop $a"
		sleep `wait_time`s
		goreman -f $PROCFILE run restart $a || echo "could not restart $a"
	done
}

function cycle_members {
	cycle etcd1 etcd2 etcd3
}
function cycle_pbridge {
	cycle pbridge1 pbridge2 pbridge3
}
function cycle_cbridge {
	cycle cbridge1 cbridge2 cbridge3
}
function cycle_stresser {
	cycle stress-put
}

function kill_maj {
	idx="etcd"`expr $RANDOM % 3 + 1`
	idx2="$idx"
	while [ "$idx" == "$idx2" ]; do
		idx2="etcd"`expr $RANDOM % 3 + 1`
	done
	echo "kill majority $idx $idx2"
	goreman -f $PROCFILE run stop $idx || echo "could not stop $idx"
	goreman -f $PROCFILE run stop $idx2 || echo "could not stop $idx2"
	sleep `wait_time`s
	goreman -f $PROCFILE run restart $idx || echo "could not restart $idx"
	goreman -f $PROCFILE run restart $idx2 || echo "could not restart $idx2"
}

function kill_all {
	for a in etcd1 etcd2 etcd3; do
		goreman -f $PROCFILE run stop $a || echo "could not stop $a"
	done
	sleep `wait_time`s
	for a in etcd1 etcd2 etcd3; do
		goreman -f $PROCFILE run restart $a || echo "could not restart $a"
	done
}

function choose {
	faults=(cycle_members kill_maj kill_all cycle_pbridge cycle_cbridge cycle_stresser)
	fault=${faults[`expr $RANDOM % ${#faults[@]}`]}
	echo $fault
	$fault || echo "failed: $fault"
}

sleep 2s
while [ 1 ]; do
	choose
done
