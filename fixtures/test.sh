RAFT_NAME=alpha RAFT_SEED=41 raft serve -u 10s -c config.json -o metrics.json &
RAFT_NAME=bravo RAFT_SEED=42 raft serve -u 10s -c config.json -o metrics.json &
RAFT_NAME=charlie RAFT_SEED=43 raft serve -u 10s -c config.json -o metrics.json &
sleep 2
raft bench -c config.json -n 5 -r 1000 -o metrics.json
wait
