package util

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/gob"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

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

func TruncateToMinute(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
}

func createScanLines(delim []byte) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		for i := 0; i+len(delim) <= len(data); {
			j := i + bytes.IndexByte(data[i:], delim[0])
			if j < i {
				break
			}
			if bytes.Equal(data[j+1:j+len(delim)], delim[1:]) {
				return j + len(delim), data[0:j], nil
			}
			i = j + 1
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
}

func NewDelimScanner(r io.Reader, delim []byte) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Split(createScanLines(delim))
	return scanner
}

func CopyMap[T any](m map[string]T) map[string]T {
	cp := make(map[string]T)
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
