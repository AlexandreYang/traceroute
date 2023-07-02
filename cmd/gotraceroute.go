package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/aeden/traceroute"
	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"log"
	"net"
	"sort"
	"strings"
)

func printHop(hop traceroute.TracerouteHop) {
	addr := fmt.Sprintf("%v.%v.%v.%v", hop.Address[0], hop.Address[1], hop.Address[2], hop.Address[3])
	hostOrAddr := addr
	if hop.Host != "" {
		hostOrAddr = hop.Host
	}
	if hop.Success {
		fmt.Printf("%-3d %v (%v)  %v\n", hop.TTL, hostOrAddr, addr, hop.ElapsedTime)
	} else {
		fmt.Printf("%-3d *\n", hop.TTL)
	}
}

func address(address [4]byte) string {
	return fmt.Sprintf("%v.%v.%v.%v", address[0], address[1], address[2], address[3])
}

func main() {
	var m = flag.Int("m", traceroute.DEFAULT_MAX_HOPS, `Set the max time-to-live (max number of hops) used in outgoing probe packets (default is 64)`)
	var f = flag.Int("f", traceroute.DEFAULT_FIRST_HOP, `Set the first used time-to-live, e.g. the first hop (default is 1)`)
	var q = flag.Int("q", 1, `Set the number of probes per "ttl" to nqueries (default is one probe).`)
	var t = flag.Int("t", 3, `Times`)

	flag.Parse()
	hosts := flag.Args()
	options := traceroute.TracerouteOptions{}
	options.SetRetries(*q - 1)
	options.SetMaxHops(*m + 1)
	options.SetFirstHop(*f)
	times := *t

	fmt.Printf("hosts %v\n", hosts)

	var allHops [][]traceroute.TracerouteHop
	for _, host := range hosts {

		ipAddr, err := net.ResolveIPAddr("ip", host)
		if err != nil {
			return
		}

		fmt.Printf("traceroute to %v (%v), %v hops max, %v byte packets\n", host, ipAddr, options.MaxHops(), options.PacketSize())

		hostHops := getHops(options, times, err, host)
		printHops(hostHops)
		allHops = append(allHops, hostHops...)
	}

	graph(allHops)
}

func printHops(allhops [][]traceroute.TracerouteHop) {
	//for _, hop := range hops {
	//	printHop(hop)
	//}
	combinedHops := []traceroute.TracerouteHop{}
	for _, hops := range allhops {
		combinedHops = append(combinedHops, hops...)
	}
	replies := make(map[int][]traceroute.TracerouteHop)
	for _, reply := range combinedHops {
		replies[reply.TTL] = append(replies[reply.TTL], reply)
	}

	hops := []int{}
	for hop := range replies {
		hops = append(hops, hop)
	}
	sort.Ints(hops)
	for _, hop := range hops {
		replyList := replies[hop]
		//fmt.Printf("%d\n", hopTTL)
		prevAddr := ""
		prevTTL := 0
		hopByAddr := make(map[[4]byte][]traceroute.TracerouteHop)
		for _, hop := range replyList {
			hopByAddr[hop.Address] = append(hopByAddr[hop.Address], hop)
		}
		for _, hops := range hopByAddr {
			for _, hop := range hops {
				addr := fmt.Sprintf("%v.%v.%v.%v", hop.Address[0], hop.Address[1], hop.Address[2], hop.Address[3])
				hostOrAddr := addr
				if hop.Host != "" {
					hostOrAddr = hop.Host
				}
				printAddr := fmt.Sprintf("%v (%v)", hostOrAddr, addr)
				if hop.Success {
					if hostOrAddr == prevAddr {
						fmt.Printf(" %v", hop.ElapsedTime)
					} else {
						ttl := fmt.Sprintf("%d", hop.TTL)
						if hop.TTL == prevTTL {
							ttl = strings.Repeat(" ", len(ttl))
						}
						fmt.Printf("%s  %v  %v", ttl, printAddr, hop.ElapsedTime)
						prevTTL = hop.TTL
					}
				} else {
					fmt.Printf("   *")
				}
				prevAddr = hostOrAddr
			}
			fmt.Printf("\n")
		}

	}
}

func getHops(options traceroute.TracerouteOptions, times int, err error, host string) [][]traceroute.TracerouteHop {
	fmt.Printf("options %+v\n\n", options)
	allhops := [][]traceroute.TracerouteHop{}
	for i := 0; i < times; i++ {
		hops := []traceroute.TracerouteHop{}
		c := make(chan traceroute.TracerouteHop, 0)
		go func() {
			for {
				hop, ok := <-c
				if !ok {
					fmt.Println()
					return
				}
				printHop(hop)
				hops = append(hops, hop)
			}
		}()

		//fmt.Printf("== Round %d ==\n", i)
		//time.Sleep(50 * time.Millisecond)
		_, err = traceroute.Traceroute(host, &options, c)
		if err != nil {
			fmt.Printf("Error: ", err)
		}
		allhops = append(allhops, hops)
	}
	return allhops
}
func graph(allhops [][]traceroute.TracerouteHop) {
	g := graphviz.New()
	graph, err := g.Graph()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := graph.Close(); err != nil {
			log.Fatal(err)
		}
		g.Close()
	}()
	//n, err := graph.CreateNode("n")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//m, err := graph.CreateNode("m")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//e, err := graph.CreateEdge("e", n, m)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//e.SetLabel("e")

	for _, hops := range allhops {
		var prevNode *cgraph.Node
		for _, hop := range hops {
			ipaddr := net.IP(hop.Address[:]).String()
			var nodeName string
			if hop.Host != "" {
				nodeName = fmt.Sprintf("%s\n(%s)", hop.Host, ipaddr)
			} else {
				nodeName = ipaddr
			}
			curNode, err := graph.CreateNode(nodeName)
			if err != nil {
				log.Fatal(err)
			}
			if prevNode != nil {
				e, err := graph.CreateEdge("edge name", prevNode, curNode)
				if err != nil {
					log.Fatal(err)
				}
				e.SetLabel(fmt.Sprintf("%.1fms", float64(hop.ElapsedTime.Microseconds())/1000.0))
			}
			prevNode = curNode
		}
	}

	var buf bytes.Buffer
	if err := g.Render(graph, "dot", &buf); err != nil {
		log.Fatal(err)
	}
	fmt.Println(buf.String())

	if err := g.Render(graph, graphviz.PNG, &buf); err != nil {
		log.Fatal(err)
	}

	// write to file
	if err := g.RenderFilename(graph, graphviz.PNG, "/tmp/graph.png"); err != nil {
		log.Fatal(err)
	}
}
