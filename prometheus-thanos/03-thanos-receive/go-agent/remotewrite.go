package main

import (
	"encoding/binary"
	"math"
	"sort"
)

// Minimal hand-rolled encoder for the Prometheus remote_write wire format
// (WriteRequest -> TimeSeries -> Label/Sample), avoiding a dependency on the
// full prometheus/prometheus module just to get the generated protobuf types.
//
// Wire layout (field numbers match prompb's remote.proto / types.proto):
//   WriteRequest { repeated TimeSeries timeseries = 1; }
//   TimeSeries   { repeated Label labels = 1; repeated Sample samples = 2; }
//   Label        { string name = 1; string value = 2; }
//   Sample       { double value = 1; int64 timestamp = 2; }

type label struct {
	name  string
	value string
}

type sample struct {
	value     float64
	timestamp int64 // ms since epoch
}

type series struct {
	labels  []label
	samples []sample
}

func encodeVarint(n uint64) []byte {
	buf := make([]byte, 0, 10)
	for {
		b := byte(n & 0x7F)
		n >>= 7
		if n != 0 {
			buf = append(buf, b|0x80)
		} else {
			buf = append(buf, b)
			break
		}
	}
	return buf
}

func encodeTag(fieldNo int, wireType int) []byte {
	return encodeVarint(uint64((fieldNo << 3) | wireType))
}

func encodeLenDelimited(fieldNo int, data []byte) []byte {
	out := encodeTag(fieldNo, 2)
	out = append(out, encodeVarint(uint64(len(data)))...)
	return append(out, data...)
}

func encodeStringField(fieldNo int, s string) []byte {
	return encodeLenDelimited(fieldNo, []byte(s))
}

func encodeLabel(l label) []byte {
	b := encodeStringField(1, l.name)
	b = append(b, encodeStringField(2, l.value)...)
	return b
}

func encodeSample(s sample) []byte {
	b := encodeTag(1, 1) // wiretype 1 = fixed64
	valBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(valBytes, math.Float64bits(s.value))
	b = append(b, valBytes...)
	b = append(b, encodeTag(2, 0)...) // wiretype 0 = varint
	b = append(b, encodeVarint(uint64(s.timestamp))...)
	return b
}

func encodeTimeSeries(ts series) []byte {
	// remote_write requires labels sorted by name ascending, or the
	// receiver rejects the write with "out of order labels".
	sorted := make([]label, len(ts.labels))
	copy(sorted, ts.labels)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })

	var out []byte
	for _, l := range sorted {
		out = append(out, encodeLenDelimited(1, encodeLabel(l))...)
	}
	for _, s := range ts.samples {
		out = append(out, encodeLenDelimited(2, encodeSample(s))...)
	}
	return out
}

func encodeWriteRequest(seriesList []series) []byte {
	var out []byte
	for _, ts := range seriesList {
		out = append(out, encodeLenDelimited(1, encodeTimeSeries(ts))...)
	}
	return out
}
