package proposals

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"unsafe"
)

const proposalsLog = "proposalsLog"

type fixedProposal struct {
	PropType ProposalType
	EpochNum uint16
	CountNum uint16
}

func SaveProposal(prop Proposal) error {
	// Save proposal will just save the proposal data in sequential order, just a naive approach
	// Proposals in the same epoch will be saved to the same append-only file. The delimiter between proposals will be the 0x00 0xFE 0x00 byte sequence.
	// Future optimisation TODO: just pass around an open file handle instead?

	propLogFolder := filepath.Join(".", proposalsLog)
	os.MkdirAll(propLogFolder, 0755)
	propLogPath := filepath.Join(propLogFolder, strconv.FormatUint(uint64(prop.EpochNum), 10))
	file, err := os.OpenFile(propLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	bytearr, err := proposalToBytes(prop)
	if err != nil {
		return err
	}
	_, err = file.Write(bytearr)
	if err != nil {
		return err
	}
	return nil
}

func ReadProposals(epoch uint16, readCount int) ([]Proposal, error) {
	// Read the latest `readCount` proposals from the proposal log.
	propLogPath := filepath.Join(".", proposalsLog, strconv.FormatUint(uint64(epoch), 10))
	file, err := os.OpenFile(propLogPath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Seek all the way to the end of the file
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	// Seek backwards from the end for `readCount` proposals, and read into memory.
	var props []Proposal
	lenTemp := make([]byte, unsafe.Sizeof(uint32(0)))
	for i := 0; i < readCount; i++ {
		// The length of each proposal is a uint32 at each end of it.
		curPos, err := file.Seek(-4, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("could not move cursor back to find length of proposal: %v", err)
		}
		_, err = file.ReadAt(lenTemp, curPos)
		if err != nil {
			return nil, fmt.Errorf("could not read length of proposal: %v", err)
		}
		length := int64(bytesToUint32(lenTemp))
		curPos, err = file.Seek(-length, io.SeekCurrent)
		propTemp := make([]byte, length)
		_, err = file.ReadAt(propTemp, curPos)
		if err != nil {
			return nil, fmt.Errorf("could not read proposal into memory: %v", err)
		}
		prop, err := bytesToProposal(propTemp)
		if err != nil {
			return nil, fmt.Errorf("could not convert bytes to proposal: %v", err)
		}

		props = append(props, prop)

		// We need to check if we've hit the start of the file.
		if curPos == 0 {
			break
		}

	}

	slices.Reverse(props)
	return props, nil
}

func stepBackAndReadOne(file *os.File) (byte, int, error) {
	tempOne := make([]byte, 1)
	currentPos, err := file.Seek(-1, io.SeekCurrent)
	if err != nil {
		return 0, -1, err
	}
	_, err = file.ReadAt(tempOne, currentPos)
	if err != nil {
		return 0, -1, err
	}
	return tempOne[0], int(currentPos), nil
}

func proposalToBytes(prop Proposal) ([]byte, error) {
	// Converts a proposal to byte form
	ret := new(bytes.Buffer)
	if err := writeProposal(ret, &prop); err != nil {
		return nil, fmt.Errorf("failed to convert proposal to bytes: %v", err)
	}
	return ret.Bytes(), nil
}

func writeProposal(w io.Writer, p *Proposal) error {
	f := fixedProposal{
		PropType: p.PropType,
		EpochNum: p.EpochNum,
		CountNum: p.CountNum,
	}

	totalLen := int(unsafe.Sizeof(ProposalType(0)))
	totalLen += int(unsafe.Sizeof(uint16(0)))
	totalLen += int(unsafe.Sizeof(uint16(0)))
	totalLen += len(p.Content)

	if err := binary.Write(w, binary.NativeEndian, f); err != nil {
		return fmt.Errorf("failed to write fixed proposal format: %v", err)
	}
	if _, err := w.Write(p.Content); err != nil {
		return fmt.Errorf("failed to write proposal content data: %v", err)
	}
	if _, err := w.Write(uint32ToBytes(uint32(totalLen))); err != nil {
		return fmt.Errorf("failed to write end length: %v", err)
	}
	return nil
}

func bytesToProposal(bArr []byte) (Proposal, error) {
	// Given a byte array, reconstruct the original struct.
	buf := bytes.NewReader(bArr)
	var p Proposal
	var f fixedProposal
	if err := binary.Read(buf, binary.NativeEndian, &f); err != nil {
		return p, fmt.Errorf("failed to read fixed-size proposal format: %v", err)
	}
	p.Content = make([]byte, buf.Len())
	if _, err := buf.Read(p.Content); err != nil {
		return p, fmt.Errorf("failed to read proposal content: %v", err)
	}
	p.PropType = f.PropType
	p.EpochNum = f.EpochNum
	p.CountNum = f.CountNum

	return p, nil
}
