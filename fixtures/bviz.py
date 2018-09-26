#!/usr/bin/env python3
# Visualize the throughput from benchmarks

import json
import argparse
import numpy as np
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt


FIGSIZE = (10,6)
COLORS = ["#2980b9", "#27ae60", "#c0392b", "#8e44ad", "#f39c12", "#16a085", "#2c3e50"]


def load_data(path):
    with open(path, 'r') as f:
        for line in f:
            # Ignore error lines or comments
            if not line.startswith("{"): continue
            yield json.loads(line)


def p95(v):
    return np.percentile(v, 95)


def plot_throughput(path, failure=False, title=None):
    if failure:
        _, axes = plt.subplots(nrows=2, sharex=True, figsize=FIGSIZE)
    else:
        _, ax = plt.subplots(figsize=FIGSIZE)
        axes = [ax]


    data = pd.DataFrame(load_data(path))
    data["ops"] = data["requests"] + data["failures"]

    # Plot the throughput graph
    sns.barplot(x="ops", y="throughput", data=data, ci=None, estimator=p95, color=COLORS[0], ax=axes[0])

    if failure:
        # Plot the failure graph and the xlable under the second axes
        sns.barplot(x="ops", y="failures", data=data, color=COLORS[2], ax=axes[1])
        axes[1].set_xlabel("number of requests")
        axes[0].set_xlabel("")
    else:
        axes[0].set_xlabel("number of requests")

    # Plot the y label and set the title
    axes[0].set_ylabel("throughput (ops/sec)")
    title = title or "Raft Throughput"
    axes[0].set_title(title)

    return axes


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('data', help='json lines throughput results output by blast')
    parser.add_argument('-f', '--failures', action='store_true', help='also plot failures')
    parser.add_argument('-t', '--title', default=None, help='set title for the figure')
    parser.add_argument('-s', '--savefig', default=None, help='save the figure to disk, if none will show')

    args = parser.parse_args()
    axes = plot_throughput(args.data, args.failures, args.title)

    plt.tight_layout()
    if args.savefig:
        plt.savefig(args.savefig)
    else:
        plt.show()
