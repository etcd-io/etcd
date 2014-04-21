'use strict';

angular.module('underscore', []).factory('_', function($window) {
  return $window._;
});

angular.module('jquery', []).factory('$', function($window) {
  return $window.$;
});

angular.module('d3', []).factory('d3', function($window) {
  return $window.d3;
});

angular.module('coreos.services', [
  'coreos.events',
  'underscore',
  'jquery'
]);
angular.module('coreos.ui', [
  'coreos.events',
  'underscore',
  'jquery',
  'd3',
  'ui.bootstrap'
]);
angular.module('coreos.filters', []);
angular.module('coreos.events', []);
angular.module('coreos', [
  'coreos.events',
  'coreos.services',
  'coreos.ui',
  'coreos.filters',
  'coreos-templates-html',
  'coreos-templates-svg',

  // External deps.
  'ngRoute',
  'ngResource',
  'ngAnimate',
  'ui.bootstrap',
  'underscore',
  'jquery',
  'd3'
])
.config(function($compileProvider) {
  // Allow irc links.
  $compileProvider
    .aHrefSanitizationWhitelist(/^\s*(https?|ftp|mailto|tel|file|irc):/);
});

'use strict';

angular.module('coreos.filters')
.filter('orderObjectBy', function() {
  return function(items, field, reverse) {
    var filtered = [];
    angular.forEach(items, function(item) {
      filtered.push(item);
    });
    filtered.sort(function (a, b) {
      return (a[field] > b[field]);
    });
    if (reverse) {
      filtered.reverse();
    }
    return filtered;
  };
});

'use strict';

angular.module('coreos.filters')
.filter('utc', function(_) {

  function convertToUtc(date) {
    return new Date(date.getUTCFullYear(),
        date.getUTCMonth(),
        date.getUTCDate(),
        date.getUTCHours(),
        date.getUTCMinutes(),
        date.getUTCSeconds());
  }

  return function(input) {
    if (_.isNumber(input)) {
      return convertToUtc(new Date(input));
    }
    if (_.isString(input)) {
      return convertToUtc(new Date(Date.parse(input)));
    }
    if (_.isDate(input)) {
      return convertToUtc(input);
    }
    return '';
  };

});

/**
 * Broadcast when the window size breakpoints change.
 * TODO(sym3tri): change implementation to use window.matchMedia instead.
 */

'use strict';

angular.module('coreos.services')
.factory('breakpointSvc', function(_, $window, $rootScope, CORE_CONST,
      CORE_EVENT) {

  var previousName;

  function getSize() {
    var width = $window.innerWidth;
    return _.find(CORE_CONST.BREAKPOINTS, function(bp) {
      if (bp.min <= width && bp.max > width) {
        return true;
      }
    }).name;
  }

  function onResize() {
    var breakpointName = getSize();
    if (breakpointName !== previousName) {
      $rootScope.$broadcast(CORE_EVENT.BREAKPOINT, breakpointName);
      previousName = breakpointName;
    }
  }

  // Broadcast initial size.
  $rootScope.$broadcast(CORE_EVENT.BREAKPOINT, getSize());

  // Watch for resizes.
  angular.element($window).on('resize', _.debounce(onResize, 20, true));

  return {
    getSize: getSize
  };

});

'use strict';

angular.module('coreos.services').provider('configSvc', function() {
  var configValues = {};

  this.config = function(newConfig) {
    if (newConfig) {
      configValues = newConfig;
    } else {
      return configValues;
    }
  };

  this.$get = function() {
    return {
      get: function(key) {
        if (key) {
          return configValues[key];
        } else {
          return angular.copy(configValues);
        }
      },

      set: function(key, value) {
        configValues[key] = value;
      }
    };
  };

});

'use strict';

angular.module('coreos').constant('CORE_CONST', {

  HIGHLIGHT_CSS_CLASS: 'co-an-highlight',

  BREAKPOINTS: [
    {
      name: 'xs',
      min: 0,
      max: 480
    },
    {
      name: 'sm',
      min: 480,
      max: 768
    },
    {
      name: 'md',
      min: 768,
      max: 992
    },
    {
      name: 'lg',
      min: 992,
      max: 1200
    },
    {
      name: 'xl',
      min: 1200,
      max: Infinity
    }
  ]

});

/**
 * @fileoverview
 *
 * Service for working with cookies since angular's built-in cookie service
 * leaves much to be desired.
 */

'use strict';

angular.module('coreos.services').factory('cookieSvc',
    function($window, timeSvc) {

  return {

    /**
     * Create a new cookie.
     */
    create: function(name, value, daysUtilExpires) {
      var date, expires;
      if (daysUtilExpires) {
        date = new Date();
        date.setTime(date.getTime() +
            (daysUtilExpires * timeSvc.ONE_DAY_IN_MS));
        expires = '; expires=' + date.toGMTString();
      }
      else {
        expires = '';
      }
      $window.document.cookie = name + '=' + value + expires + '; path=/';
    },

    /**
     * Retrieve a cookie by name.
     */
    get: function(name) {
      var nameEq, cookieList, i, cookieStr;
      nameEq = name + '=';
      cookieList = $window.document.cookie.split(';');
      for (i = 0; i < cookieList.length; i++) {
        cookieStr = cookieList[i];
        while (cookieStr.charAt(0) === ' ') {
          cookieStr = cookieStr.substring(1, cookieStr.length);
        }
        if (cookieStr.indexOf(nameEq) === 0) {
          return cookieStr.substring(nameEq.length, cookieStr.length);
        }
      }
      return null;
    },

    /**
     * Delete a cookie by name.
     */
    remove: function(name) {
      this.create(name, '', -1);
    }

  };

});

/**
 * @fileoverview
 *
 * Simply inject this service to start broadcasting events.
 * It will feature-detect any available browser visibility api.
 * If the feature exists it will broadcast an event when browser visibiltiy
 * changes.
 */

'use strict';

angular.module('coreos.services')
.factory('documentVisibilitySvc', function($rootScope, $document, _,
      CORE_EVENT) {

  var document = $document[0],
      features,
      detectedFeature;

  function broadcastChangeEvent() {
    $rootScope.$broadcast(CORE_EVENT.DOC_VISIBILITY_CHANGE,
        document[detectedFeature.propertyName]);
  }

  features = {
    standard: {
      eventName: 'visibilitychange',
      propertyName: 'hidden'
    },
    moz: {
      eventName: 'mozvisibilitychange',
      propertyName: 'mozHidden'
    },
    ms: {
      eventName: 'msvisibilitychange',
      propertyName: 'msHidden'
    },
    webkit: {
      eventName: 'webkitvisibilitychange',
      propertyName: 'webkitHidden'
    }
  };

  Object.keys(features).some(function(feature) {
    if (_.isBoolean(document[features[feature].propertyName])) {
      detectedFeature = features[feature];
      return true;
    }
  });

  if (detectedFeature) {
    $document.on(detectedFeature.eventName, broadcastChangeEvent);
  }

  return {

    /**
     * Is the window currently hidden or not.
     */
    isHidden: function() {
      if (detectedFeature) {
        return document[detectedFeature.propertyName];
      }
    }

  };

});

'use strict';

angular.module('coreos.events').constant('CORE_EVENT', {
  PAGE_NOT_FOUND: 'core.event.page_not_found',
  BREAKPOINT: 'core.event.breakpoint',
  RESP_ERROR: 'core.event.resp_error',
  RESP_MUTATE: 'core.event.resp_mutate',
  DOC_VISIBILITY_CHANGE: 'core.event.doc_visibility_change',
  POLL_ERROR: 'core.event.poll_error'
});

/**
 * @fileoverview
 *
 * Utility service to highlight an element or selection of elements.
 * NOTE: Expects a [HIGHLIGHT_CSS_CLASS] class to be defined in constants.
 */

'use strict';

angular.module('coreos.services')
.factory('highlighterSvc', function($timeout, $, CORE_CONST) {

  var pendingTimeout;

  return {

    /**
     * Highlight an element in the DOM.
     *
     * @param {String|Element} elemOrSelector
     */
    highlight: function(elemOrSelector) {
      var elem;
      if (!elemOrSelector) {
        return;
      }
      elem = $(elemOrSelector);
      if (elem.hasClass(CORE_CONST.HIGHLIGHT_CSS_CLASS)) {
        $timeout.cancel(pendingTimeout);
        elem.removeClass(CORE_CONST.HIGHLIGHT_CSS_CLASS);
      }
      elem.addClass(CORE_CONST.HIGHLIGHT_CSS_CLASS);
      pendingTimeout = $timeout(
          elem.removeClass.bind(elem, CORE_CONST.HIGHLIGHT_CSS_CLASS), 5000);
    }

  };

});

'use strict';

angular.module('coreos.services')
.factory('interceptorErrorSvc', function($q, $rootScope, CORE_EVENT) {

  function parseMessage(rejection) {
    var errorMsg;
    if (rejection.config.description) {
      errorMsg = 'Error attempting: ' + rejection.config.description;
    } else {
      errorMsg = 'A network error occurred.';
    }
    return errorMsg;
  }

  return {

    /**
     * For every failing $http request: broadcast an error event.
     */
    'responseError': function(rejection) {
      if (!rejection.config.supressNotifications) {
        $rootScope.$broadcast(CORE_EVENT.RESP_ERROR,
          rejection,
          parseMessage(rejection));
      }
      return $q.reject(rejection);
    }

  };

});

'use strict';

angular.module('coreos.services')
.factory('interceptorMutateSvc', function($q, $rootScope, CORE_EVENT) {

  // Remove last path segement of a url.
  function removeLastPath(url) {
    var newUrl = url.split('/');
    newUrl.pop();
    newUrl = newUrl.join('/');
    return newUrl;
  }

  return {

    /**
     * For every successful mutating $http request broadcast the urls.
     * Useful for cache invalidation.
     */
    'response': function(response) {
      var method = response.config.method,
          url = response.config.url,
          cacheKeys;

      if (method !== 'GET') {
        cacheKeys = [];
        cacheKeys.push(url);
        if (method !== 'POST') {
          cacheKeys.push(removeLastPath(url));
        }
        $rootScope.$broadcast(CORE_EVENT.RESP_MUTATE, response);
      }
      return response || $q.when(response);
    }

  };

});

/**
 * A general purpose polling service.
 *
 * Provide a series of options with callacks and this service will start a
 * poller for the task.
 *
 * On failure it will try up to `maxRetries`, then will be killed and callback
 * to the `catchMaxFail()` function if provided.
 *
 * Optionally pass in a `scope` associated with the poller to automatically
 * kill the poller when the scope is destroyed.
 *
 * Global settings for this provider can be configured in the app `config`
 * stage. Instance will override defaults if provided ot the `register()`
 * function.
 *
 * EXAMPLE USAGE:
 *
 *    poller.register('myPoller', {
 *      fn: functionToRunRepeadedly,
 *      then: successCallback,
 *      catch: errorCallback,
 *      catchMaxFail: afterMaxFailuresCallback,
 *      scope: $scope,
 *      startIn: 0,
 *      interval: 5000
 *    });
 */


'use strict';

angular.module('coreos.services').provider('pollerSvc', function() {
  var settings = {},
      pollers = {};

  /**
   * Update global settings for the provider.
   * @param {Object} newSettings
   */
  this.settings = function(newSettings) {
    if (newSettings) {
      settings = newSettings;
    } else {
      return settings;
    }
  };

  /**
   * The main factory method.
   * Dependencies are injected and is invoked by angular.
   */
  this.$get = function pollerFactory($q, $http, $timeout, _, CORE_EVENT) {
    /* jshint unused:false */

    function isRegistered(name) {
      return !!pollers[name];
    }

    /**
     * Schedule the `execute` function to run.
     * @param {Number} delay When to start in ms.
     */
    function schedule(name, executor, delay) {
      var poller = pollers[name];
      if (!poller || poller._errorCount > poller.maxRetries) {
        return;
      }
      poller._state = 'waiting';
      poller._timeoutPromise = $timeout(executor, delay);
    }

    /**
     * Wrap a function to prevent it from running if the current state
     * is "terminated".
     */
    function runIfActive(name, fn) {
      var poller = pollers[name];
      if (!poller) {
        return angular.noop;
      }
      return function() {
        if (poller._state !== 'terminated') {
          return fn.apply(null, arguments);
        }
      };
    }

    function killPoller(name) {
      var poller;
      if (!isRegistered(name)) {
        return;
      }
      poller = pollers[name];
      poller._state = 'terminated';
      // Cancel the interval timer.
      if (poller._timeoutPromise) {
        $timeout.cancel(poller._timeoutPromise);
      }
      // Remove the scope.$destroy handler.
      poller._unlistenDestroy();
      // Delete from the list.
      delete pollers[name];
    }

    /**
     * Create an executor function for a poller with the given name.
     */
    function createExecutor(name) {
      var poller = pollers[name];
      if (!poller) {
        return angular.noop;
      }

      /**
       * The main function that will be run on an interval for a poller.
       * This wraps the user-provided function, executes callbacks after
       * completion, and handles scheduling.
       */
      return function execute() {
        if (poller._paused) {
          schedule(name, poller._executor, poller.interval);
          return;
        }
        poller._state = 'executing';
        poller.fn()
          .then(runIfActive(name, function() {
            poller._state = 'success';
            poller._errorCount = 0;
            poller.then.apply(null, arguments);
          }))
          .catch(runIfActive(name, function() {
            var args;
            poller._state = 'error';
            poller._errorCount += 1;
            poller.catch.apply(null, arguments);
            if (poller._errorCount > poller.maxRetries) {
              args = _.toArray(arguments);
              args.unshift(name);
              poller.catchMaxFail.apply(null, args);
              killPoller(name);
            }
          }))
          .finally(runIfActive(name, function() {
            poller.finally.apply(null, arguments);
            schedule(name, poller._executor, poller.interval);
          }));
      };
    }

    return {

      /**
       * Determines if a poller is already registered by name.
       * @param {String} name
       * @return {Boolean}
       */
      isRegistered: isRegistered,

      /**
       * Register the promise in the index, and schedule it to start polling.
       *
       * @param {String} name The uniqe name to associate with the poller.
       * @param {Object} options
       */
      register: function(name, options) {
        // kill the old poller if one by same name already exists.
        if (isRegistered(name)) {
          this.kill(name);
        }

        // Initialize all poller options.
        _.defaults(options, settings, {
          startIn: 0,
          maxRetries: 0,
          catch: angular.noop,
          then: angular.noop,
          finally: angular.noop,
          catchMaxFail: function() {
            if (options.scope) {
              options.scope.$emit(CORE_EVENT.POLL_ERROR);
            }
          },
          _unlistenDestroy: angular.noop,
          _errorCount: 0,
          _state: 'starting'
        });

        if (options.scope) {
          // If a scope is provided, automatically kill the poller when the
          // scope is destroyed.
          options._unlistenDestroy =
            options.scope.$on('$destroy', this.kill.bind(this, name));

          // When scope is prvided automatically pause polling when tab
          // loses visability.
          // TODO: add pauseAll() function and move this to app.run()
          options.scope.$on(CORE_EVENT.DOC_VISIBILITY_CHANGE,
              function(e, isHidden) {
            if (isHidden) {
              options._paused = true;
            } else {
              options._paused = false;
            }
          });
        }

        // Keep track of the poller in the index.
        pollers[name] = options;

        // Generate the executor wrapper for the poller.
        options._executor = createExecutor(name);

        // Schedule the initial run of the poller.
        schedule(name, options._executor, options.startIn);
      },

      /**
       * Kill a poller by name and remove all references, callbacks, etc.
       * @param {String} name
       */
      kill: function(name) {
        killPoller(name);
      },

      /**
       * Kill all registered pollers.
       */
      killAll: function() {
        Object.keys(pollers).forEach(this.kill.bind(this));
      }

    };

  };

});

/**
 * @fileoverview
 *
 * Utility service that scrolls elements into view.
 */

'use strict';

angular.module('coreos.services')
.factory('scrollerSvc', function($timeout, $) {

  function scroll(elem) {
    elem.first()[0].scrollIntoView();
  }

  var scrollerSvc = {

    /**
     * Scroll to the element on the page with matching id.
     * Adds and removes highlight classes too.
     *
     * @param {String|Element} elemOrSelector
     */
    scrollTo: function(elemOrSelector) {
      var maxTries = 100,
          numTries = 0,
          interval = 10,
          elem;

      if (!elemOrSelector) {
        return;
      }

      // Wait for element to appear in DOM if it doesn't exist yet,
      // then scroll to it.
      function attemptScroll() {
        elem = $(elemOrSelector);
        if (numTries < maxTries) {
          if (!elem.length) {
            numTries++;
            $timeout(attemptScroll, interval);
          } else {
            scroll(elem);
          }
        }
      }

      $timeout(attemptScroll, 0);
    }

  };

  return scrollerSvc;

});

'use strict';

angular.module('coreos.services')
.factory('arraySvc', function() {

  return {

    /**
     * Remove first occurance of an item from an array in-place.
     *
     * @param {Arrray} ary Array to mutate.
     * @param {*} item Array item to remove.
     * @return {Array} The input array.
     */
    remove: function(ary, item) {
      var index;
      if (!ary || !ary.length) {
        return [];
      }
      index = ary.indexOf(item);
      if (index > -1) {
        ary.splice(index, 1);
      }
      return ary;
    }

  };

});

'use strict';

angular.module('coreos.services')
.factory('mathSvc', function(_) {

  return {

    /**
     * If passed an array sums all items in the array.
     * Otherwise sums all arguments together.
     *
     * @param {Array|Number...}
     * @return {Number}
     */
    sum: function() {
      var ary;
      if (_.isArray(arguments[0])) {
        ary = arguments[0];
      } else {
        ary = _.toArray(arguments);
      }
      return ary.reduce(function(prev, curr) {
        return prev + curr;
      }, 0);
    }

  };

});

'use strict';

angular.module('coreos.services')
.factory('timeSvc', function(_) {

  var ONE_MINUTE_IN_MS = 60 * 1000,
      ONE_HOUR_IN_MS = ONE_MINUTE_IN_MS * 60,
      ONE_DAY_IN_MS = ONE_HOUR_IN_MS * 24,
      ONE_WEEK_IN_MS = ONE_DAY_IN_MS * 7,
      THIRTY_DAYS_IN_MS = ONE_DAY_IN_MS * 30;

  function getTimestamp(val) {
    if (val && _.isNumber(val)) {
      return val;
    }
    return Date.now();
  }

  return {
    ONE_MINUTE_IN_MS: ONE_MINUTE_IN_MS,
    ONE_HOUR_IN_MS: ONE_HOUR_IN_MS,
    ONE_DAY_IN_MS: ONE_DAY_IN_MS,
    ONE_WEEK_IN_MS: ONE_WEEK_IN_MS,
    THIRTY_DAYS_IN_MS: THIRTY_DAYS_IN_MS,

    milliToSecs: function(ms) {
      return Math.floor(ms / 1000);
    },

    secsToMins: function(secs) {
      return Math.floor(parseInt(secs, 10) / 60) || 0;
    },

    minsToSecs: function(mins) {
      return Math.abs(parseInt(mins, 10) * 60) || 0;
    },

    oneHourAgo: function(ts) {
      return getTimestamp(ts) - this.ONE_HOUR_IN_MS;
    },

    oneDayAgo: function(ts) {
      return getTimestamp(ts) - this.ONE_DAY_IN_MS;
    },

    oneWeekAgo: function(ts) {
      return getTimestamp(ts) - this.ONE_WEEK_IN_MS;
    },

    thirtyDaysAgo: function(ts) {
      return getTimestamp(ts) - this.THIRTY_DAYS_IN_MS;
    },

    getRelativeTimestamp: function(term) {
      var now = Date.now();
      switch(term) {
        case 'month':
          return this.thirtyDaysAgo(now);
        case 'week':
          return this.oneWeekAgo(now);
        case 'day':
          return this.oneDayAgo(now);
        case 'hour':
          return this.oneHourAgo(now);
      }
    }

  };

});

/**
 * @fileoverview
 * Wrap buttons and automatically enable/disbale and show loading indicator.
 */

'use strict';

angular.module('coreos.ui')
.directive('coBtnBar', function($, $timeout, $compile) {

  return {
    templateUrl: '/coreos.ui/btn-bar/btn-bar.html',
    restrict: 'EA',
    transclude: true,
    replace: true,
    scope: {
      // A promise that indicates completion of async operation.
      'completePromise': '='
    },
    link: function(scope, elem) {
      var linkButton,
          loaderDirectiveEl;

      linkButton = $('.btn-link', elem).last();
      loaderDirectiveEl =
          angular.element('<co-inline-loader></co-inline-loader>');
      $compile(loaderDirectiveEl)(scope);

      function disableButtons() {
        elem.append(loaderDirectiveEl);
        $('button', elem).attr('disabled', 'disabled');
        linkButton.addClass('hidden');
      }

      function enableButtons() {
        loaderDirectiveEl.remove();
        $('button', elem).removeAttr('disabled');
        linkButton.removeClass('hidden');
      }

      scope.$watch('completePromise', function(completePromise) {
        if (completePromise) {
          // Force async execution so disabling the button won't prevent form
          // submission.
          $timeout(disableButtons, 0);
          completePromise.finally(function() {
            // Also enable buttons asynchronously in case the request completes
            // before disableButtons() runs.
            $timeout(enableButtons, 0);
          });
        }
      });
    }

  };

});

/**
 * Simple directive to navigate to a route when the
 * element is clicked on.
 */

'use strict';

angular.module('coreos.ui')
.directive('coClickNav', function($location) {

  return {
    restrict: 'A',
    link: function(scope, elem, attrs) {
      function onClickHandler(event) {
        $location.url(attrs.coClickNav);
        scope.$apply();
        event.preventDefault();
        event.stopPropagation();
      }
      elem.on('click', onClickHandler);
      elem.on('$destroy', function() {
        elem.off('click', onClickHandler);
      });
    }
  };

});

/**
 * @fileoverview
 * Display a cog icon and construct dropdown menu.
 */

'use strict';

angular.module('coreos.ui')
.directive('coCog', function() {

  return {
    templateUrl: '/coreos.ui/cog/cog.html',
    restrict: 'E',
    replace: true,
    scope: {
      'apps': '=',
      'options': '=',
      'size': '@',
      'anchor': '@'
    },
    link: function(scope, elem) {
      scope.clickHandler = function($event, option) {
        $event.stopPropagation();
        $event.preventDefault();
        if (option.callback) {
          option.callback();
        }
        elem.removeClass('open');
      };
    }
  };

});

'use strict';

angular.module('coreos.ui')
.controller('ConfirmModalCtrl', function($scope, $modalInstance,
      executeFn, title, message, btnText, errorFormatter) {

  $scope.errorFormatter = errorFormatter;
  $scope.title = title;
  $scope.message = message;
  $scope.btnText = btnText || 'Confirm';

  $scope.execute = function() {
    $scope.requestPromise = executeFn(null, {
      supressNotifications: true
    })
    .then($modalInstance.close);
  };

  $scope.cancel = function() {
    $modalInstance.dismiss('cancel');
  };

});

/**
 * @fileoverview
 * An arc donut chart.
 */

// TDOO(sym3tri): add hover text.

'use strict';

angular.module('coreos.ui')
.directive('coDonut', function(d3, _) {

  return {

    templateUrl: '/coreos.ui/donut/donut.html',
    transclude: true,
    restrict: 'E',
    replace: true,
    scope: {
      // The original source data to graph.
      percent: '=',
      color: '@'
    },
    controller: function($scope) {
      var outerRadius, circleWidth;
      $scope.width = $scope.height = 80;
      outerRadius = $scope.width / 2;
      circleWidth = 15;
      $scope.arc = d3.svg.arc()
        .innerRadius(outerRadius - circleWidth)
        .outerRadius(outerRadius)
        .startAngle(0);
      // Constant to turn percents into radian angles.
      $scope.tau = 2 * Math.PI;
    },
    link: function(scope, elem) {
      scope.isRendered = false;

      function render() {
        var endAngle = scope.tau, // 100%
            textColor = '#333',
            bgcolor = '#eee',
            color = scope.color || '#000',
            fontSize = 18;

        // Keep track of added DOM elements.
        scope.el = {};

        scope.el.svg = d3.select(elem.find('.co-m-gauge__content')[0])
          .append('svg')
          .attr('width', scope.width)
          .attr('height', scope.height)
          .append('g')
            .attr('transform',
                'translate(' +
                scope.width / 2 + ',' +
                scope.height / 2 + ')');

        scope.el.text = scope.el.svg.append('text')
          .attr('fill', textColor)
          .attr('y', Math.floor(fontSize / 3))
          .attr('font-size', fontSize + 'px')
          .attr('text-anchor', 'middle');

        scope.el.arcGroup = scope.el.svg.append('g')
          .attr('transform', 'rotate(180)');

        scope.el.background = scope.el.arcGroup.append('path')
          .datum({
            endAngle: endAngle
          })
          .style('fill', bgcolor)
          .attr('d', scope.arc);

        scope.el.foreground = scope.el.arcGroup.append('path')
          .datum({
            endAngle: scope.tau * (scope.percent || 0)
          })
          .style('fill', color)
          .style('opacity', 0.8)
          .attr('d', scope.arc);

        scope.isRendered = true;
      }

      /**
       * Update the value of the donut chart.
       */
      function updateValue() {
        if (!_.isNumber(scope.percent)) {
          scope.el.text.text('?');
          return;
        }
        scope.el.text.text(Math.round(scope.percent * 100) + '%');
        scope.el.foreground.transition()
          .duration(750)
          .call(arcTween, scope.percent * scope.tau);
      }

      /**
       * Transition function to animate the arc.
       */
      function arcTween(transition, newAngle) {
        transition.attrTween('d', function(d) {
          var interpolate = d3.interpolate(d.endAngle, newAngle);
          return function(t) {
            d.endAngle = interpolate(t);
            return scope.arc(d);
          };
        });
      }

      /**
       * Cleanup.
       */
      elem.on('$destroy', function() {
        scope.el.svg.remove();
      });

      render();

      scope.$watch('percent', function() {
        if (scope.isRendered) {
          updateValue();
        }
      });
    }
  };

});

/**
 * @fileoverview
 * Displays a message based on a promise.
 */

'use strict';
angular.module('coreos.ui')


.provider('errorMessageSvc', function() {

  var formatters = {};

  this.registerFormatter = function(name, fn) {
    formatters[name] = fn;
  };

  this.$get = function() {
    return {
      getFormatter: function(name) {
        return formatters[name] || angular.noop;
      }
    };
  };

})


.directive('coErrorMessage', function(errorMessageSvc) {

  return {
    templateUrl: '/coreos.ui/error-message/error-message.html',
    restrict: 'E',
    replace: true,
    scope: {
      promise: '=',
      formatter: '@',
      customMessage: '@message'
    },
    controller: function postLink($scope) {
      $scope.show = false;

      function handler(resp) {
        if ($scope.formatter) {
          $scope.message =
            errorMessageSvc.getFormatter($scope.formatter)(resp);
        } else if ($scope.customMessage) {
          $scope.message = $scope.customMessage;
        } else {
          return;
        }
        $scope.show = true;
      }

      $scope.$watch('promise', function(promise) {
        $scope.show = false;
        if (promise && promise.catch) {
          promise.catch(handler);
        }
      });

    }
  };

});

/**
 * @fileoverview
 * Inject favicons into the <head>.
 * Only use on <head> tag.
 */


'use strict';
angular.module('coreos.ui')

.directive('coFavicons', function($compile, $rootScope, configSvc) {
  /*jshint maxlen:false */

  return {
    restrict: 'A',
    replace: true,
    link: function postLink(scope, elem) {
      var newScope = $rootScope.$new(),
      htmlTemplate =
        '<link rel="apple-touch-icon-precomposed" sizes="144x144" href="{{path}}/apple-touch-icon-144-precomposed.png">' +
        '<link rel="apple-touch-icon-precomposed" sizes="114x114" href="{{path}}/apple-touch-icon-114-precomposed.png">' +
        '<link rel="apple-touch-icon-precomposed" sizes="72x72" href="{{path}}/apple-touch-icon-72-precomposed.png">' +
        '<link rel="apple-touch-icon-precomposed" href="{{path}}/apple-touch-icon-57-precomposed.png">' +
        '<link rel="shortcut icon" href="{{path}}/favicon.png">';
      newScope.path = configSvc.get('libPath') + '/img';
      elem.append($compile(htmlTemplate)(newScope));
    }
  };

});

/*
*/

/**
 * @fileoverview
 * Standard CoreOS footer.
 *
 */

'use strict';
angular.module('coreos.ui')

.directive('coFooter', function() {
  return {
    templateUrl: '/coreos.ui/footer/footer.html',
    transclude: true,
    restrict: 'E',
    replace: true
  };
})

.directive('coFooterLink', function() {
  return {
    templateUrl: '/coreos.ui/footer/footer-link.html',
    transclude: true,
    restrict: 'E',
    replace: true,
    scope: {
      href: '@',
      iconClass: '@'
    }
  };
})


/**
 * Convenience wrapper for doing sticky footers.
 */
.directive('coFooterWrapper', function() {
  return {
    templateUrl: '/coreos.ui/footer/footer-wrapper.html',
    transclude: true,
    restrict: 'E',
    replace: true
  };

});

/**
 * @fileoverview
 * Highlight an item when its bound data changes.
 */

'use strict';

angular.module('coreos.ui')
.directive('coHighlight', function(highlighterSvc) {

  return {
    restrict: 'A',
    link: function(scope, elem, attrs) {

      scope.$watch(attrs.coHighlight, function(newValue, oldValue) {
        if (newValue !== oldValue) {
          highlighterSvc.highlight(elem);
        }
      });

    }
  };

});

/**
 * @fileoverview
 *
 * Inline loading indicator widget.
 */

'use strict';
angular.module('coreos.ui')

.directive('coInlineLoader', function() {

  return {
    templateUrl: '/coreos.ui/inline-loader/inline-loader.html',
    restrict: 'E',
    replace: true
  };

});

/**
 * @fileoverview
 *
 * Loading indicator that centers itself inside its parent.
 */


'use strict';
angular.module('coreos.ui')

.directive('coLoader', function() {
  return {
    templateUrl: '/coreos.ui/loader/loader.html',
    restrict: 'E',
    replace: true
  };
});

/**
 * @fileoverview
 * Display page title with primary action link.
 */


'use strict';
angular.module('coreos.ui')

.directive('coNavTitle', function() {
  return {
    templateUrl: '/coreos.ui/nav-title/nav-title.html',
    transclude: true,
    restrict: 'E',
    replace: true,
    scope: {
      title: '@'
    }
  };
});

/**
 * @fileoverview
 * Top navbar which inlcudes nav links.
 */


'use strict';
angular.module('coreos.ui')

.directive('coNavbar', function(configSvc) {

  return {
    templateUrl: '/coreos.ui/navbar/navbar.html',
    transclude: true,
    restrict: 'E',
    replace: true,
    controller: function($scope) {
      $scope.config = configSvc.get();
      $scope.isCollapsed = true;
    }
  };

})


/**
 * Simple directive to create bootstrap friendly navbar links.
 * Will automatically add the 'active' class based on the route.
 */
.directive('coNavbarLink', function($location) {

  return {
    templateUrl: '/coreos.ui/navbar/navbar-link.html',
    transclude: true,
    restrict: 'E',
    replace: true,
    scope: {
      // The path to link to.
      'href': '@'
    },
    link: function(scope) {
      scope.isActive = function() {
        return $location.path() === scope.href;
      };
    }
  };

})

/**
 * Optional dropdown menu to put in the right of the navbar.
 */
.directive('coNavbarDropdown', function() {

  return {
    templateUrl: '/coreos.ui/navbar/navbar-dropdown.html',
    transclude: true,
    restrict: 'E',
    replace: true,
    scope: {
      text: '@'
    }
  };

});


/**
 * @fileoverview
 * Directive to easily inline svg images.
 * NOTE: kind of a hack to get ng-include to work properly within a directive
 * without wrapping it with an extra DOM element.
 */

'use strict';

angular.module('coreos.ui')
.directive('coSvg', function($, $rootScope, $compile) {

  return {
    template: '<div></div>',
    restrict: 'E',
    replace: true,
    scope: {
      src: '@',
      width: '@',
      height: '@'
    },
    link: function(scope, elem, attrs) {
      var containerEl, html, newScope;
      newScope = $rootScope.$new();
      html = '<div class="co-m-svg" '+
              'ng-class="classes" ng-style="style" ng-include="src"></div>';
      newScope.style = {};
      if (scope.width) {
        newScope.style.width = scope.width + 'px';
      }
      if (scope.height) {
        newScope.style.height = scope.height + 'px';
      }
      if (attrs.class) {
        newScope.classes = attrs.class;
      }
      scope.$watch('src', function(src) {
        if (src) {
          newScope.src = src;
          containerEl = $compile(html)(newScope);
          elem.replaceWith(containerEl);
        }
      });
    }
  };

});

'use strict';

angular.module('coreos.ui')
.directive('coTextCopy', function() {

  return {
    restrict: 'A',
    replace: true,
    link: function(scope, elem) {
      function onClickHandler(event) {
        elem.select();
        event.preventDefault();
        event.stopPropagation();
      }
      elem.on('click', onClickHandler);
      elem.on('$destroy', function() {
        elem.off('click', onClickHandler);
      });
    }
  };

});

/**
 * @fileoverview
 *
 * Keeps the title tag updated.
 */

'use strict';
angular.module('coreos.ui')


.directive('coTitle', function() {

  return {
    transclude: false,
    restrict: 'A',
    scope: {
      suffix: '@coTitleSuffix'
    },
    controller: function($scope, $rootScope, $route) {
      $scope.pageTitle = '';
      $scope.defaultTitle = null;
      $rootScope.$on('$routeChangeSuccess', function() {
        $scope.pageTitle = $route.current.title || $route.current.$$route.title;
      });
    },
    link: function(scope, elem) {
      scope.$watch('pageTitle', function(title) {
        if (title) {
          if (!scope.defaultTitle) {
            scope.defaultTitle = elem.text();
          }
          elem.text(title + ' ' + scope.suffix);
        } else {
          if (scope.defaultTitle) {
            elem.text(scope.defaultTitle);
          }
        }
      });
    }
  };

});

/**
 * @fileoverview
 * Directive to display global error or info messages.
 * Enqueue messages through the toastSvc.
 */


'use strict';

angular.module('coreos.ui')
.directive('coToast', function() {
  return {
    templateUrl: '/coreos.ui/toast/toast.html',
    restrict: 'E',
    replace: true,
    scope: true,
    controller: function($scope, toastSvc) {
      $scope.messages = toastSvc.messages;
      $scope.dismiss = toastSvc.dismiss;
    }
  };
});


angular.module('coreos.services')
.factory('toastSvc', function($timeout) {

  var AUTO_DISMISS_TIME = 5000,
      service,
      lastTimeoutPromise;

  function dequeue() {
    if (service.messages.length) {
      service.messages.shift();
    }
  }

  function enqueue(type, text) {
    service.messages.push({
      type: type,
      text: text
    });
    lastTimeoutPromise = $timeout(dequeue, AUTO_DISMISS_TIME);
  }

  function cancelTimeout() {
    if (lastTimeoutPromise) {
      $timeout.cancel(lastTimeoutPromise);
    }
  }

  service = {

    messages: [],

    error: enqueue.bind(null, 'error'),

    info: enqueue.bind(null, 'info'),

    dismiss: function(index) {
      cancelTimeout();
      service.messages.splice(index, 1);
    },

    dismissAll: function() {
      cancelTimeout();
      service.messages.length = 0;
    }

  };

  return service;

});

angular.module('coreos-templates-html', ['/coreos.ui/btn-bar/btn-bar.html', '/coreos.ui/cog/cog.html', '/coreos.ui/confirm-modal/confirm-modal.html', '/coreos.ui/donut/donut.html', '/coreos.ui/error-message/error-message.html', '/coreos.ui/favicons/favicons.html', '/coreos.ui/footer/footer-link.html', '/coreos.ui/footer/footer-wrapper.html', '/coreos.ui/footer/footer.html', '/coreos.ui/inline-loader/inline-loader.html', '/coreos.ui/loader/loader.html', '/coreos.ui/nav-title/nav-title.html', '/coreos.ui/navbar/navbar-dropdown.html', '/coreos.ui/navbar/navbar-link.html', '/coreos.ui/navbar/navbar.html', '/coreos.ui/toast/toast.html']);

angular.module("/coreos.ui/btn-bar/btn-bar.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/btn-bar/btn-bar.html",
    "<div class=\"co-m-btn-bar\" ng-transclude>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/cog/cog.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/cog/cog.html",
    "<div class=\"co-m-cog\">\n" +
    "  <span class=\"co-m-cog__icon co-m-cog__icon--size-{{size}} dropdown-toggle fa fa-cog\"></span>\n" +
    "  <ul class=\"dropdown-menu co-m-cog__dropdown co-m-dropdown--dark co-m-cog__dropdown--anchor-{{anchor}}\">\n" +
    "    <li ng-repeat=\"option in options | orderBy:'weight'\">\n" +
    "      <a ng-if=\"option.href\" ng-href=\"{{option.href}}\">{{option.label}}</a>\n" +
    "      <a ng-if=\"!option.href\" ng-click=\"clickHandler($event, option)\">{{option.label}}</a>\n" +
    "    </li>\n" +
    "  </ul>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/confirm-modal/confirm-modal.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/confirm-modal/confirm-modal.html",
    "<div>\n" +
    "  <form ng-submit=\"execute()\" name=\"form\" role=\"form\">\n" +
    "    <div class=\"modal-header\">\n" +
    "      <h4 class=\"modal-title\" ng-bind=\"title\"></h4>\n" +
    "    </div>\n" +
    "    <div class=\"modal-body\" ng-bind=\"message\"></div>\n" +
    "    <div class=\"modal-footer\" co-btn-bar complete-promise=\"requestPromise\">\n" +
    "      <co-error-message formatter=\"{{errorFormatter}}\" promise=\"requestPromise\"></co-error-message>\n" +
    "      <button type=\"submit\" class=\"btn btn-primary\" ng-bind=\"btnText\"></button>\n" +
    "      <button type=\"button\" ng-click=\"cancel()\" class=\"btn btn-link\">Cancel</button>\n" +
    "    </div>\n" +
    "  </form>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/donut/donut.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/donut/donut.html",
    "<div class=\"co-m-donut co-m-gauge\">\n" +
    "  <div class=\"co-m-gauge__content\"></div>\n" +
    "  <div class=\"co-m-gauge__label\" ng-transclude></div>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/error-message/error-message.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/error-message/error-message.html",
    "<div ng-show=\"show\" class=\"co-m-message co-m-message--error co-an-fade-in-out ng-hide\">{{message}}</div>\n" +
    "");
}]);

angular.module("/coreos.ui/favicons/favicons.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/favicons/favicons.html",
    "");
}]);

angular.module("/coreos.ui/footer/footer-link.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/footer/footer-link.html",
    "<a class=\"co-m-footer-link\" href=\"{{href}}\">\n" +
    "  <span class=\"co-m-footer-link--icon\" ng-if=\"iconClass\" ng-class=\"iconClass\"></span>\n" +
    "  <span ng-transclude></span>\n" +
    "</a>\n" +
    "");
}]);

angular.module("/coreos.ui/footer/footer-wrapper.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/footer/footer-wrapper.html",
    "<div id=\"co-l-footer-wrapper\">\n" +
    "  <div ng-transclude></div>\n" +
    "  <div id=\"co-l-footer-push\"></div>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/footer/footer.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/footer/footer.html",
    "<div id=\"co-l-footer\">\n" +
    "  <div class=\"container\" ng-transclude></div>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/inline-loader/inline-loader.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/inline-loader/inline-loader.html",
    "<div class=\"co-m-inline-loader co-an-fade-in-out\">\n" +
    "  <div class=\"co-m-inline-loader-dot__one\"></div>\n" +
    "  <div class=\"co-m-inline-loader-dot__two\"></div>\n" +
    "  <div class=\"co-m-inline-loader-dot__three\"></div>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/loader/loader.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/loader/loader.html",
    "<div class=\"co-m-loader co-an-fade-in-out\">\n" +
    "  <span class=\"co-m-loader__spinner\"></span>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/nav-title/nav-title.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/nav-title/nav-title.html",
    "<div class=\"co-m-nav-title row\">\n" +
    "  <div ng-transclude class=\"col-lg-3 col-md-3 col-sm-3 col-xs-6\"></div>\n" +
    "  <div class=\"col-lg-6 col-md-6 col-sm-6 col-xs-12\">\n" +
    "    <h1 class=\"co-m-page-title co-fx-text-shadow\">{{title}}</h1>\n" +
    "  </div>\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/navbar/navbar-dropdown.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/navbar/navbar-dropdown.html",
    "<ul class=\"nav navbar-nav pull-right\">\n" +
    "  <li class=\"dropdown pull-right\">\n" +
    "    <a href=\"#\" class=\"dropdown-toggle\">{{text}} <b class=\"caret\"></b></a>\n" +
    "    <ul ng-transclude class=\"dropdown-menu co-m-dropdown--dark\"></ul>\n" +
    "  </li>\n" +
    "</ul>\n" +
    "");
}]);

angular.module("/coreos.ui/navbar/navbar-link.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/navbar/navbar-link.html",
    "<li class=\"co-m-nav-link\" ng-class=\"{'active': isActive()}\">\n" +
    "  <a ng-href=\"{{href}}\" ng-transclude></a>\n" +
    "</li>\n" +
    "");
}]);

angular.module("/coreos.ui/navbar/navbar.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/navbar/navbar.html",
    "<div class=\"co-m-navbar co-fx-box-shadow navbar navbar-fixed-top\">\n" +
    "\n" +
    "  <div class=\"navbar-header\">\n" +
    "    <button ng-click=\"isCollapsed = !isCollapsed\" class=\"navbar-toggle\" type=\"button\">\n" +
    "      <span class=\"glyphicon glyphicon-align-justify\"></span>\n" +
    "    </button>\n" +
    "    <a ng-href=\"{{config.siteBasePath}}\" class=\"navbar-brand\">\n" +
    "      <co-svg class=\"co-m-navbar__logo\" src=\"/coreos.svg/logo.svg\"></co-svg>\n" +
    "    </a>\n" +
    "  </div>\n" +
    "\n" +
    "  <div collapse=\"isCollapsed\" ng-transclude class=\"collapse navbar-collapse\"></div>\n" +
    "\n" +
    "</div>\n" +
    "");
}]);

angular.module("/coreos.ui/toast/toast.html", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.ui/toast/toast.html",
    "<div class=\"co-m-toast\">\n" +
    "  <div ng-repeat=\"message in messages\"\n" +
    "      class=\"co-m-toast__message co-m-message co-m-message--{{message.type}} co-an-fade-in-out co-fx-box-shadow\">\n" +
    "    {{message.text}}\n" +
    "    <span ng-click=\"dismiss($index)\" class=\"pull-right glyphicon glyphicon-remove text-right co-m-message__close\"></span>\n" +
    "  </div>\n" +
    "</div>\n" +
    "");
}]);

angular.module('coreos-templates-svg', ['/coreos.svg/globe-only.svg', '/coreos.svg/icon-add.svg', '/coreos.svg/icon-back.svg', '/coreos.svg/icon-delete.svg', '/coreos.svg/icon-reboot.svg', '/coreos.svg/icon-right-arrow.svg', '/coreos.svg/logo.svg']);

angular.module("/coreos.svg/globe-only.svg", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.svg/globe-only.svg",
    "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n" +
    "<!-- Generator: Adobe Illustrator 17.0.0, SVG Export Plug-In . SVG Version: 6.00 Build 0)  -->\n" +
    "<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n" +
    "<svg version=\"1.1\" id=\"Layer_1\" xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" x=\"0px\" y=\"0px\"\n" +
    "	 preserveAspectRatio=\"xMidYMin\" viewBox=\"0 0 222.068 222.068\" enable-background=\"new 0 0 222.068 222.068\"\n" +
    "	 xml:space=\"preserve\">\n" +
    "<g>\n" +
    "	<path fill=\"#54A3DA\" d=\"M110.804,3.163c-59.27,0-107.479,48.212-107.479,107.473c0,59.265,48.209,107.474,107.479,107.474\n" +
    "		c59.252,0,107.465-48.209,107.465-107.474C218.269,51.375,170.056,3.163,110.804,3.163z\"/>\n" +
    "	<path fill=\"#F1616E\" d=\"M110.804,13.025c-17.283,0-31.941,27.645-37.235,66.069c-0.169,1.236-0.333,2.487-0.478,3.746\n" +
    "		c-0.723,6.047-1.213,12.335-1.458,18.808c-0.117,2.962-0.175,5.956-0.175,8.988c0,3.029,0.058,6.029,0.175,8.985\n" +
    "		c0.245,6.472,0.735,12.764,1.458,18.811c8.104,1.049,16.769,1.761,25.807,2.099c3.907,0.146,7.872,0.233,11.907,0.233\n" +
    "		c4.023,0,8-0.088,11.895-0.233c9.049-0.338,17.708-1.05,25.819-2.099c0.892-0.114,1.77-0.239,2.659-0.368\n" +
    "		c33.754-4.74,57.235-15.232,57.235-27.428C208.412,56.724,164.707,13.025,110.804,13.025z\"/>\n" +
    "	<path fill=\"#FFFFFF\" d=\"M151.177,83.205c-0.979-1.428-2.029-2.796-3.148-4.11c-8.956-10.557-22.297-17.265-37.224-17.265\n" +
    "		c-4.839,0-9.148,7.407-11.907,18.909c-1.096,4.586-1.947,9.819-2.495,15.498c-0.432,4.551-0.665,9.391-0.665,14.399\n" +
    "		s0.233,9.849,0.665,14.396c4.554,0.432,9.387,0.664,14.402,0.664c5.009,0,9.842-0.232,14.396-0.664\n" +
    "		c10.011-0.95,18.653-2.875,24.775-5.411c6.046-2.501,9.624-5.615,9.624-8.985C159.599,100.468,156.494,91.024,151.177,83.205z\"/>\n" +
    "</g>\n" +
    "</svg>\n" +
    "");
}]);

angular.module("/coreos.svg/icon-add.svg", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.svg/icon-add.svg",
    "<svg version=\"1.1\" xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" x=\"0px\" y=\"0px\"\n" +
    "  preserveAspectRatio=\"xMinYMin\" viewBox=\"0 0 72.556 61\" enable-background=\"new 0 0 72.556 61\" xml:space=\"preserve\">\n" +
    "  <path d=\"M34.521,8v11.088v23v10.737c0,2.209,1.791,4,4,4c2.209,0,4-1.791,4-4V42.067V19.109V8c0-2.209-1.791-4-4-4\n" +
    "  C36.312,4,34.521,5.791,34.521,8z\"/>\n" +
    "  <path d=\"M16.109,34.412h11.088h23h10.737c2.209,0,4-1.791,4-4c0-2.209-1.791-4-4-4H50.175H27.217H16.109c-2.209,0-4,1.791-4,4\n" +
    "  C12.109,32.621,13.9,34.412,16.109,34.412z\"/>\n" +
    "</svg>\n" +
    "");
}]);

angular.module("/coreos.svg/icon-back.svg", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.svg/icon-back.svg",
    "<svg version=\"1.1\" xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" x=\"0px\" y=\"0px\"\n" +
    "  preserveAspectRatio=\"xMinYMin\" viewBox=\"0 0 73.356 61\" enable-background=\"new 0 0 73.356 61\" xml:space=\"preserve\">\n" +
    "  <path d=\"M5.27,33.226l22.428,22.428c1.562,1.562,4.095,1.562,5.657,0c1.562-1.562,1.562-4.095,0-5.657L17.77,34.413h48.514\n" +
    "  c2.209,0,4-1.791,4-4s-1.791-4-4-4H17.749l15.604-15.582c1.563-1.561,1.565-4.094,0.004-5.657C32.576,4.391,31.552,4,30.527,4\n" +
    "  c-1.023,0-2.046,0.39-2.827,1.169L5.272,27.567c-0.751,0.75-1.173,1.768-1.173,2.829C4.098,31.458,4.52,32.476,5.27,33.226z\"/>\n" +
    "</svg>\n" +
    "");
}]);

angular.module("/coreos.svg/icon-delete.svg", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.svg/icon-delete.svg",
    "<svg version=\"1.1\" fill=\"#f00\" id=\"Layer_1\" xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\"\n" +
    "  x=\"0px\" y=\"0px\" preserveAspectRatio=\"xMinYMin\" viewBox=\"0 0 76.143 61\" enable-background=\"new 0 0 76.143 61\" xml:space=\"preserve\">\n" +
    "  <path d=\"M49.41,13.505l-6.035,6.035L27.112,35.803l-6.035,6.035c-1.562,1.562-1.562,4.095,0,5.657c1.562,1.562,4.095,1.562,5.657,0\n" +
    "  l6.05-6.05l16.234-16.234l6.05-6.05c1.562-1.562,1.562-4.095,0-5.657C53.505,11.943,50.972,11.943,49.41,13.505z\"/>\n" +
    "  <path d=\"M21.077,19.162l6.035,6.035L43.375,41.46l6.035,6.035c1.562,1.562,4.095,1.562,5.657,0c1.562-1.562,1.562-4.095,0-5.657\n" +
    "  l-6.05-6.05L32.783,19.555l-6.05-6.05c-1.562-1.562-4.095-1.562-5.657,0C19.515,15.067,19.515,17.6,21.077,19.162z\"/>\n" +
    "</svg>\n" +
    "");
}]);

angular.module("/coreos.svg/icon-reboot.svg", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.svg/icon-reboot.svg",
    "<svg version=\"1.1\" id=\"Layer_1\" xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" x=\"0px\" y=\"0px\"\n" +
    "	 preserveAspectRatio=\"xMinYMin\" viewBox=\"0 0 65.947 65.41\" enable-background=\"new 0 0 65.947 65.41\" xml:space=\"preserve\">\n" +
    "<g>\n" +
    "	<path d=\"M22.014,15.949c2.428-1.575,5.211-2.632,8.205-3.03c0,0,1.846-0.106,2.797-0.097C44.113,12.932,53.022,22,52.954,33.088\n" +
    "		l11.226-1.075C63.884,19.558,56.337,8.875,45.553,4.081c-0.043-0.025-0.07-0.061-0.115-0.08c-3.756-1.645-7.896-2.578-12.25-2.621\n" +
    "		c-0.014,0-0.025,0.002-0.039,0.002c-0.006,0-0.012-0.002-0.02-0.002c-0.691-0.006-1.371,0.021-2.051,0.066\n" +
    "		c-0.475,0.026-0.941,0.073-1.414,0.12c-0.072,0.008-0.148,0.011-0.221,0.02v0.006c-5.494,0.601-10.578,2.603-14.848,5.678\n" +
    "		l-3.068-4.523L7.038,21.636l18.849-2.034L22.014,15.949z\"/>\n" +
    "	<path d=\"M44.204,48.517c-2.428,1.575-5.211,2.632-8.205,3.03c0,0-1.846,0.106-2.797,0.097c-11.098-0.11-20.007-9.178-19.938-20.266\n" +
    "		L2.038,32.454c0.296,12.454,7.843,23.138,18.627,27.932c0.043,0.025,0.07,0.06,0.115,0.08c3.756,1.644,7.896,2.578,12.25,2.621\n" +
    "		c0.014,0,0.025-0.002,0.039-0.002c0.006,0,0.012,0.002,0.02,0.002c0.691,0.006,1.371-0.021,2.051-0.065\n" +
    "		c0.475-0.026,0.941-0.073,1.414-0.12c0.072-0.008,0.148-0.011,0.221-0.02v-0.006c5.494-0.601,10.578-2.604,14.848-5.678\n" +
    "		l3.068,4.523L59.18,42.83l-18.849,2.034L44.204,48.517z\"/>\n" +
    "</g>\n" +
    "</svg>\n" +
    "");
}]);

angular.module("/coreos.svg/icon-right-arrow.svg", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.svg/icon-right-arrow.svg",
    "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n" +
    "<!-- Generator: Adobe Illustrator 17.0.0, SVG Export Plug-In . SVG Version: 6.00 Build 0)  -->\n" +
    "<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n" +
    "<svg version=\"1.1\" id=\"Layer_1\" xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" x=\"0px\" y=\"0px\"\n" +
    "	 width=\"6px\" height=\"10px\" viewBox=\"0 0 6 10\" enable-background=\"new 0 0 6 10\" xml:space=\"preserve\">\n" +
    "<g>\n" +
    "	<polygon fill=\"#333333\" points=\"0,0 0,10 6,5 	\"/>\n" +
    "</g>\n" +
    "</svg>\n" +
    "");
}]);

angular.module("/coreos.svg/logo.svg", []).run(["$templateCache", function($templateCache) {
  $templateCache.put("/coreos.svg/logo.svg",
    "<svg version=\"1.1\" id=\"Layer_1\" xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" x=\"0px\" y=\"0px\"\n" +
    "    preserveAspectRatio=\"xMidYMin\" height=\"30px\" viewBox=\"24.5 40.5 744 224\" enable-background=\"new 24.5 40.5 744 224\" xml:space=\"preserve\">\n" +
    "  <g>\n" +
    "    <g>\n" +
    "      <path fill=\"#53A3DA\" d=\"M136.168,45.527C76.898,45.527,28.689,93.739,28.689,153c0,59.265,48.209,107.474,107.479,107.474\n" +
    "        c59.252,0,107.465-48.209,107.465-107.474C243.633,93.739,195.42,45.527,136.168,45.527z\"/>\n" +
    "      <path fill=\"#F1606D\" d=\"M136.168,55.389c-17.283,0-31.941,27.645-37.235,66.069c-0.169,1.236-0.333,2.487-0.478,3.746\n" +
    "        c-0.723,6.047-1.213,12.335-1.458,18.808c-0.117,2.962-0.175,5.956-0.175,8.988c0,3.029,0.058,6.029,0.175,8.985\n" +
    "        c0.245,6.472,0.735,12.764,1.458,18.811c8.104,1.049,16.769,1.761,25.807,2.099c3.907,0.146,7.872,0.233,11.907,0.233\n" +
    "        c4.023,0,8-0.088,11.895-0.233c9.049-0.338,17.708-1.05,25.819-2.099c0.892-0.114,1.77-0.239,2.659-0.368\n" +
    "        c33.754-4.74,57.235-15.232,57.235-27.428C233.776,99.088,190.071,55.389,136.168,55.389z\"/>\n" +
    "      <path fill=\"#FFFFFF\" d=\"M176.541,125.569c-0.979-1.428-2.029-2.796-3.148-4.11c-8.956-10.557-22.297-17.265-37.224-17.265\n" +
    "        c-4.839,0-9.148,7.407-11.907,18.909c-1.096,4.586-1.947,9.819-2.495,15.498c-0.432,4.551-0.665,9.391-0.665,14.399\n" +
    "        s0.233,9.849,0.665,14.396c4.554,0.432,9.387,0.664,14.402,0.664c5.009,0,9.842-0.232,14.396-0.664\n" +
    "        c10.011-0.95,18.653-2.875,24.775-5.411c6.046-2.501,9.624-5.615,9.624-8.985C184.963,142.832,181.858,133.388,176.541,125.569z\"\n" +
    "        />\n" +
    "    </g>\n" +
    "    <g>\n" +
    "      <path fill=\"#231F20\" d=\"M344.891,100.053c12.585,0,22.816,6.138,29.262,13.062l-10.064,11.326\n" +
    "        c-5.353-5.192-11.175-8.495-19.041-8.495c-16.839,0-28.953,14.16-28.953,37.291c0,23.448,11.169,37.608,28.32,37.608\n" +
    "        c9.128,0,15.895-3.775,21.717-10.228l10.067,11.169c-8.335,9.598-19.038,14.95-32.099,14.95c-26.119,0-46.731-18.88-46.731-53.025\n" +
    "        C297.37,120.036,318.454,100.053,344.891,100.053z\"/>\n" +
    "      <path fill=\"#231F20\" d=\"M416.961,125.701c19.352,0,36.822,14.793,36.822,40.597c0,25.647-17.471,40.439-36.822,40.439\n" +
    "        c-19.197,0-36.66-14.792-36.66-40.439C380.301,140.494,397.764,125.701,416.961,125.701z M416.961,191.945\n" +
    "        c11.33,0,18.25-10.228,18.25-25.647c0-15.577-6.92-25.804-18.25-25.804s-18.094,10.227-18.094,25.804\n" +
    "        C398.867,181.717,405.631,191.945,416.961,191.945z\"/>\n" +
    "      <path fill=\"#231F20\" d=\"M459.771,127.589h14.943l1.26,13.688h0.629c5.506-10.07,13.691-15.577,21.871-15.577\n" +
    "        c3.938,0,6.455,0.472,8.811,1.574l-3.148,15.734c-2.67-0.784-4.717-1.257-8.018-1.257c-6.139,0-13.539,4.245-18.256,15.893v47.203\n" +
    "        h-18.092L459.771,127.589L459.771,127.589z\"/>\n" +
    "      <path fill=\"#231F20\" d=\"M541.121,125.701c20.928,0,31.941,15.107,31.941,36.667c0,3.458-0.314,6.604-0.787,8.495h-49.09\n" +
    "        c1.57,14.003,10.379,21.869,22.811,21.869c6.613,0,12.273-2.041,17.941-5.662l6.135,11.326\n" +
    "        c-7.395,4.878-16.676,8.341-26.432,8.341c-21.404,0-38.08-14.95-38.08-40.439C505.561,141.12,523.023,125.701,541.121,125.701z\n" +
    "         M557.326,159.376c0-12.277-5.189-19.671-15.732-19.671c-9.125,0-16.996,6.768-18.57,19.671H557.326z\"/>\n" +
    "      <path fill=\"#F1606D\" d=\"M600.602,152.607c0-32.729,17.785-53.344,42.799-53.344c24.863,0,42.641,20.615,42.641,53.344\n" +
    "        c0,32.889-17.777,54.13-42.641,54.13C618.387,206.737,600.602,185.496,600.602,152.607z M678.49,152.607\n" +
    "        c0-28.639-14.158-46.731-35.09-46.731c-21.084,0-35.248,18.093-35.248,46.731c0,28.796,14.164,47.521,35.248,47.521\n" +
    "        C664.332,200.128,678.49,181.403,678.49,152.607z\"/>\n" +
    "      <path fill=\"#53A4D9\" d=\"M699.738,186.125c7.557,8.495,18.412,14.003,30.529,14.003c15.732,0,25.807-8.499,25.807-20.767\n" +
    "        c0-12.904-8.494-17.154-18.723-21.717l-15.736-7.082c-8.969-3.936-20.934-10.385-20.934-25.808\n" +
    "        c0-14.947,12.904-25.492,30.059-25.492c12.588,0,22.658,5.665,28.949,12.435l-4.244,4.878c-5.982-6.452-14.32-10.7-24.705-10.7\n" +
    "        c-13.691,0-22.816,7.239-22.816,18.565c0,11.962,10.385,16.521,17.936,19.985l15.738,6.921\n" +
    "        c11.486,5.195,21.713,11.647,21.713,27.539s-13.061,27.851-33.201,27.851c-15.107,0-26.75-6.451-34.932-15.576L699.738,186.125z\"\n" +
    "        />\n" +
    "    </g>\n" +
    "  </g>\n" +
    "</svg>\n" +
    "");
}]);
