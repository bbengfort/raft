# fabfile
# Run benchmarking commands on geonet servers
#
# Author:  Benjamin Bengfort <benjamin@bengfort.com>
# Created: Fri Aug 03 13:57:40 2018 -0400
#
# ID: fabfile.py [] benjamin@bengfort.com $

"""
Run benchmarking commands on geonet servers
"""

##########################################################################
## Imports
##########################################################################

import os
import re
import json
import pytz

from datetime import datetime
from StringIO import StringIO
from tabulate import tabulate
from operator import itemgetter
from collections import defaultdict
from dotenv import load_dotenv, find_dotenv
from dateutil.parser import parse as date_parse

from fabric.contrib import files
from fabric.colors import red, green, cyan
from fabric.api import env, run, cd, get, put, hide
from fabric.api import parallel, task, runs_once, execute


##########################################################################
## Environment Helpers
##########################################################################

# Load the host information
def load_hosts(path):
    with open(path, 'r') as f:
        return json.load(f)


def load_host_regions(hosts):
    locations = defaultdict(list)
    for host in hosts:
        loc = " ".join(host.split("-")[1:-1])
        locations[loc].append(host)
    return locations


def parse_bool(val):
    if isinstance(val, basestring):
        val = val.lower().strip()
        if val in {'yes', 'y', 'true', 't', '1'}:
            return True
        if val in {'no', 'n', 'false', 'f', '0'}:
            return False
    return bool(val)


##########################################################################
## Environment
##########################################################################

## Load the environment
load_dotenv(find_dotenv())

## Local paths
fixtures = os.path.dirname(__file__)
hostinfo = os.path.join(fixtures, "hosts.json")
sshconf  = os.path.expanduser("~/.ssh/geonet.config")

## Remote Paths
workspace = "/data/raft"
repo = "~/workspace/go/src/github.com/bbengfort/raft"

## Load Hosts
hosts = load_hosts(hostinfo)
regions = load_host_regions(hosts)
addrs = {info['hostname']: host for host, info in hosts.items()}
env.hosts = sorted(list(hosts.keys()))

## Fabric Env
env.user = "ubuntu"
env.ssh_config_path = sshconf
env.colorize_errors = True
env.use_ssh_config = True
env.forward_agent = True


##########################################################################
## Fabric Commands
##########################################################################

@task
@parallel
def install():
    """
    Install epaxos for the first time on each machine
    """
    with cd(os.path.dirname(repo)):
        run("git clone git@github.com:bbengfort/raft.git")

    with cd(repo):
        run("dep ensure")
        run("go install ./...")

    run("mkdir -p {}".format(workspace))


@task
@parallel
def uninstall():
    """
    Uninstall ePaxos on every machine
    """
    run("rm -rf {}".format(repo))
    run("rm -rf {}".format(workspace))


@task
@parallel
def update():
    """
    Update Raft by pulling the repository and installing the command.
    """
    with cd(repo):
        run("git pull")
        run("dep ensure")
        run("go install ./...")


@task
@runs_once
def version():
    """
    Get the current Raft version number
    """

    @parallel
    def _version():
        return run("raft -version")

    row = re.compile(r'raft version ([\d\.]+)', re.I)
    table = []
    counts = defaultdict(int)

    with hide("output", "running"):
        data = execute(_version)

    for host, line in data.items():
        version = row.match(line).groups()[0]
        counts[version] += 1
        table.append([host, version])

    table.sort(key=lambda r: r[0])
    table.insert(0, ["Host", "Version"])
    print("\n"+tabulate(table, tablefmt='simple', headers='firstrow'))

    counts = [["Version", "Count"]] + [item for item in counts.items()]
    print("\n"+tabulate(counts, tablefmt='simple', headers='firstrow'))

@task
@parallel
def cleanup():
    """
    Cleans up results files so that the experiment can be run again.
    """
    names = ("metrics.json", )

    for name in names:
        path = os.path.join(workspace, name)
        run("rm -f {}".format(path))


@task
@parallel
def serve():
    with cd(workspace):
        run("raft serve -u 1m")


@task
@parallel
def bench(config, clients):
    """
    Run all servers on the host as well as benchmarks for the number of
    clients specified.
    """
    name = addrs[env.host]
    command = []

    # load the configuration
    with open(config, 'r') as f:
        config = json.load(f)

    # Create the serve command
    peers = set(peer["name"] for peer in config["peers"])
    if name in peers:
        args = make_args(c="config.json", o="metrics.json", u="1m", n=name)
        command.append("raft serve {}".format(args))

    # Create the benchmark command
    n = round_robin(int(clients), env.host)
    if n > 0:
        args = make_args(c="config.json", n=n, r=100, o="metrics.json", d="20s")
        command.append("raft bench -b {}".format(args))

    if len(command) == 0:
        return

    with cd(workspace):
        run(pproc_command(command))


@task
@parallel
def getmerge(name="metrics.json", path="data", suffix=None):
    """
    Get the results.json and the metrics.json files and save them with the
    specified suffix to the localpath.
    """
    remote = os.path.join(workspace, name)
    hostname = addrs[env.host]
    local = os.path.join(path, hostname, add_suffix(name, suffix))
    local  = unique_name(local)
    if files.exists(remote):
        get(remote, local)


@task
@parallel
def putconfig(local):
    remote = os.path.join(workspace, "config.json")
    put(local, remote)


##########################################################################
## Task Helper Functions
##########################################################################

def pproc_command(commands):
    """
    Creates a pproc command from a list of command strings.
    """
    commands = " ".join([
        "\"{}\"".format(command) for command in commands
    ])
    return "pproc {}".format(commands)


def add_suffix(path, suffix=None):
    if suffix:
        base, ext = os.path.splitext(path)
        path = "{}-{}{}".format(base, suffix, ext)
    return path


def unique_name(path, start=0, maxtries=1000):
    for idx in range(start+1, start+maxtries):
        ipath = add_suffix(path, idx)
        if not os.path.exists(ipath):
            return ipath

    raise ValueError(
        "could not get a unique path after {} tries".format(maxtries)
    )


def round_robin(n, host, hosts=list(addrs)):
    """
    Returns a number n (of clients) for the specified host, by allocating the
    n clients evenly in a round robin fashion. For example, if hosts = 3 and
    n = 5; then this function returns 2 for host[0], 2 for host[1] and 1 for
    host[2].
    """
    num = n / len(hosts)
    idx = hosts.index(host)
    if n % len(hosts) > 0 and idx < (n % len(hosts)):
        num += 1
    return num


def make_args(**kwargs):
    return " ".join([
        "-{} {}".format(key, val) for key, val in kwargs.items()
    ])
