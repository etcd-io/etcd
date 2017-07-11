#!/bin/sh

exec tools/local-tester/bridge/bridge \
	-delay-accept		\
	-reset-listen		\
	-conn-fault-rate=0.25	\
	-immediate-close	\
	-blackhole		\
	-time-close		\
	-write-remote-only	\
	-read-remote-only	\
	-random-blackhole	\
	-corrupt-receive	\
	-corrupt-send		\
	-reorder		\
	$@
