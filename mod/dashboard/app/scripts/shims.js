'use strict';

angular.module('underscore', []).factory('_', function($window) {
  return $window._;
});

angular.module('jquery', []).factory('$', function($window) {
  return $window.$;
});

angular.module('vg', []).factory('vg', function($window) {
  return $window.vg;
});

angular.module('moment', []).factory('moment', function($window) {

  $window.moment.lang('en', {
    relativeTime : {
      future: 'Expires in %s',
      past:   'Expired %s ago',
      s:  'seconds',
      m:  'a minute',
      mm: '%d minutes',
      h:  'an hour',
      hh: '%d hours',
      d:  'a day',
      dd: '%d days',
      M:  'a month',
      MM: '%d months',
      y:  'a year',
      yy: '%d years'
    }
  });

  return $window.moment;
});
