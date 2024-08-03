// provides reading and writing of typed data from a byte buffer
package binBuffer

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"strconv"
	//"github.com/nickaxgit/mightyServer/graph"
)

type BinBuffer struct {
	data      []byte
	readPtr   uint32
	writePtr  uint32
	finite    bool    //if true, the buffer is NOT a ring buffer, and will panic if the writepointer exceeds the length
	LastValue float32 //handy copy of the last (float) value written
}

func NewFromSlice(data []byte, rp uint32, wp uint32, finite bool) *BinBuffer {
	return &BinBuffer{data: data, readPtr: rp, writePtr: wp, finite: false}
}

func NewFromFile(filename string) *BinBuffer {

	println("Loading file", filename)
	file, err := os.Open(filename)

	if err != nil {
		println("no such file", filename)
		return &BinBuffer{data: []byte{}, readPtr: 0}
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		println("Error getting file size", filename)
		panic("Error getting file size")
	}

	println(filename, "File size", stat.Size())
	buff := make([]byte, stat.Size())
	file.Read(buff)

	return &BinBuffer{data: buff, readPtr: 0}
}

func (bb *BinBuffer) Length() uint32 {
	return uint32(len(bb.data))
}

func (bb *BinBuffer) SetReadBackFromWrite(ro int32) {

	p := int32(bb.writePtr) + ro
	if p < 0 {
		p += int32(len(bb.data))
	}
	bb.readPtr = uint32(p)
}

func (bb *BinBuffer) WriteToFile(filename string, truncateAtWritePointer bool) {
	file, err := os.Create(filename)
	if err != nil {
		println("Error creating file", filename)
		panic("Error creating file")
	}
	defer file.Close()
	end := len(bb.data)
	if truncateAtWritePointer {
		end = int(bb.writePtr)
	}
	println("Writing", end, "bytes to", filename)
	n, err := file.Write(bb.data[0:end])
	if err != nil {
		println("Error writing to file", filename)
		panic("Error writing to file")
	}
	println("Wrote ", n, "bytes to", filename)
}

func (bb *BinBuffer) GetWritePtr() uint32 {
	return bb.writePtr
}

func (bb *BinBuffer) SVGPath(min float32, max float32) string {

	var b bytes.Buffer //an efficient way to concatenate strings

	println("bb.writePtr", bb.writePtr)

	bb.SetReadBackFromWrite(-800 * 4) //look back 800 samples

	// startByte := int(bb.writePtr) - 800*4 //look back 800 samples  -NOT in a BUFFER OF 1024 byytes!!

	// println("startbyte", startByte)

	// if startByte < 0 {
	// 	startByte += int(len(bb.data))
	// 	if startByte < 0 {
	// 		println("trying to graph more than the length of the buffer")
	// 		panic("")
	// 	}
	// }
	// bb.readPtr = uint32(startByte)

	println("bb rptr", bb.readPtr)

	b.WriteString("M0")
	b.WriteString(AutoRange(bb.ReadFloat32(), min, max, float32(600))) //first Y value

	for i := 0; i < 799; i++ {
		b.WriteString(" L")
		b.WriteString(strconv.Itoa(i)) //Xcoord
		b.WriteString(AutoRange(bb.ReadFloat32(), min, max, float32(600)))
	}
	return b.String()

}

func AutoRange(v float32, min float32, max float32, pixelsHigh float32) string {
	scaled := pixelsHigh - (v-min)/(max-min)*pixelsHigh
	return " " + strconv.Itoa(int(scaled))
}

// returns a slice (a reference to) the nextRead n bytes, and advances the read pointer
func (bb *BinBuffer) nextRead(numBytes uint32) []byte {

	var r []byte
	if bb.readPtr+numBytes > uint32(len(bb.data)) {
		//it's a 'split read'
		println("split read", bb.readPtr, numBytes, len(bb.data))
		overBy := bb.readPtr + numBytes - uint32(len(bb.data))
		r = append(bb.data[bb.readPtr:], bb.data[:overBy]...)
		bb.readPtr = overBy
	} else {
		r = bb.data[bb.readPtr : bb.readPtr+numBytes]
		bb.readPtr += numBytes

	}

	return r
}

func (bb *BinBuffer) nextWrite(numBytes uint32) []byte {
	bb.writePtr += numBytes //the ONLY place the write pointer is advanced

	if int(bb.writePtr) == len(bb.data) {
		if !bb.finite {
			bb.writePtr = 4 //because we return a slice up to the write pointer
		} //not pretty
	}
	if int(bb.writePtr) > len(bb.data) {
		if !bb.finite {
			println("write pointer went beyond the end of a finite (non ring) buffer", bb.writePtr, len(bb.data))
		} else {
			println("Write pointer out of bounds (didn't wrap nicley)", bb.writePtr, len(bb.data))
		}
		panic("The fat lady has sung")
	}

	return bb.data[bb.writePtr-numBytes : bb.writePtr]
}

// Serializes one buffer into another buffer (bb2 into bb)
func (bb *BinBuffer) WriteBB(bb2 *BinBuffer) {
	bb.WriteUint32(bb2.Length())
	bb.WriteUint32(bb2.readPtr)
	bb.WriteUint32(bb2.writePtr)
	bb.WriteBytes(bb2.data)
}

// restores(creates) a buffer from data within bb
func (bb *BinBuffer) ReadBB() *BinBuffer {

	length := bb.ReadUint32()
	if length > (1024 * 1024) {
		println("length exceeds 1MB", length)
		panic("")
	}

	rp := bb.ReadUint32()
	wp := bb.ReadUint32()

	if wp > (1024 * 1024) {
		println("wp exceeds 1MB", wp)
		panic("")
	}
	if rp > (1024 * 1024) {
		println("rp exceeds 1MB", rp)
		panic("")
	}

	return NewFromSlice(bb.ReadBytes(length), rp, wp, false)
}

func (bb *BinBuffer) ReadUint32() uint32 {
	return binary.LittleEndian.Uint32(bb.nextRead(4))
}

func (bb *BinBuffer) WriteUint32(v uint32) {
	//it's slightly odd that this would work, but next4 is a reference to a slice of the buffer
	binary.LittleEndian.PutUint32(bb.nextWrite(4), v)
}

func (bb *BinBuffer) ReadBytes(n uint32) []byte {
	return bb.nextRead(n)
}

func (bb *BinBuffer) WriteBytes(b []byte) {
	length := uint32(len(b))
	copy(bb.nextWrite(length), b)
}

func (bb *BinBuffer) ReadFloat32() float32 {
	println("readfloat32", bb.readPtr)
	i32 := binary.LittleEndian.Uint32(bb.nextRead(4))
	println("i32", i32)
	return math.Float32frombits(i32)
}

func (bb *BinBuffer) WriteFloat32(v float32) {
	bb.LastValue = v
	binary.LittleEndian.PutUint32(bb.nextWrite(4), math.Float32bits(v))
}
