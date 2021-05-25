#!/usr/bin/env python3
import six
import sys
import os
import argparse
import logging
import pandas as pd
import matplotlib.pyplot as plt
from mpl_toolkits.mplot3d import Axes3D


logging.basicConfig(format='[%(levelname)s %(asctime)s %(name)s] %(message)s')
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

params = None


def parse_args():
    parser = argparse.ArgumentParser(description='plot graph using mixed read/write result file.')
    parser.add_argument('input_file_a', type=str,
                        help='first input data files in csv format. (required)')
    parser.add_argument('input_file_b', type=str, nargs='?',
                        help='second input data files in csv format. (optional)')
    parser.add_argument('-t', '--title', dest='title', type=str, required=True,
                        help='plot graph title string')
    parser.add_argument('-o', '--output-image', dest='output', type=str, required=True,
                        help='output image filename')
    return parser.parse_args()


def load_data_files(*args):
    df_list = []
    try:
        for i in args:
            if i is not None:
                logger.debug('loading csv file {}'.format(i))
                df_list.append(pd.read_csv(i))
    except FileNotFoundError as e:
        logger.error(str(e))
        sys.exit(1)
    res = []
    try:
        for df in df_list:
            new_df = df[['ratio', 'conn_size', 'value_size']].copy()
            cols = [x for x in df.columns if x.find('iter') != -1]
            tmp = [df[x].str.split(':') for x in cols]

            read_df = [x.apply(lambda x: float(x[0])) for x in tmp]
            read_avg = sum(read_df)/len(read_df)
            new_df['read'] = read_avg

            write_df = [x.apply(lambda x: float(x[1])) for x in tmp]
            write_avg = sum(write_df)/len(write_df)
            new_df['write'] = write_avg

            new_df['ratio'] = new_df['ratio'].astype(float)
            new_df['conn_size'] = new_df['conn_size'].astype(int)
            new_df['value_size'] = new_df['value_size'].astype(int)
            res.append(new_df)
    except Exception as e:
        logger.error(str(e))
        sys.exit(1)
    return res


def plot_data(title, *args):
    if len(args) == 1:
        figsize = (12, 16)
        df0 = args[0]
        fig = plt.figure(figsize=figsize)
        count = 0
        for val, df in df0.groupby('ratio'):
            count += 1
            plt.subplot(4, 2, count)
            plt.tripcolor(df['conn_size'], df['value_size'], df['read'] + df['write'])
            plt.title('R/W Ratio {:.4f}'.format(val))
            plt.yscale('log', base=2)
            plt.ylabel('Value Size')
            plt.xscale('log', base=2)
            plt.xlabel('Connections Amount')
            plt.colorbar()
            plt.tight_layout()
    elif len(args) == 2:
        figsize = (12, 26)
        df0 = args[0]
        df1 = args[1]
        fig = plt.figure(figsize=figsize)
        count = 0
        delta_df = df1.copy()
        delta_df[['read', 'write']] = (df1[['read', 'write']] - df0[['read', 'write']])/df0[['read', 'write']]
        for tmp in [df0, df1, delta_df]:
            count += 1
            count2 = count
            for val, df in tmp.groupby('ratio'):
                plt.subplot(8, 3, count2)
                if count2 % 3 == 0:
                    cmap_name = 'bwr'
                else:
                    cmap_name = 'viridis'
                plt.tripcolor(df['conn_size'], df['value_size'], df['read'] + df['write'], cmap=plt.get_cmap(cmap_name))
                if count2 == 1:
                    plt.title('{}\nR/W Ratio {:.4f}'.format(os.path.basename(params.input_file_a), val))
                elif count2 == 2:
                    plt.title('{}\nR/W Ratio {:.4f}'.format(os.path.basename(params.input_file_b), val))
                elif count2 == 3:
                    plt.title('Delta\nR/W Ratio {:.4f}'.format(val))
                else:
                    plt.title('R/W Ratio {:.4f}'.format(val))
                plt.yscale('log', base=2)
                plt.ylabel('Value Size')
                plt.xscale('log', base=2)
                plt.xlabel('Connections Amount')
                plt.colorbar()
                plt.tight_layout()
                count2 += 3
    else:
        raise Exception('invalid plot input data')
    fig.suptitle(title)
    fig.subplots_adjust(top=0.95)
    plt.savefig(params.output)


def plot_data_3d(df, title):
    fig = plt.figure(figsize=(10, 10))
    ax = fig.add_subplot(projection='3d')
    ax.scatter(df['conn_size'], df['value_size'], 1/(1+1/df['ratio']), c=df['read'] + df['write'])
    ax.set_title('{}'.format(title))
    ax.set_zlabel('R/W Ratio')
    ax.set_ylabel('Value Size')
    ax.set_xlabel('Connections Amount')
    plt.show()


def main():
    global params
    logging.basicConfig()
    params = parse_args()
    result = load_data_files(params.input_file_a, params.input_file_b)
    plot_data(params.title, *result)


if __name__ == '__main__':
    main()
