package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"time"
)

var (
	backlog = flag.Int("backlog", 65536, "")
	backoff = flag.Duration("backoff", 100*time.Millisecond, "")
	buffer  = flag.Int("buffer", 65536, "")
	tcp     = flag.String("tcp", ":9639", "")
)

func init() {
	flag.Parse()

	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
}

func startTCP(ch chan<- net.Conn) error {
	s, err := net.Listen("tcp", *tcp)
	if err != nil {
		return err
	}

	go func() {
		for {
			if conn, err := s.Accept(); err == nil {
				ch <- conn
			} else {
				log.Println(err)
			}
		}
	}()
	return nil
}

func main() {
	var (
		conns   = make(chan net.Conn, 128)
		lines   = make(chan string, *backlog)
		outputs = map[string]chan string{}
	)
	for _, v := range flag.Args() {
		outputs[v] = make(chan string, *backlog)
		go func(addr string, ch chan string) {
			var (
				err    error
				sink   net.Conn
				writer *bufio.Writer
			)
			for {
				select {
				case line := <-ch:
					if sink == nil {
						if sink, err = net.Dial("tcp", addr); err != nil {
							sink = nil
							log.Println(err)
							time.Sleep(*backoff)
						}
						if sink != nil {
							writer = bufio.NewWriterSize(sink, *buffer)
						}
					}
					if sink != nil {
						if _, err := writer.Write([]byte(line)); err != nil {
							writer.Flush()
							sink = nil
							log.Println(err)
							time.Sleep(*backoff)
						}
					}
				}
			}
		}(v, outputs[v])
	}

	err := startTCP(conns)
	if err != nil {
		log.Fatalln(err)
	}

	alive := true
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	for alive {
		select {
		case <-sigCh:
			alive = false

		case c := <-conns:
			go func() {
				scanner := bufio.NewScanner(c)
				for scanner.Scan() {
					text := scanner.Text()
					if len(text) > 0 {
						lines <- text + "\n"
					}
				}
				c.Close()
			}()

		case line := <-lines:
			for _, ch := range outputs {
				select {
				case ch <- line:
				default:
				}
			}

		} // select
	} // for
}
