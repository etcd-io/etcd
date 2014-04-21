'use strict';

angular.module('etcd.ui').value('graphConfig', {

  'padding': {'top': 10, 'left': 5, 'bottom': 40, 'right': 10},
  'data': [
    {
      'name': 'stats'
    },
    {
      'name': 'thresholds',
      'values': [50, 100]
    }
  ],
  'scales': [
    {
      'name': 'y',
      'type': 'ordinal',
      'range': 'height',
      'domain': {'data': 'stats', 'field': 'index'}
    },
    {
      'name': 'x',
      'range': 'width',
      'domainMin': 0,
      'domainMax': 100,
      'nice': true,
      'zero': true,
      'domain': {'data': 'stats', 'field': 'data.latency.current'}
    },
    {
      'name': 'color',
      'type': 'linear',
      'domain': [10, 50, 100, 1000000000],
      'range': ['#00DB24', '#FFC000', '#c40022', '#c40022']
    }
  ],
  'axes': [
    {
      'type': 'x',
      'scale': 'x',
      'ticks': 6,
      'name': 'Latency (ms)'
    },
    {
      'type': 'y',
      'scale': 'y',
      'properties': {
        'ticks': {
          'stroke': {'value': 'transparent'}
        },
        'majorTicks': {
          'stroke': {'value': 'transparent'}
        },
        'labels': {
          'fill': {'value': 'transparent'}
        },
        'axis': {
          'stroke': {'value': '#333'},
          'strokeWidth': {'value': 1}
        }
      }
    }
  ],
  'marks': [
    {
      'type': 'rect',
      'from': {'data': 'stats'},
      'properties': {
        'enter': {
          'x': {'scale': 'x', 'value': 0},
          'x2': {'scale': 'x', 'field': 'data.latency.current'},
          'y': {'scale': 'y', 'field': 'index', 'offset': -1},
          'height': {'value': 3},
          'fill': {'scale':'color', 'field':'data.latency.current'}
        }
      }
    },
    {
        'type': 'symbol',
        'from': {'data': 'stats'},
        'properties': {
          'enter': {
            'x': {'scale': 'x', 'field': 'data.latency.current'},
            'y': {'scale': 'y', 'field': 'index'},
            'size': {'value': 50},
            'fill': {'value': '#000'}
          }
        }
      }
    ]

});
