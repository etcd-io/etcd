'use strict';

var app = angular.module('etcdControlPanel', [
  'ngRoute',
  'ngResource',
  'etcd',
  'etcdDirectives',
  'timeRelative',
  'underscore',
  'jquery',
  'moment',
  'vg'
]);

app.constant('urlPrefix', '/mod/dashboard');
app.constant('keyPrefix', '/v2/keys/');

app.config(function($routeProvider, $locationProvider, urlPrefix) {

  function prefixUrl(url) {
    return urlPrefix + url;
  }

  $locationProvider.html5Mode(true);

  $routeProvider
    .when(prefixUrl('/'), {
      controller: 'HomeCtrl',
      templateUrl: prefixUrl('/views/home.html')
    })
    .when(prefixUrl('/stats'), {
      controller: 'StatsCtrl',
      templateUrl: prefixUrl('/views/stats.html')
    })
    .when(prefixUrl('/browser'), {
      controller: 'BrowserCtrl',
      templateUrl: prefixUrl('/views/browser.html')
    })
    .otherwise({
      templateUrl: prefixUrl('/404.html')
    });

});
