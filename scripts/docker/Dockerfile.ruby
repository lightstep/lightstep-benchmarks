FROM ruby:2.3

WORKDIR /data

RUN apt-get update && \
    apt-get install -qqy \
	ca-certificates && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY controller ./
COPY benchmark.rb ./
COPY lightstep.gem ./

RUN gem install ./lightstep.gem
