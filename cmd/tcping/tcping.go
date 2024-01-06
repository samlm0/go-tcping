package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/samlm0/go-tcping"
)

var usage = `
Usage:

    tcping [-c count] [-i interval] [-t timeout] host

Examples:

    # ping google 5 times
    tcping -c 5 www.google.com:80

    # ping google 5 times at 500ms intervals
    tcping -c 5 -i 500ms www.google.com:80

`

func main() {
	timeout := flag.Duration("t", time.Second, "")
	interval := flag.Duration("i", time.Second, "")
	count := flag.Int("c", 0, "")
	flag.Usage = func() {
		fmt.Print(usage)
	}
	flag.Parse()
	host := flag.Arg(0)

	p, err := tcping.New(host)
	if err != nil {
		fmt.Println("Failed to ping target host:", err)
		return
	}
	p.Count = *count
	p.Interval = 1 * *interval
	p.Timeout = 1 * *timeout
	p.OnEvent = func(e *tcping.PacketEvent, err error) {
		if e.IsTimeout {
			fmt.Println("Request timeout for tcp_seq " + strconv.Itoa(e.Seq))
			return
		}
		fmt.Printf("TCP ping %s: tcp_seq=%d time=%.4f ms\n",
			e.From, e.Seq, float32(e.Latency.Microseconds())/1000)
	}
	fmt.Printf("TCPING %s (%s): \n", host, p.Host.IP)
	ctx, cancel := context.WithCancel(context.Background())

	// listen ctrl+c signal
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()
	p.Start(ctx)

	statistic := p.GetStatistic()
	fmt.Printf("\n--- %v ping statistics ---\n", host)
	fmt.Printf("%d packets transmitted, %d received, %.2f%% packet loss, time %v ms\n",
		statistic.SendCount, statistic.ReceivedCount, (1-float32(statistic.ReceivedCount)/float32(statistic.SendCount))*100, statistic.TimeTotal.Milliseconds())
	fmt.Printf("rtt min/avg/max/mdev = %.2f/%.2f/%.2f/%.2f ms\n",
		float32(statistic.TimeMin.Microseconds())/1000,
		float32(statistic.TimeAvg.Microseconds())/1000,
		float32(statistic.TimeMax.Microseconds())/1000,
		float32(statistic.TimeMdev.Microseconds())/1000,
	)
}
