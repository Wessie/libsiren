package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wessie/libsiren"
)

var timeoutDuration int64 = int64(time.Second)
var timeoutDelta int64 = int64(time.Second)
var clients = newClientMap()

func newClientMap() *clientMap {
	return &clientMap{
		clients: make(map[*libsiren.Client]int),
	}
}

type clientMap struct {
	clients map[*libsiren.Client]int
	sync.Mutex
}

func (m *clientMap) Get(key *libsiren.Client) int {
	m.Lock()
	defer m.Unlock()
	return m.clients[key]
}

func (m *clientMap) Set(key *libsiren.Client, value int) {
	m.Lock()
	defer m.Unlock()
	m.clients[key] = value
}

func (m *clientMap) Del(key *libsiren.Client) {
	m.Lock()
	defer m.Unlock()
	delete(m.clients, key)
}

func main() {
	var n int
	flag.IntVar(&n, "n", 1, "amount of clients to connect")
	flag.Parse()

	if flag.Arg(0) == "" {
		fmt.Println("missing url parameter")
		os.Exit(1)
	}

	url := flag.Arg(0)
	var toconnect = make(chan bool, n)

	go handleConnections(url, nil, toconnect)

	for i := 0; i < n; i++ {
		toconnect <- true
	}

	select {}
}

func handleConnections(url string, opt *libsiren.Options, toconnect chan bool) {
	for n := 0; ; n++ {
		<-toconnect
		go func(n int) {
			c, err := libsiren.Connect(url, nil)
			if err != nil {
				go retryConn(toconnect)
				return
			}

			clients.Set(c, n)
			handleClient(c, toconnect)
		}(n)
	}
}

func handleClient(c *libsiren.Client, reconnect chan bool) {
	log.Printf("connected client %d\n", clients.Get(c))
	n, err := io.Copy(ioutil.Discard, c.Response.Body)
	defer c.Response.Body.Close()
	defer clients.Del(c)

	if err != nil {
		log.Printf("unexpected disconnection on client %d: read %d bytes: %s\n", clients.Get(c), n, err)
	} else {
		log.Printf("disconnected client %d cleanly\n", clients.Get(c))
	}

	retryConn(reconnect)
}

func retryConn(reconnect chan bool) {
	timeout := atomic.AddInt64(&timeoutDuration, timeoutDelta)
	log.Println("reconnecting one client in %s", time.Duration(timeout))
	time.Sleep(time.Duration(timeout))
	reconnect <- true
}
