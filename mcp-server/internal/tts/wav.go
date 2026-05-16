package tts

import (
	"bytes"
	"encoding/binary"
	"io"
)

// WritePCMAsWAV writes a canonical 44-byte RIFF/WAVE header followed by raw
// PCM samples. It mirrors write_wav in tts-remote-server.py: mono, 16-bit,
// little-endian, at the given sample rate.
func WritePCMAsWAV(w io.Writer, pcm []byte, sampleRate int) error {
	const (
		numChannels   = 1
		bitsPerSample = 16
	)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataLen := len(pcm)
	riffLen := 36 + dataLen

	var hdr bytes.Buffer
	hdr.WriteString("RIFF")
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(riffLen))
	hdr.WriteString("WAVE")
	hdr.WriteString("fmt ")
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(16)) // PCM fmt chunk size
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(1))  // audio format = PCM
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(numChannels))
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(bitsPerSample))
	hdr.WriteString("data")
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(dataLen))

	if _, err := w.Write(hdr.Bytes()); err != nil {
		return err
	}
	_, err := w.Write(pcm)
	return err
}
