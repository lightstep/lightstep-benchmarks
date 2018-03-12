#!/bin/bash

git submodule update
for client in ./clients/*; do
    (cd $client && git checkout master && git pull)
done
