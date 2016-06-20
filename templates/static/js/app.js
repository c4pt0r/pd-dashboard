'use strict';

var dashboardApp = angular.module('dashboardApp', []);

dashboardApp.controller('LogEventController', function LogEventController($scope) {
    $scope.init = function(wsHost) {
        var ws = new WebSocket("ws://" + wsHost + "/ws");
        ws.onopen = function(evt) {
            console.log("OPEN");
        }
        ws.onclose = function(evt) {
            console.log("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            console.log("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            console.log("ERROR: " + evt.data);
        }
    };
});
