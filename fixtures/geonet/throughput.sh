#!/bin/bash

# Location of the results and configs
RESULTS="data/"
CONFIGS="configs/*.json"

# Describe the time format
TIMEFORMAT="experiment completed in %2lR"

time {
    # Step One: Ensure that the package is up to date
    fab update
    fab version

    # Step Two: Clean out any old results that still linger
    fab cleanup

    # Loop through all the configurations
    for conf in $CONFIGS; do
        if [[ -f $conf ]]; then

          # Put the config to the server
          fab putconfig:$conf

          # Step Four: # Run the throughput benchmark
          # for (( J=1; J<=17; J++ )); do
          fab "bench:$conf,17"

          # Fetch the results and cleanup
          fab getmerge:path=$RESULTS
          fab cleanup
          # done

        fi
    done

}
