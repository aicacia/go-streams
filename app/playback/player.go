package playback

import (
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aicacia/streams/app/format"
	"github.com/aicacia/streams/app/rtsp"
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
	running     []bool
	closed      bool
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
		closed:      false,
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
	if !p.closed {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		for _, demuxer := range p.demuxers {
			if demuxer != nil {
				demuxer.Close()
			}
		}
		p.closed = true
		close(p.stream)
	}
	return nil
}

func (p *Player) IsClosed() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.closed
}

func (p *Player) Stream() chan *av.Packet {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.stream
}

func (p *Player) Codec(idx int8) av.CodecData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.demuxers[idx].Codec()
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

func (p *Player) setCurrentTime(t *time.Time) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.currentTime = t
}

func (p *Player) GetCurrentTime() *time.Time {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.currentTime
}

func (p *Player) getRate() float32 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.rate
}

func (p *Player) SetRate(rate float32) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.rate = rate
}

func (p *Player) close(idx int8) (err error) {
	p.mutex.RLock()
	demuxer := p.demuxers[idx]
	p.mutex.RUnlock()
	if demuxer != nil {
		err = demuxer.Close()
		p.mutex.Lock()
		p.demuxers[idx] = nil
		p.mutex.Unlock()
		if demuxer.Codec().Type().IsAudio() {
			p.mutex.RLock()
			hasVideo := false
			for _, demuxer := range p.demuxers {
				if demuxer != nil && demuxer.Codec().Type().IsVideo() {
					hasVideo = true
					break
				}
			}
			p.mutex.RUnlock()
			if hasVideo {
				return
			}
		}
		if err != nil {
			log.Println("Player demuxer close error", err)
		}
	}
	err = p.Close()
	return
}

func (p *Player) playDemuxer(idx int8) {
	isVideo := p.Codec(idx).Type().IsVideo()
	direction := p.direction
	started := false
	for {
		packet, err := p.readPacket(idx)
		if err != nil {
			log.Printf("%d codec failed to read packet %s\n", idx, err)
			break
		}
		packetTime := rtsp.GetPacketTime(packet)
		if isVideo {
			p.setCurrentTime(&packetTime)
			if !started {
				if packet.IsKeyFrame {
					started = true
				} else {
					continue
				}
			}
		}
		var sleepTime time.Duration
		if direction == PlaybackForward {
			sleepTime = packetTime.Sub(*p.GetCurrentTime()) - packet.Duration
		} else if direction == PlaybackBackward {
			sleepTime = p.GetCurrentTime().Sub(packetTime) - packet.Duration
		}
		if sleepTime > 0 {
			sleepTime = time.Duration(float32(sleepTime) / p.getRate())
			time.Sleep(sleepTime)
			if p.IsClosed() {
				break
			}
		}
		p.stream <- packet
		nextPacketTime := time.Duration(float32(packet.Duration) / p.getRate())
		time.Sleep(nextPacketTime)
		if p.IsClosed() {
			break
		}
	}
	if isVideo {
		p.Close()
	} else {
		p.close(idx)
	}
}
