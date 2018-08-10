# Raft Example

This folder contains an example configuration and bash scripts to create the environment for a three-replica consensus quorum.

To get started, in three separate terminal windows, source each of the environments, `alpha.sh`, `bravo.sh`, and `charlie.sh` -- this will create three named replicas and export variables with commands. Once the environment has been set up run:

```
$ $SERVE
[raft] 07:55:16.822470 raft replica with 2 remote peers created
[raft] 07:55:16.822630 alpha is now running
```

In each of the three terminals, this will run the alpha, bravo, and charlie replicas respectively. 
