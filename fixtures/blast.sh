#!/bin/bash
# Runs the blast benchmark for increasing numbers of requests.

# Location of the results
RESULTS="throughput.json"

RUNS=12
MIN_OPS=50
MAX_OPS=1000
OPS_INCR=50
UPTIME="6s"

# Describe the time format
TIMEFORMAT="experiment completed in %2lR"

time {
  # Run the experiment for each blast $RUNS times
  for (( I=0; I<$RUNS; I+=1 )); do

      # Run a benchmark from min ops to max ops by ops incr
      for (( J=$MIN_OPS; J<=$MAX_OPS; J=J+$OPS_INCR )); do
        # Run
        RAFT_NAME=alpha RAFT_SEED=41 raft serve -c config.json -u $UPTIME &
        RAFT_NAME=bravo RAFT_SEED=42 raft serve -c config.json -u $UPTIME &
        RAFT_NAME=charlie RAFT_SEED=43 raft serve -c config.json -u $UPTIME &
        sleep 3
        raft bench --blast -c config.json -r $J -a alpha >> $RESULTS
        wait

      done
  done
}
