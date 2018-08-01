#!/bin/bash
# Runs the benchmarking script against a local quorum of 3 replicas.

# Location of the results
RESULTS="throughput.csv"

RUNS=12
MIN_CLIENTS=1
MAX_CLIENTS=42

# Describe the time format
TIMEFORMAT="experiment completed in %2lR"

time {
  # Write header to the output file
  if [ ! -f $RESULTS ]; then
    echo "clients,messages,duration,throughput,version" >> $RESULTS
  fi

  # Run the experiment for each clients $RUNS times
  for (( I=0; I<=$RUNS; I+=1 )); do

      # Run a benchmark from min clients to max clients
      for (( J=$MIN_CLIENTS; J<=$MAX_CLIENTS; J++ )); do

        # Set the uptime to ensure the benchmark can be successfully completed
        if [ $J -lt 36 ]; then
          UPTIME=10s
        else
          UPTIME=20s
        fi

        # Run
        RAFT_NAME=alpha RAFT_SEED=41 raft serve -u $UPTIME -c config.json &
        RAFT_NAME=bravo RAFT_SEED=42 raft serve -u $UPTIME -c config.json &
        RAFT_NAME=charlie RAFT_SEED=43 raft serve -u $UPTIME -c config.json &
        sleep 2
        raft bench -c config.json -n $J -r 1000 >> $RESULTS
        wait

      done
  done
}
