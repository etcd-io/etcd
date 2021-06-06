#!/usr/bin/env python3
import sys
import os
import argparse
import logging
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import matplotlib.colors as colors

logging.basicConfig(format='[%(levelname)s %(asctime)s %(name)s] %(message)s')
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

params = None


def parse_args():
    parser = argparse.ArgumentParser(
        description='plot graph using mixed read/write result file.')
    parser.add_argument('input_file_a', type=str,
                        help='first input data files in csv format. (required)')
    parser.add_argument('input_file_b', type=str, nargs='?',
                        help='second input data files in csv format. (optional)')
    parser.add_argument('-t', '--title', dest='title', type=str, required=True,
                        help='plot graph title string')
    parser.add_argument('-z', '--zero-centered', dest='zero', action='store_true', required=False,
                        help='plot the improvement graph with white color represents 0.0',
                        default=True)
    parser.add_argument('--no-zero-centered', dest='zero', action='store_false', required=False,
                        help='plot the improvement graph without white color represents 0.0')
    parser.add_argument('-o', '--output-image-file', dest='output', type=str, required=True,
                        help='output image filename')
    parser.add_argument('-F', '--output-format', dest='format', type=str, default='png',
                        help='output image file format. default: jpg')
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
            param_df = df[df['type'] == 'PARAM']
            param_str = ''
            if len(param_df) != 0:
                param_str = param_df['comment'].iloc[0]
            new_df = df[df['type'] == 'DATA'][[
                'ratio', 'conn_size', 'value_size']].copy()
            cols = [x for x in df.columns if x.find('iter') != -1]
            tmp = [df[df['type'] == 'DATA'][x].str.split(':') for x in cols]

            read_df = [x.apply(lambda x: float(x[0])) for x in tmp]
            read_avg = sum(read_df) / len(read_df)
            new_df['read'] = read_avg

            write_df = [x.apply(lambda x: float(x[1])) for x in tmp]
            write_avg = sum(write_df) / len(write_df)
            new_df['write'] = write_avg

            new_df['ratio'] = new_df['ratio'].astype(float)
            new_df['conn_size'] = new_df['conn_size'].astype(int)
            new_df['value_size'] = new_df['value_size'].astype(int)
            res.append({
                'dataframe': new_df,
                'param': param_str
            })
    except Exception as e:
        logger.error(str(e))
        sys.exit(1)
    return res


# This is copied directly from matplotlib source code. Some early versions of matplotlib
# do not have CenteredNorm class
class CenteredNorm(colors.Normalize):

    def __init__(self, vcenter=0, halfrange=None, clip=False):
        """
        Normalize symmetrical data around a center (0 by default).

        Unlike `TwoSlopeNorm`, `CenteredNorm` applies an equal rate of change
        around the center.

        Useful when mapping symmetrical data around a conceptual center
        e.g., data that range from -2 to 4, with 0 as the midpoint, and
        with equal rates of change around that midpoint.

        Parameters
        ----------
        vcenter : float, default: 0
            The data value that defines ``0.5`` in the normalization.
        halfrange : float, optional
            The range of data values that defines a range of ``0.5`` in the
            normalization, so that *vcenter* - *halfrange* is ``0.0`` and
            *vcenter* + *halfrange* is ``1.0`` in the normalization.
            Defaults to the largest absolute difference to *vcenter* for
            the values in the dataset.

        Examples
        --------
        This maps data values -2 to 0.25, 0 to 0.5, and 4 to 1.0
        (assuming equal rates of change above and below 0.0):

            >>> import matplotlib.colors as mcolors
            >>> norm = mcolors.CenteredNorm(halfrange=4.0)
            >>> data = [-2., 0., 4.]
            >>> norm(data)
            array([0.25, 0.5 , 1.  ])
        """
        self._vcenter = vcenter
        self.vmin = None
        self.vmax = None
        # calling the halfrange setter to set vmin and vmax
        self.halfrange = halfrange
        self.clip = clip

    def _set_vmin_vmax(self):
        """
        Set *vmin* and *vmax* based on *vcenter* and *halfrange*.
        """
        self.vmax = self._vcenter + self._halfrange
        self.vmin = self._vcenter - self._halfrange

    def autoscale(self, A):
        """
        Set *halfrange* to ``max(abs(A-vcenter))``, then set *vmin* and *vmax*.
        """
        A = np.asanyarray(A)
        self._halfrange = max(self._vcenter-A.min(),
                              A.max()-self._vcenter)
        self._set_vmin_vmax()

    def autoscale_None(self, A):
        """Set *vmin* and *vmax*."""
        A = np.asanyarray(A)
        if self._halfrange is None and A.size:
            self.autoscale(A)

    @property
    def vcenter(self):
        return self._vcenter

    @vcenter.setter
    def vcenter(self, vcenter):
        self._vcenter = vcenter
        if self.vmax is not None:
            # recompute halfrange assuming vmin and vmax represent
            # min and max of data
            self._halfrange = max(self._vcenter-self.vmin,
                                  self.vmax-self._vcenter)
            self._set_vmin_vmax()

    @property
    def halfrange(self):
        return self._halfrange

    @halfrange.setter
    def halfrange(self, halfrange):
        if halfrange is None:
            self._halfrange = None
            self.vmin = None
            self.vmax = None
        else:
            self._halfrange = abs(halfrange)

    def __call__(self, value, clip=None):
        if self._halfrange is not None:
            # enforce symmetry, reset vmin and vmax
            self._set_vmin_vmax()
        return super().__call__(value, clip=clip)


# plot type is the type of the data to plot. Either 'read' or 'write'
def plot_data(title, plot_type, cmap_name_default, *args):
    if len(args) == 1:
        fig_size = (12, 16)
        df0 = args[0]['dataframe']
        df0param = args[0]['param']
        fig = plt.figure(figsize=fig_size)
        count = 0
        for val, df in df0.groupby('ratio'):
            count += 1
            plt.subplot(4, 2, count)
            plt.tripcolor(df['conn_size'], df['value_size'], df[plot_type])
            plt.title('R/W Ratio {:.4f} [{:.2f}, {:.2f}]'.format(val, df[plot_type].min(),
                                                                 df[plot_type].max()))
            plt.yscale('log', base=2)
            plt.ylabel('Value Size')
            plt.xscale('log', base=2)
            plt.xlabel('Connections Amount')
            plt.colorbar()
            plt.tight_layout()
        fig.suptitle('{} [{}]\n{}'.format(title, plot_type.upper(), df0param))
    elif len(args) == 2:
        fig_size = (12, 26)
        df0 = args[0]['dataframe']
        df0param = args[0]['param']
        df1 = args[1]['dataframe']
        df1param = args[1]['param']
        fig = plt.figure(figsize=fig_size)
        col = 0
        delta_df = df1.copy()
        delta_df[[plot_type]] = ((df1[[plot_type]] - df0[[plot_type]]) /
                                 df0[[plot_type]]) * 100
        for tmp in [df0, df1, delta_df]:
            row = 0
            for val, df in tmp.groupby('ratio'):
                pos = row * 3 + col + 1
                plt.subplot(8, 3, pos)
                norm = None
                if col == 2:
                    cmap_name = 'bwr'
                    if params.zero:
                        norm = CenteredNorm()
                else:
                    cmap_name = cmap_name_default
                plt.tripcolor(df['conn_size'], df['value_size'], df[plot_type],
                              norm=norm,
                              cmap=plt.get_cmap(cmap_name))
                if row == 0:
                    if col == 0:
                        plt.title('{}\nR/W Ratio {:.4f} [{:.1f}, {:.1f}]'.format(
                            os.path.basename(params.input_file_a),
                            val, df[plot_type].min(), df[plot_type].max()))
                    elif col == 1:
                        plt.title('{}\nR/W Ratio {:.4f} [{:.1f}, {:.1f}]'.format(
                            os.path.basename(params.input_file_b),
                            val, df[plot_type].min(), df[plot_type].max()))
                    elif col == 2:
                        plt.title('Gain\nR/W Ratio {:.4f} [{:.2f}%, {:.2f}%]'.format(val, df[plot_type].min(),
                                                                                     df[plot_type].max()))
                else:
                    if col == 2:
                        plt.title('R/W Ratio {:.4f} [{:.2f}%, {:.2f}%]'.format(val, df[plot_type].min(),
                                                                               df[plot_type].max()))
                    else:
                        plt.title('R/W Ratio {:.4f} [{:.1f}, {:.1f}]'.format(val, df[plot_type].min(),
                                                                             df[plot_type].max()))
                plt.yscale('log', base=2)
                plt.ylabel('Value Size')
                plt.xscale('log', base=2)
                plt.xlabel('Connections Amount')

                if col == 2:
                    plt.colorbar(format='%.2f%%')
                else:
                    plt.colorbar()
                plt.tight_layout()
                row += 1
            col += 1
        fig.suptitle('{} [{}]\n{}    {}\n{}    {}'.format(
            title, plot_type.upper(), os.path.basename(params.input_file_a), df0param,
            os.path.basename(params.input_file_b), df1param))
    else:
        raise Exception('invalid plot input data')
    fig.subplots_adjust(top=0.93)
    plt.savefig("{}_{}.{}".format(params.output, plot_type,
                params.format), format=params.format)


def main():
    global params
    logging.basicConfig()
    params = parse_args()
    result = load_data_files(params.input_file_a, params.input_file_b)
    for i in [('read', 'viridis'), ('write', 'plasma')]:
        plot_type, cmap_name = i
        plot_data(params.title, plot_type, cmap_name, *result)


if __name__ == '__main__':
    main()
