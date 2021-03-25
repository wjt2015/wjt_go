package pcap_base

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"log"
)

func PcapFunc() {
	version := pcap.Version()

	log.Printf("pcap_version=%+v\n", version)
	ifs, err := pcap.FindAllDevs()
	log.Printf("ifs=%+v;err=%+v\n", ifs, err)

	handle, err := pcap.OpenLive("en0", 1<<20, false, -1)
	log.Printf("pcap_err=%+v\n", err)
	if err != nil {
		log.Panicf("OpenLive error!err=%+v\n", err)
		return
	}
	//log.Printf("handle=%+v;err=%+v;\n",handle,err)
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	packet, err := packetSource.NextPacket()

	log.Printf("一个包;packet=%+v\n", packet)

	chanForPacket := packetSource.Packets()

	i := 0
	//遍历所有抓到的包;
	for packet := range chanForPacket {
		log.Printf("i=%+v;packet=%+v\n", i, packet)

		//解析包的层:

		for j,layer:=range packet.Layers(){
			layerType:=layer.LayerType()
			layerTypeStr:=layerType.String()
			log.Printf("i=%+v;j=%+v;layer=%+v;layerType=%+v;layerTypeStr=%+v;\n",i,j,layer,layerType,layerTypeStr)



		}
		i++
	}

}
