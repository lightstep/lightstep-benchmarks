package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

const (
	testStorageBucket = "lightstep-client-benchmarks"
)

var (
	testName = flag.String("test", "", "Name of the test")
)

func usage() {
	fmt.Println("usage: %s --name=<...>", os.Args[0])
	os.Exit(1)
}

type summarizer struct {
}

func main() {
	flag.Parse()

	if *testName == "" {
		usage()
	}

	ctx := context.Background()
	gcpClient, err := google.DefaultClient(ctx, storage.ScopeFullControl)
	if err != nil {
		glog.Fatal("GCP Default client: ", err)
	}
	storageClient, err := storage.NewClient(ctx, cloud.WithBaseHTTP(gcpClient))
	if err != nil {
		log.Fatal("GCP Storage client", err)
	}
	defer storageClient.Close()
	bucket := storageClient.Bucket(testStorageBucket)

	olist, err := bucket.List(ctx, nil)
	if err != nil {
		log.Fatal("GCP Storage client", err)
	}
	if olist.Next != nil {
		log.Fatal("GCP unhandled Next result field: ", olist)
	}
	s := summarizer{}
	prefix := *testName + "/"
	for _, obj := range olist.Results {
		if !strings.HasPrefix(obj.Name, prefix) {
			continue
		}
		if err := s.getResults(bucket, obj.Name); err != nil {
			log.Fatal("Couldn't read results: ", obj.Name)
		}
	}

}

func (s *summarizer) getResults(b *storage.BucketHandle, name string) {

}
