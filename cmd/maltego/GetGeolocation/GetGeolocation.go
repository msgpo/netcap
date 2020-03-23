package main

import (
	maltego "github.com/dreadl0ck/netcap/cmd/maltego/maltego"
	"github.com/dreadl0ck/netcap/types"
	"strconv"
)

func main() {
	maltego.IPTransform(
		nil,
		func(lt maltego.LocalTransform, trx *maltego.MaltegoTransform, profile  *types.DeviceProfile, minPackets, maxPackets uint64, profilesFile string, mac string, ipaddr string) {
			if profile.MacAddr == mac {
				for _, ip := range profile.Contacts {
					if ip.Addr == ipaddr {
						if ip.Geolocation == "" {
							continue
						}
						addGeolocation(trx, ip, minPackets, maxPackets)
					}
				}
				for _, ip := range profile.DeviceIPs {
					if ip.Addr == ipaddr {
						if ip.Geolocation == "" {
							continue
						}
						addGeolocation(trx, ip, minPackets, maxPackets)
					}
				}
			}
		},
	)
}

func addGeolocation(trx *maltego.MaltegoTransform, ip *types.IPProfile, minPackets, maxPackets uint64) {
	ent := trx.AddEntity("maltego.Location", ip.Geolocation)
	ent.SetType("maltego.Location")
	ent.SetValue(ip.Geolocation)

	// di := "<h3>Geolocation</h3><p>Timestamp: " + ip.TimestampFirst + "</p>"
	// ent.AddDisplayInformation(di, "Netcap Info")

	ent.SetLinkLabel(strconv.FormatInt(ip.NumPackets, 10) + " pkts")
	ent.SetLinkColor("#000000")
	ent.SetLinkThickness(maltego.GetThickness(uint64(ip.NumPackets), minPackets, maxPackets))
}