#!/usr/bin/env python

import os
import argparse

import numpy as np
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt

sns.set_style('whitegrid')
sns.set_context('notebook')

FIXTURES = os.path.dirname(__file__)
THROUGHPUT = os.path.join(FIXTURES, "throughput.csv")
FIGURE = os.path.join(FIXTURES, "benchmark.png")


def draw_benchmark(path, vtype='line', exclude=None):
    exclude = set([]) if exclude is None else set(exclude)
    df = pd.read_csv(path)
    df = df[~df['version'].isin(exclude)]

    if vtype == 'both':
        _, axes = plt.subplots(ncols=2, figsize=(18,6), sharey=True)
        draw_line_benchmark(df, axes[0])
        draw_bar_benchmark(df, axes[1])

    else:
        _, ax = plt.subplots(figsize=(9,6))

        if vtype == 'line':
            draw_line_benchmark(df, ax)
        elif vtype == 'bar':
            draw_bar_benchmark(df, ax)
        else:
            raise ValueError("unknown viz type: '{}'".format(vtype))

    plt.tight_layout()
    plt.savefig(FIGURE)


def draw_line_benchmark(df, ax):
    max_clients = df['clients'].max()

    for vers in df['version'].unique():
        sample = df[df['version'] == vers]
        means = sample.groupby('clients')['throughput'].mean()
        std = sample.groupby('clients')['throughput'].std()

        ax.plot(means, label=vers)
        ax.fill_between(np.arange(1, max_clients+1), means+std, means-std, alpha=0.25)

    ax.set_xlim(1, max_clients)
    ax.set_ylabel("throughput (requests/second)")
    ax.set_xlabel("concurrent clients")
    ax.set_title("Raft Benchmark")
    ax.legend(frameon=True)
    return ax


def draw_bar_benchmark(df, ax):
    g = sns.barplot('clients', 'throughput', hue='version', ax=ax, data=df)
    ax.set_ylabel("")
    ax.set_xlabel("concurrent clients")
    ax.set_title("Raft Benchmark")
    return ax


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description="draw the benchmark visualization from a dataset",
    )

    parser.add_argument(
        '-t', '--type', choices=('bar', 'line', 'both'), default='line',
        help='specify the type of chart to produce'
    )
    parser.add_argument(
        '-e', '--exclude', nargs="*",
        help='specify server types to exclude from visualization',
    )
    parser.add_argument("data")

    args = parser.parse_args()
    draw_benchmark(args.data, args.type, args.exclude)
