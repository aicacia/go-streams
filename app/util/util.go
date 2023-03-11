package util

import (
	"bytes"
	"crypto/tls"
	"encoding/gob"
	"hash/fnv"
	"log"
	"net/http"
	"strings"

	"github.com/deepch/vdk/av"
)

func ToBytes(e any) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(e)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil

}

func FromBytes(b []byte, e any) error {
	var buf bytes.Buffer
	_, err := buf.Write(b)
	if err != nil {
		return err
	}
	dec := gob.NewDecoder(&buf)
	return dec.Decode(e)
}

func Contains[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func IsAudioOnly(codecs []av.CodecData) bool {
	for _, codec := range codecs {
		if codec.Type().IsVideo() {
			return false
		}
	}
	return true
}

func FuzzyEquals(
	query string,
	text string,
) bool {
	if len(query) > len(text) {
		return false
	} else {
		return strings.Contains(strings.ToLower(text), strings.ToLower(query))
	}
}

func Hash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func CodecsToStrings(codecs []av.CodecData) []string {
	var out []string
	for _, codec := range codecs {
		if codec.Type() != av.H264 && codec.Type() != av.PCM_ALAW && codec.Type() != av.PCM_MULAW && codec.Type() != av.OPUS {
			log.Println("Codec Not Supported WebRTC ignore this track", codec.Type())
			continue
		}
		if codec.Type().IsVideo() {
			out = append(out, "video")
		} else {
			out = append(out, "audio")
		}
	}
	return out
}

func NewInsecureClient() http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return http.Client{Transport: tr}
}

func ReverseBytes(s []byte) []byte {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
