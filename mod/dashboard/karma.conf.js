var path = require('path'),
    APP_CONFIG = require('config');

module.exports = function(config) {

  function bowerPath(dir) {
    return path.join(APP_CONFIG.bowerPath, dir);
  }

  function appPath(dir) {
    return path.join(APP_CONFIG.appPath, dir);
  }

  function testPath(dir) {
    return path.join(APP_CONFIG.testPath, dir);
  }

  config.set({

    // Base path, that will be used to resolve files and exclude.
    basePath: __dirname,

    // Test framework to use.
    frameworks: ['jasmine'],

    // list of files / patterns to load in the browser
    files: [
      // TODO: programatically generate list of bower components
      bowerPath('/angular/angular.js'),
      bowerPath('/angular-route/angular-route.js'),
      bowerPath('/angular-resource/angular-resource.js'),
      bowerPath('/angular-animate/angular-animate.js'),
      bowerPath('/angular-mocks/angular-mocks.js'),
      bowerPath('/angular-bootstrap/ui-bootstrap-tpls.js'),
      bowerPath('/underscore/underscore.js'),
      bowerPath('/underscore.string/lib/underscore.string.js'),
      bowerPath('/jquery/dist/jquery.js'),
      bowerPath('/d3/d3.js'),

      // Tests & test helper files.
      testPath('/util/**/*.js'),
      testPath('/mock/**/*.js'),
      testPath('/**/*.spec.js'),

      // Actual client-side code.
      appPath('/*.js'),
      appPath('/compiled/*.js'),
      appPath('/{directive,module,service,filter,coreos}/**/*.js')
    ],

    reporters: ['progress'],

    // web server port
    port: 8100,

    // cli runner port
    runnerPort: 9100,

    colors: true,

    // LOG_DISABLE, LOG_ERROR, LOG_WARN, LOG_INFO, LOG_DEBUG
    logLevel: config.LOG_INFO,

    autoWatch: false,

    browsers: ['Chrome'],

    // If browser does not capture in given timeout [ms], kill it
    captureTimeout: 5000,

    // Continuous Integration mode.
    // If true, it capture browsers, run tests and exit.
    singleRun: true

  });

};
