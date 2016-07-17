FROM ruby:2.1

WORKDIR /data

RUN apt-get update && \
    apt-get install -qqy \
	ca-certificates

COPY controller /data
COPY rbclient.rb /data
COPY Gemfile /data

RUN bundle install
