'use strict';

angular.module('etcd.module', []);
angular.module('etcd.ui', []);
angular.module('etcd.page', []);

// The main etcd dashboard module.
var etcdDashboard = angular.module('etcd.dashboard', [
  'coreos',
  'etcd.module',
  'etcd.ui',
  'etcd.page',
  'ngRoute',
  'ngResource',
  'ngAnimate',
  'ui.bootstrap',
  'templates-views',
  'underscore',
  'jquery',
  'd3'
]);

// Routes
etcdDashboard.config(function($routeProvider, $locationProvider, $httpProvider,
    $compileProvider, pollerSvcProvider, errorMessageSvcProvider,
    configSvcProvider) {

  var siteBasePath = '/mod/dashboard';

  // Make routes less verbose.
  function path(suffix) {
    return siteBasePath + suffix;
  }

  // coreos-web config.
  configSvcProvider.config({
    siteBasePath: siteBasePath,
    libPath: '/mod/dashboard/static/coreos-web'
  });

  // Use HTML5 push state.
  $locationProvider.html5Mode(true);

  // Parse error messages from the api.
  errorMessageSvcProvider.registerFormatter('etcdApi', function(resp) {
    if (resp.data && resp.data.message) {
      return resp.data.message;
    }
    return 'An error occurred.';
  });

  // Emit event for any request error.
  $httpProvider.interceptors.push('interceptorErrorSvc');

  // Poller settings.
  pollerSvcProvider.settings({
    interval: 5000,
    maxRetries: 5
  });

  // Configure routes.
  $routeProvider
    .when(path('/'), {
      redirectTo: path('/browser')
    })
    .when(path('/browser'), {
      controller: 'BrowserCtrl',
      templateUrl: '/page/browser/browser.html',
      title: 'Key Browser'
    })
    .when(path('/stats'), {
      controller: 'StatsCtrl',
      templateUrl: '/page/stats/stats.html',
      title: 'Stats'
    })
    .otherwise({
      templateUrl: '/404.html',
      title: 'Page Not Found (404)'
    });

})

// After bootstrap initialization.
.run(function($http, $rootScope, $location, $window, $route, _, configSvc,
      toastSvc, CORE_EVENT) {

  // Show toast when poller fails.
  $rootScope.$on(CORE_EVENT.POLL_ERROR, function() {
    toastSvc.error('Error polling for data.');
  });

  // Show toast for any non-suppressed http response errors.
  $rootScope.$on(CORE_EVENT.RESP_ERROR, function(e, rejection) {
    var errorMsg = 'Request Error';
    if (rejection.data && rejection.data.message) {
      errorMsg = rejection.data.message;
    }
    toastSvc.error(errorMsg);
  });

  // Redirect to 404 page if event is thrown.
  $rootScope.$on(CORE_EVENT.PAGE_NOT_FOUND, function() {
    $location.url(configSvc.get().siteBaseUrl + '/404');
  });

});
