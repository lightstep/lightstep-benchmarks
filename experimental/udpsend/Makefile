GO=go

all: build

report.pb.go: report.proto
	docker run --rm -v $(shell pwd):/input:ro -v $(shell pwd):/output \
	  lightstep/protoc:latest \
	  protoc --go_out=plugins=grpc:/output --proto_path=/input /input/report.proto

build: report.pb.go udpsend.go
	${GO} build udpsend.go report.pb.go

# gcloud compute ssh --zone=us-central1-f --project=lightstep-dev jmacd-experiment
cloud:
	GOOS=linux go build ./udpsend.go  ./report.pb.go
	gcloud compute copy-files --project=lightstep-dev --zone=us-central1-f ./udpsend $(USER)-experiment:
