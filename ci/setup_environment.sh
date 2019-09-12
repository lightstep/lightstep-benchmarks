#!/bin/bash

set -e

apt-get update 
apt-get install --no-install-recommends --no-install-suggests -y \
                build-essential \
                cmake \
                pkg-config \
                git \
                ca-certificates \
                curl \
                automake \
                autogen \
                autoconf \
                libtool \
                gnupg2 \
                ssh \
                vim \
                wget \
                python python-setuptools python-pip \
                zlib1g-dev \
                libffi-dev \
                python3.7 python3.7-dev \
                python3-distutils \
                zip \
                unzip \
                trickle \
                sudo \
                npm \
                golang
update-alternatives --remove python /usr/bin/python2
update-alternatives --install /usr/bin/python python /usr/bin/python3.7 10

update-alternatives --remove python3 /usr/bin/python3
update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.7 10

curl https://bootstrap.pypa.io/get-pip.py -o get-pip.py
python get-pip.py
