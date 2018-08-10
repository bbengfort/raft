#!/usr/bin/env python
# Update configurations with IP Addresses

import os
import json
import glob
import argparse

from collections import OrderedDict

CONFIGS = os.path.join(os.path.dirname(__file__), "*.json")


def configs():
    """
    List the configuration paths in the directory
    """
    for path in glob.glob(CONFIGS):
        yield path


def load_hosts(path):
    """
    Create name: ip_address mapping for update
    """
    with open(path, 'r') as f:
        hosts = json.load(f)

    return {
        name: conf["hostname"]
        for name, conf in hosts.items()
    }


def update_configs(hosts):
    hosts = load_hosts(hosts)

    for path in configs():
        with open(path, 'r') as f:
            conf = json.load(f, object_pairs_hook=OrderedDict)

        for peer in conf['peers']:
            peer["ip_address"] = hosts[peer["name"]]

        with open(path, 'w') as o:
            json.dump(conf, o, indent=2)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description="update configs with IP addresses from hosts.json"
    )
    parser.add_argument("hosts", help="host file with IP addresses")
    args = parser.parse_args()

    update_configs(args.hosts)
