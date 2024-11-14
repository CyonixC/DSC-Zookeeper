package proposals

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestPropLogWriteRead3(t *testing.T) {
	cont := []byte{0x45, 0xfe, 0xe6}
	prop1 := Proposal{
		PropType: StateChange,
		CountNum: 0,
		EpochNum: 3,
		Content:  cont,
	}
	prop2 := Proposal{
		PropType: StateChange,
		CountNum: 1,
		EpochNum: 3,
		Content:  cont,
	}
	prop3 := Proposal{
		PropType: StateChange,
		CountNum: 2,
		EpochNum: 3,
		Content:  cont,
	}
	if err := SaveProposal(prop1); err != nil {
		fmt.Println(err)
	}
	defer func() {
		propPath := filepath.Join(".", proposalsLog)
		if err := os.RemoveAll(propPath); err != nil {
			fmt.Println(err)
		}
	}()
	if err := SaveProposal(prop2); err != nil {
		fmt.Println(err)
	}
	if err := SaveProposal(prop3); err != nil {
		fmt.Println(err)
	}

	props, err := ReadProposals(3, 3)
	if err != nil {
		log.Fatal(err)
	}
	for i, p := range props {
		if p.PropType != StateChange {
			t.Errorf("Proposal %d read incorrectly: expected StateChange, got %s", i, p.PropType.ToStr())
		}
		if p.EpochNum != 3 {
			t.Errorf("Proposal %d read incorrectly: expected epoch 3, got %d", i, p.EpochNum)
		}
		if int(p.CountNum) != i {
			t.Errorf("Proposal %d read incorrectly: expected epoch 3, got %d", i, p.CountNum)
		}
		if !bytes.Equal(p.Content, cont) {
			t.Errorf("Proposal %d read incorrectly: expected [69 254 230], got %v", i, p.Content)
		}
	}
}

func TestPropLogOverRead(t *testing.T) {
	cont := []byte{0x45, 0xfe, 0xe6}
	prop1 := Proposal{
		PropType: StateChange,
		CountNum: 0,
		EpochNum: 3,
		Content:  cont,
	}
	prop2 := Proposal{
		PropType: StateChange,
		CountNum: 1,
		EpochNum: 3,
		Content:  cont,
	}
	prop3 := Proposal{
		PropType: StateChange,
		CountNum: 2,
		EpochNum: 3,
		Content:  cont,
	}
	if err := SaveProposal(prop1); err != nil {
		fmt.Println(err)
	}
	defer func() {
		propPath := filepath.Join(".", proposalsLog)
		if err := os.RemoveAll(propPath); err != nil {
			fmt.Println(err)
		}
	}()
	if err := SaveProposal(prop2); err != nil {
		fmt.Println(err)
	}
	if err := SaveProposal(prop3); err != nil {
		fmt.Println(err)
	}

	props, err := ReadProposals(3, 7)
	if err != nil {
		log.Fatal(err)
	}
	for i, p := range props {
		if p.PropType != StateChange {
			t.Errorf("Proposal %d read incorrectly: expected StateChange, got %s", i, p.PropType.ToStr())
		}
		if p.EpochNum != 3 {
			t.Errorf("Proposal %d read incorrectly: expected epoch 3, got %d", i, p.EpochNum)
		}
		if int(p.CountNum) != i {
			t.Errorf("Proposal %d read incorrectly: expected epoch 3, got %d", i, p.CountNum)
		}
		if !bytes.Equal(p.Content, cont) {
			t.Errorf("Proposal %d read incorrectly: expected [69 254 230], got %v", i, p.Content)
		}
	}
}

func TestMain(m *testing.M) {
	fmt.Println("Setting up proposal logging folders...")
	testingFolder := filepath.Join(".", "proposalsTests")
	os.Mkdir(testingFolder, 0777)
	os.Chdir(testingFolder)
	code := m.Run()
	fmt.Println("Removing proposal logging folders...")
	os.Chdir("..")
	if err := os.RemoveAll(testingFolder); err != nil {
		fmt.Println(err)
	}

	os.Exit(code)
}
