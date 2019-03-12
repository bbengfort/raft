RAFT_NAME=alpha RAFT_SEED=41 raft serve -u 15s -c config.json -o metrics.json &
RAFT_NAME=bravo RAFT_SEED=42 raft serve -u 15s -c config.json -o metrics.json &
RAFT_NAME=charlie RAFT_SEED=43 raft serve -u 15s -c config.json -o metrics.json &
sleep 2
for (( I=0; I<10; I+=1 )); do 
    raft bench -c config.json --blast -r 1000 -a alpha
done 
wait
