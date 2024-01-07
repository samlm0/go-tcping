package tcping

import (
	"context"
	"errors"
	mrand "math/rand"
	"net"
	"time"
)

type Pinger struct {
	Host      *net.IPAddr
	Interval  time.Duration
	Timeout   time.Duration
	Count     int
	id        uint16
	OnEvent   func(*PacketEvent, error)
	outbound  *net.IPAddr
	statistic *PacketStatistic
}

type PacketEvent struct {
	IsTimeout bool
	From      string
	Seq       int
	Latency   time.Duration
}

type PacketStatistic struct {
	SendCount     int           `json:"send_count"`
	ReceivedCount int           `json:"received_count"`
	LossedCount   int           `json:"lossed_count"`
	TimeTotal     time.Duration `json:"time_total"`
	TimeMax       time.Duration `json:"time_max"`
	TimeMin       time.Duration `json:"time_min"`
	TimeAvg       time.Duration `json:"time_avg"`
	TimeMdev      time.Duration `json:"time_mdev"`
}

func New(target string) (*Pinger, error) {

	addr, err := net.ResolveIPAddr("ip:tcp", target)
	if err != nil {
		return nil, err
	}

	if len(addr.String()) == 0 {
		return nil, errors.New("failed to resolve target host")
	}

	p := &Pinger{
		Host:     addr,
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
	p.sendPacket(i, ctx)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			i++
			p.sendPacket(i, ctx)
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
