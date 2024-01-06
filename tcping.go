package tcping

import (
	"context"
	"errors"
	"fmt"
	mrand "math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	tcpsharker "github.com/tevino/tcp-shaker"
)

type Pinger struct {
	Host     *net.IPAddr
	Port     int
	Interval time.Duration
	Timeout  time.Duration
	Count    int
	id       uint16
	OnEvent  func(*PacketEvent, error)

	statistic *PacketStatistic
}

type PacketEvent struct {
	IsTimeout bool
	From      string
	Seq       int
	Latency   time.Duration
}

type PacketStatistic struct {
	SendCount     int
	ReceivedCount int
	LossedCount   int
	TimeTotal     time.Duration
	TimeMax       time.Duration
	TimeMin       time.Duration
	TimeAvg       time.Duration
	TimeMdev      time.Duration
}

var realPinger = tcpsharker.NewChecker()

var isInit = false

func New(target string) (*Pinger, error) {
	if !isInit {
		go func() {
			if err := realPinger.CheckingLoop(context.Background()); err != nil {
				fmt.Println("checking loop stopped due to fatal error: ", err)
			}
		}()
		<-realPinger.WaitReady()
		isInit = true
	}

	tmp := strings.Split(target, ":")
	if len(tmp) != 2 {
		return nil, errors.New("invalid target")
	}
	addr, err := net.ResolveIPAddr("ip:tcp", tmp[0])
	if err != nil {
		return nil, err
	}
	port, _ := strconv.Atoi(tmp[1])
	if !(port >= 1 && port <= 65535) {
		return nil, errors.New("invalid target")
	}

	if len(addr.String()) == 0 {
		return nil, errors.New("failed to resolve target host")
	}

	p := &Pinger{
		Host:     addr,
		Port:     port,
		Interval: 1 * time.Second,
		Timeout:  5 * time.Second,
		Count:    10,
		statistic: &PacketStatistic{
			SendCount:     0,
			ReceivedCount: 0,
			LossedCount:   0,
			TimeTotal:     0,
			TimeMax:       0,
			TimeMin:       0,
			TimeAvg:       0,
			TimeMdev:      0,
		},
	}

	p.id = uint16(mrand.Int())
	return p, nil
}

func (p *Pinger) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ticker := time.NewTicker(p.Interval)
	i := 1
	p.sendPacket(i)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			i++
			p.sendPacket(i)
			if p.Count == 0 {
				continue
			}

			if i+1 > p.Count {
				ticker.Stop()
				return
			}
		}
	}
}

func (p *Pinger) GetStatistic() PacketStatistic {
	return *p.statistic
}

func (p *Pinger) updateStatistic(e *PacketEvent) {
	p.statistic.SendCount++
	if e.IsTimeout {
		p.statistic.LossedCount++
	} else {
		p.statistic.ReceivedCount++
	}
	p.statistic.TimeTotal += e.Latency
	if p.statistic.TimeTotal != 0 && p.statistic.ReceivedCount != 0 {
		p.statistic.TimeAvg = p.statistic.TimeTotal / time.Duration(p.statistic.ReceivedCount)
	}
	if p.statistic.TimeMax < e.Latency {
		p.statistic.TimeMax = e.Latency
	}

	if p.statistic.TimeMin == 0 || p.statistic.TimeMin > e.Latency {
		p.statistic.TimeMin = e.Latency
	}

	p.statistic.TimeMdev = p.statistic.TimeMax - p.statistic.TimeMin
}

func (p *Pinger) sendPacket(seq int) {
	timeA := time.Now()
	err := realPinger.CheckAddr(p.Host.String()+":"+strconv.Itoa(p.Port), p.Timeout)
	timeB := time.Now()
	var event *PacketEvent
	if err == nil {
		event = &PacketEvent{
			Seq:     seq,
			From:    p.Host.String(),
			Latency: timeB.Sub(timeA),
		}
	} else {
		event = &PacketEvent{Seq: seq, IsTimeout: true}
	}
	p.updateStatistic(event)
	if p.OnEvent != nil {
		p.OnEvent(event, err)
	}
}
