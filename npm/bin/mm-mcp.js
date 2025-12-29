#!/usr/bin/env node
'use strict';

const { spawnSync } = require('child_process');
const path = require('path');

const binName = process.platform === 'win32' ? 'mm-mcp.exe' : 'mm-mcp';
const binPath = path.join(__dirname, binName);

const result = spawnSync(binPath, process.argv.slice(2), {
  stdio: 'inherit'
});

if (result.error) {
  console.error(`mini-msg: failed to run ${binName}: ${result.error.message}`);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
