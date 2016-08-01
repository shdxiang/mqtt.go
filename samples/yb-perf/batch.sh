#!/usr/bin/env bash

for i in `seq 1 10`; do
	let subcnt=${i}*1000
	echo ${subcnt}
	go run yb-perf.go --broker tcp://127.0.0.1:1883 --pubcnt 1 --subcnt ${subcnt} 2>>yb-perf.res
done
