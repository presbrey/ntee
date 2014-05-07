package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	testLines     = make(chan string, 512)
	testListeners = map[string]net.Listener{}
)

func init() {
	*buffer = 1
	*timeout = time.Second

	for i := 0; i < 4; i++ {
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		go func() {
			c, err := l.Accept()
			if err != nil {
				log.Fatalln(err)
			}
			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				text := scanner.Text()
				if len(text) > 0 {
					testLines <- text
				}
			}
			if scanner.Err() != nil {
				log.Fatalln(scanner.Err())
			}
		}()
		if err != nil {
			log.Fatalln(err)
		}
		testListeners[l.Addr().String()] = l
	}

	outs := []string{}
	for k := range testListeners {
		outs = append(outs, k)
	}
	go tee(outs)
}

func TestTee(t *testing.T) {
	var (
		conn net.Conn
		err  error
	)
	assert.NotNil(t, testListeners)

	for i := 0; i < 10; i++ {
		conn, err = net.Dial("tcp", "127.0.0.1:9639")
		if err != nil {
			if _, ok := err.(*net.OpError); ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}
		for j := 0; j < 10; j++ {
			_, err = fmt.Fprintln(conn, "asdf")
			assert.NoError(t, err)
		}
		err = nil
		break
	}
	assert.NoError(t, err)

	n := 0
	for {
		select {
		case <-testLines:
			n += 1
		case <-time.After(3 * time.Second):
			goto End
		}
	}
End:
	assert.Equal(t, n, 40)
}
