package main

import (
	"context"
	"flag"
	"log"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/target/strelka/src/go/api/strelka"
	"github.com/target/strelka/src/go/pkg/rpc"
	"github.com/target/strelka/src/go/pkg/structs"
)

func main() {
	frontendUrl := flag.String("s", "127.0.0.1:57314", "url for the strelka frontend server")
	connCert := flag.String("c", "", "path to connection certificate")
	logPath := flag.String("l", "strelka-oneshot.log", "path to response log file, - for stdout")
	scanFile := flag.String("f", "", "file to submit for scanning")
	scanTimeout := flag.Int("t", 60, "scanning timeout in seconds")

	flag.Parse()

	// scanFile is mandatory
	if *scanFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	serv := *frontendUrl
	auth := rpc.SetAuth(*connCert)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	conn, err := grpc.DialContext(ctx, serv, auth, grpc.WithBlock())
	if err != nil {
		log.Fatalf("failed to connect to %s: %v", serv, err)
	}
	defer conn.Close()

	var wgResponse sync.WaitGroup

	frontend := strelka.NewFrontendClient(conn)
	responses := make(chan *strelka.ScanResponse, 100)
	defer close(responses)

	wgResponse.Add(1)
	go func() {
	    if *logPath == "-" {
	        rpc.PrintResponses(responses)
	    } else {
	        rpc.LogResponses(responses, *logPath)
	    }
		wgResponse.Done()
	}()

	client := "go-oneshot"

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("failed to retrieve hostname: %v", err)
	}

	request := &strelka.Request{
		Client:     client,
		Source:     hostname,
		Gatekeeper: false,
	}

	req := structs.ScanFileRequest{
		Request: request,
		Attributes: &strelka.Attributes{
			Filename: *scanFile,
		},
		Chunk:  32768,
		Delay:  time.Second * 0,
		Delete: false,
	}

	rpc.ScanFile(frontend, time.Second * time.Duration(*scanTimeout), req, responses)

	responses <- nil
	wgResponse.Wait()
}
