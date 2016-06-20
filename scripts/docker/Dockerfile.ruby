FROM ruby:2.1
WORKDIR /data
COPY controller /data
COPY rbclient.rb /data
COPY Gemfile /data
RUN bundle install
CMD ["./controller", "--logtostderr", "-v=1", "--client=python"]
