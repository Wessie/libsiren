package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/Wessie/libsiren"
)

func main() {
	meta := flag.Bool("metadata", false, "try to receive metadata, will be printed to stderr")
	noaudio := flag.Bool("noaudio", false, "discard audio data, output meta to stdout instead")
	flag.Parse()

	if flag.Arg(0) == "" {
		log.Fatal("missing url argument")
	}

	url := flag.Arg(0)

	opt := libsiren.Options{
		Metadata: *meta,
	}

	c, err := libsiren.Connect(url, &opt)
	if err != nil {
		log.Fatal(err)
	}

	if *meta {
		var metaout io.Writer = os.Stderr
		if *noaudio {
			metaout = os.Stdout
		}

		go func() {
			for m := range c.Metadata {
				fmt.Fprintf(metaout, "metadata: %v\n", m)
			}
		}()
	}

	if *noaudio {
		io.Copy(ioutil.Discard, c.Response.Body)
	} else {
		io.Copy(os.Stdout, c.Response.Body)
	}
}
