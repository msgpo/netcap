/*
 * NETCAP - Traffic Analysis Framework
 * Copyright (c) 2017-2020 Philipp Mieden <dreadl0ck [at] protonmail [dot] ch>
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package encoder

import (
	"encoding/hex"
	"fmt"
	"github.com/dreadl0ck/netcap/resolvers"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dreadl0ck/gopacket"
	"github.com/dreadl0ck/netcap/types"
	"github.com/gogo/protobuf/proto"
)

type Service struct {
	*types.Service
	sync.Mutex
}

// AtomicDeviceProfileMap contains all connections and provides synchronized access
type AtomicServiceMap struct {
	// map Server IP + Port to Service
	Items map[string]*Service
	sync.Mutex
}

// Size returns the number of elements in the Items map
func (a *AtomicServiceMap) Size() int {
	a.Lock()
	defer a.Unlock()
	return len(a.Items)
}

var (
	// ServiceStore hold all connections
	ServiceStore = &AtomicServiceMap{
		Items: make(map[string]*Service),
	}
)

func addInfo(old string, new string) string {
	if len(old) == 0 {
		return new
	} else {
		var b strings.Builder
		b.WriteString(old)
		b.WriteString(" | ")
		b.WriteString(new)
		return b.String()
	}
}

// saves the banner for a TCP service to the filesystem
// and limits the length of the saved data to the BannerSize value from the config
func saveTCPServiceBanner(banner []byte, ident string, firstPacket time.Time, net, transport gopacket.Flow) {

	// limit length of data
	if len(banner) >= c.BannerSize {
		banner = banner[:c.BannerSize]
	}

	// check if we already have a banner for the IP + Port combination
	ServiceStore.Lock()
	if _, ok := ServiceStore.Items[ident]; ok {
		ServiceStore.Unlock()

		// banner exists. nothing to do
		return
	}
	ServiceStore.Unlock()

	// nope. lets create a new one
	s := NewService(firstPacket.String())
	s.Banner = banner
	s.IP = net.Dst().String()
	s.Port = transport.Dst().String()

	// set flow ident, h.parent.ident is the client flow
	s.Flow = ident

	dst, err := strconv.Atoi(transport.Dst().String())
	if err == nil {
		s.Protocol = "TCP"
		s.Name = resolvers.LookupServiceByPort(dst, "tcp")
	}

	// match banner against nmap service probes
	for _, serviceProbe := range serviceProbes {
		if c.UseRE2 {
			if m := serviceProbe.RegEx.FindStringSubmatch(string(banner)); m != nil {

				s.Product = addInfo(s.Product, serviceProbe.Ident)
				s.Vendor = addInfo(s.Vendor, serviceProbe.Vendor)
				s.Version = addInfo(s.Version, serviceProbe.Version)

				if strings.Contains(serviceProbe.Version, "$1") {
					if len(m) > 1 {
						s.Version = addInfo(s.Version, strings.Replace(serviceProbe.Version, "$1", m[1], 1))
					}
				}

				if strings.Contains(serviceProbe.Hostname, "$1") {
					if len(m) > 1 {
						s.Notes = addInfo(s.Notes, strings.Replace(serviceProbe.Hostname, "$1", m[1], 1))
					}
				}

				// TODO: make a group extraction util and expand all groups in all strings properly
				if strings.Contains(serviceProbe.Info, "$1") {
					if len(m) > 1 {
						s.Product = addInfo(s.Product, strings.Replace(serviceProbe.Info, "$1", m[1], 1))
					}
				}
				if strings.Contains(s.Product, "$2") {
					if len(m) > 2 {
						s.Product = addInfo(s.Product, strings.Replace(serviceProbe.Info, "$2", m[2], 1))
					}
				}

				if c.Debug {
					fmt.Println("\n\nMATCH!", ident)
					fmt.Println(serviceProbe, "\nBanner:", "\n"+hex.Dump(banner))
				}
			}
		} else {
			if m, err := serviceProbe.RegEx2.FindStringMatch(string(banner)); err == nil && m != nil {

				s.Product = addInfo(s.Product, serviceProbe.Ident)
				s.Vendor = addInfo(s.Vendor, serviceProbe.Vendor)
				s.Version = addInfo(s.Version, serviceProbe.Version)

				if strings.Contains(serviceProbe.Version, "$1") {
					if len(m.Groups()) > 1 {
						s.Version = addInfo(s.Version, strings.Replace(serviceProbe.Version, "$1", m.Groups()[1].Captures[0].String(), 1))
					}
				}

				// TODO: make a group extraction util
				if strings.Contains(serviceProbe.Info, "$1") {
					if len(m.Groups()) > 1 {
						s.Product = addInfo(s.Product, strings.Replace(serviceProbe.Info, "$1", m.Groups()[1].Captures[0].String(), 1))
					}
				}
				if strings.Contains(s.Product, "$2") {
					if len(m.Groups()) > 2 {
						s.Product = addInfo(s.Product, strings.Replace(serviceProbe.Info, "$2", m.Groups()[2].Captures[0].String(), 1))
					}
				}

				if c.Debug {
					fmt.Println("\nMATCH!", ident)
					fmt.Println(serviceProbe, "\nBanner:", "\n"+hex.Dump(banner))
				}
			}
		}
	}

	// add new service
	ServiceStore.Lock()
	ServiceStore.Items[ident] = s
	ServiceStore.Unlock()

	statsMutex.Lock()
	reassemblyStats.numServices++
	statsMutex.Unlock()
}

// NewDeviceProfile creates a new device specific profile
func NewService(ts string) *Service {
	return &Service{
		Service: &types.Service{
			Timestamp: ts,
		},
	}
}

var serviceEncoder = CreateCustomEncoder(types.Type_NC_Service, "Service", func(d *CustomEncoder) error {
	return InitProbes()
}, func(p gopacket.Packet) proto.Message {
	return nil
}, func(e *CustomEncoder) error {

	// flush writer
	if !e.writer.IsChanWriter {
		for _, c := range ServiceStore.Items {
			c.Lock()
			e.write(c.Service)
			c.Unlock()
		}
	}
	return nil
})