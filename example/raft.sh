#!/bin/bash
export RAFT="../cmd/raft/main.go"
export SERVE="go run $RAFT serve -c config.json"
export COMMIT="go run $RAFT commit -c config.json -k foo -v $(ts)"
export BENCH="go run $RAFT bench -c config.json -n 5 -r 5000"
