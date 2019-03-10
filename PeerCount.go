package epsp

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
)

type peerCount struct {
	Region string
	Count  uint64
}

func (p peerCount) GetRegion() string {
	return p.Region
}

func (p peerCount) GetCount() uint64 {
	return p.Count
}

// PeerCounts is count of region
type PeerCounts []peerCount

// GoogleChart makes GoogleChart JSON table.
func (p PeerCounts) GoogleChart() []byte {
	bs := []byte(`[`)
	datas := [][]byte{[]byte(`["Latitude", "Longitude", "Region", "ピア数"]`)}
	for _, v := range p {
		reglatlng := AreaForRegLatLng(v.GetRegion())
		if len(reglatlng) == 3 {
			reglatlng[2] = `"` + reglatlng[2] + `"`
			reglatlng = append(reglatlng, strconv.FormatUint(v.GetCount(), 10))
			datas = append(datas, []byte(`[`+strings.Join(reglatlng, `, `)+`]`))
		}
	}
	bs = append(bs, bytes.Join(datas, []byte(`,`))...)
	bs = append(bs, ']')
	return bs
}

// NewPeerCount is constructor of PeerCount
func NewPeerCount(recvdata2 string) (peerCountByRegion PeerCounts) {
	for _, regpeer := range strings.Split(recvdata2, `;`) {
		rp := strings.Split(regpeer, `,`)
		if peerct, err := strconv.ParseUint(rp[1], 10, 64); err == nil {
			pct := peerCount{Region: rp[0], Count: peerct}
			peerCountByRegion = append(peerCountByRegion, pct)
		}
	}
	return
}

func (p PeerCounts) String() string {

	const maxregion = 8

	pcbr := p

	sort.Slice(pcbr, func(i, j int) bool {
		return pcbr[i].GetCount() > pcbr[j].GetCount()
	})

	if len(pcbr) > maxregion {
		pcbr = pcbr[:maxregion]
	}

	var ss []string
	for i := range pcbr {
		ss = append(ss, Area(pcbr[i].GetRegion())+`:`+strconv.FormatUint(uint64(pcbr[i].GetCount()), 10))
	}
	return strings.Join(ss, `, `)
}

// NumOfAllPeers は、すべてのPeer数の合計です
func (p PeerCounts) NumOfAllPeers() (numOfAPeers uint64) {
	for _, v := range p {
		numOfAPeers += uint64(v.GetCount())
	}
	return
}
