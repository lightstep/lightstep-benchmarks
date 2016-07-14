FROM ruby:2.1

WORKDIR /data

RUN apt-get update && \
    apt-get install -qqy \
	ca-certificates

COPY controller /data
COPY config.json /data
COPY rbclient.rb /data
COPY Gemfile /data

RUN bundle install

CMD ["./controller", "--logtostderr", "-v=1", "--client=ruby", "--config=./config.json"]
