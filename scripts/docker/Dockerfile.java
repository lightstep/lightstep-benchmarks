FROM java:latest

WORKDIR /data

COPY controller /data
COPY config.json /data
COPY lightstep-benchmark.jar /data

ENV CLASSPATH /data/lightstep-benchmark.jar

CMD ["./controller", "--logtostderr", "-v=1", "--client=java", "--config=./config.json"]
