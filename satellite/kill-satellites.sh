#!/bin/bash

kill `ps aux | grep mock_satellite.py | tr -s ' ' | cut -d " " -f 2 | tr '\n' ' '`
