#!/usr/bin/env node

/*jslint node: true, continue: true, eqeq: true, forin: true, nomen: true, plusplus: true, todo: true, vars: true, white: true */

// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

"use strict";

// A simple way to run javascript from Go.
// The protocol is:
//    - read a line from stdin
//    - decode as base64
//    - evaluate it
//    - write back the result as a single line of base-64-encoded JSON containing
//    an object {result, exception}.

var sys = require("sys");

var stdin = process.openStdin();
var currentBuf = new Buffer(0);
var state = {}
stdin.on("data", function(d) {
    var i, line, result;
    currentBuf = Buffer.concat([currentBuf, d]);
    for(i = 0; i < currentBuf.length; i++){
        if(d[i] === 10){
            // We've found a newline (10 == '\n'), so use the line up to this point.
            line = currentBuf.slice(0, i);
            currentBuf = currentBuf.slice(i + 1);
            break;
        }
    }
    if(line === undefined){
        return;
    }
    result = {};
    line = (new Buffer(line.toString(), 'base64')).toString('utf8');
    try {
        result.result = eval(line);
    } catch (err) {
        result.exception = err.message;
    }
    console.log((new Buffer(JSON.stringify(result))).toString('base64'));
});
