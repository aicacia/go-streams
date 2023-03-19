package playback

import (
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aicacia/streams/app/format"
	"github.com/deepch/vdk/av"
)

const playerChanSize = 1024

type Player struct {
	mutex       sync.RWMutex
	folder      string
	currentTime *time.Time
	direction   int8
	rate        float32
	demuxers    []*format.Demuxer
	stream      chan *av.Packet
}

func NewPlayer(folder string, currentTime *time.Time, direction int8, rate float32) (*Player, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	var demuxers []*format.Demuxer
	for _, entry := range entries {
		if !entry.IsDir() {
			idxStr, err := strconv.ParseInt(entry.Name(), 10, 8)
			if err != nil {
				return nil, err
			}
			idx := int8(idxStr)
			demuxer, err := format.NewDemuxer(folder, idx)
			if err != nil {
				return nil, err
			}
			demuxers = append(demuxers, demuxer)
		}
	}
	return &Player{
		folder:      folder,
		currentTime: currentTime,
		direction:   direction,
		rate:        rate,
		demuxers:    demuxers,
		stream:      make(chan *av.Packet, playerChanSize),
	}, nil
}

func (p *Player) Start() {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	for idx := range p.demuxers {
		go p.playDemuxer(int8(idx))
	}
}

func (p *Player) Codecs() []av.CodecData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	var codecs []av.CodecData
	for _, demuxer := range p.demuxers {
		codecs = append(codecs, demuxer.Codec())
	}
	return codecs
}

func (p *Player) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _, demuxer := range p.demuxers {
		if demuxer != nil {
			demuxer.Close()
		}
	}
	close(p.stream)
	return nil
}

func (p *Player) Stream() chan *av.Packet {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.stream
}

func (p *Player) Direction() int8 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.direction
}

func (p *Player) readPacket(idx int8) (*av.Packet, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.demuxers[idx].ReadPacket(p.direction)
}

func (p *Player) close(idx int8) (err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if demuxer := p.demuxers[idx]; demuxer != nil {
		err = demuxer.Close()
		p.demuxers[idx] = nil
	}
	for _, demuxer := range p.demuxers {
		if demuxer != nil {
			return
		}
	}
	defer p.Close()
	return
}

func (p *Player) playDemuxer(idx int8) {
	for {
		packet, err := p.readPacket(idx)
		if err != nil {
			log.Println(err)
			break
		}
		p.stream <- packet
		time.Sleep(packet.Duration)
	}
	p.close(idx)
}
