GO=go

all: build

build: udpsend.go
	${GO} build udpsend.go

# gcloud compute ssh --zone=us-central1-f --project=lightstep-dev jmacd-experiment
cloud:
	GOOS=linux go build ./udpsend.go  ./report.pb.go
	gcloud compute copy-files --project=lightstep-dev --zone=us-central1-f ./udpsend $(USER)-experiment:
