//go:build linux

package tcping

import (
	"context"
	"encoding/hex"
	mrand "math/rand"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func getOutboundIP(dstIP net.IP) (*net.IPAddr, error) {
	conn, err := net.Dial("udp", dstIP.String()+":80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	udpAddr := conn.LocalAddr().(*net.UDPAddr)
	ipAddr := &net.IPAddr{
		IP:   udpAddr.IP,
		Zone: udpAddr.Zone,
	}
	return ipAddr, nil
}

func (p *Pinger) reportTimeout(event *PacketEvent, err error) {
	event.IsTimeout = true
	p.updateStatistic(event)
	if p.OnEvent != nil {
		p.OnEvent(event, err)
	}
}

func (p *Pinger) sendPacket(seq int, ctx context.Context) {
	var err error

	if p.outbound == nil {
		p.outbound, err = getOutboundIP(p.Host.IP)
	}
	srcIP := *p.outbound
	if err != nil {
		panic(err)
	}

	ipLayer := &layers.IPv4{
		SrcIP: srcIP.IP,
		DstIP: p.Host.IP,
	}

	tcpHeader := &layers.TCP{
		SrcPort: layers.TCPPort(mrand.Int()),
		DstPort: layers.TCPPort(0),
		Seq:     mrand.Uint32(),
		Ack:     mrand.Uint32(),
		ACK:     true,
		RST:     false,
		Window:  1505,
	}

	tcpHeader.SetNetworkLayerForChecksum(ipLayer)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	if err := tcpHeader.SerializeTo(buf, opts); err != nil {
		panic(err)
	}
	event := &PacketEvent{
		Seq:  seq,
		From: p.Host.IP.String(),
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "ip4:tcp", p.Host.IP.String())
	if err != nil {
		p.reportTimeout(event, err)
		return
	}
	defer conn.Close()

	deadline := time.Now().Add(p.Timeout)
	conn.SetDeadline(deadline)

	timeA := time.Now()
	_, err = conn.Write(buf.Bytes())
	if err != nil {
		p.reportTimeout(event, err)
		return
	}

	recvBuf := make([]byte, 4096)
	n, err := conn.Read(recvBuf)
	timeB := time.Now()
	if err != nil {
		p.reportTimeout(event, err)
		return
	}
	ackNum := hex.EncodeToString(buf.Bytes())[16:20]
	seqNum := hex.EncodeToString(recvBuf[:n])[48:52]

	if ackNum == seqNum {
		event.Latency = timeB.Sub(timeA)
		p.updateStatistic(event)
		if p.OnEvent != nil {
			p.OnEvent(event, nil)
		}
		return
	}

	p.reportTimeout(event, err)
}
