/*
    Maxime Piraux's master's thesis
    Copyright (C) 2017-2018  Maxime Piraux

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU Affero General Public License version 3
	as published by the Free Software Foundation.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU Affero General Public License for more details.

    You should have received a copy of the GNU Affero General Public License
    along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/
package main

import (
	"github.com/davecgh/go-spew/spew"
	m "github.com/mpiraux/master-thesis"
	"flag"
	"strings"
	"fmt"
	"io/ioutil"
	"os"
	"encoding/json"
	"github.com/mpiraux/master-thesis/scenarii"
)

func main() {
	address := flag.String("address", "", "The address to connect to")
	useIPv6 := flag.Bool("6", false, "Use IPV6")
	url := flag.String("url", "/index.html", "The URL to request")
	flag.Parse()
	conn, err := m.NewDefaultConnection(*address, (*address)[:strings.LastIndex(*address, ":")], nil, *useIPv6)
	if err != nil {
		panic(err)
	}
	pcap, err := m.StartPcapCapture(conn)
	if err != nil {
		panic(err)
	}
	defer func() {
		pcap, err := m.StopPcapCapture(pcap)
		if err == nil {
			ioutil.WriteFile("/tmp/http_get.pcap", pcap, os.ModePerm)
		}
	}()

	trace := m.NewTrace("http_get", 1, *address)
	trace.AttachTo(conn)
	defer func() {
		trace.Complete(conn)
		err = trace.AddPcap(pcap)
		if err != nil {
			trace.Results["pcap_error"] = err.Error()
		}

		var t []m.Trace
		t = append(t, *trace)
		out, err := json.Marshal(t)
		if err != nil {
			println(err)
		}
		println(string(out))
	}()

	spew.Dump(conn.DestinationCID)

	packets, err := scenarii.CompleteHandshake(conn)
	if err != nil {
		spew.Dump(packets, err)
	}

	spew.Dump(conn.ClientRandom)
	spew.Dump(conn.ExporterSecret)

	conn.Streams[4] = &m.Stream{}
	streamFrame := m.NewStreamFrame(4, conn.Streams[4], []byte(fmt.Sprintf("GET %s\r\n", *url)), true)
	ackFrame := conn.GetAckFrame()

	protectedPacket := m.NewProtectedPacket(conn)
	protectedPacket.Frames = append(protectedPacket.Frames, streamFrame, ackFrame)
	conn.SendProtectedPacket(protectedPacket)

	for {
		packets, err, _ := conn.ReadNextPackets()
		if err != nil {
			spew.Dump(err)
			break
		}

		spew.Dump("---> Received packets")
		spew.Dump(packets)

		for _, packet := range packets {
			if packet.ShouldBeAcknowledged() {
				protectedPacket = m.NewProtectedPacket(conn)
				protectedPacket.Frames = append(protectedPacket.Frames, conn.GetAckFrame())
				spew.Dump("<--- Send ack packet")
				spew.Dump(protectedPacket)
				conn.SendProtectedPacket(protectedPacket)
			}
		}
	}
}