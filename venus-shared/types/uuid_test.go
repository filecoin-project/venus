package types

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestUUID_Scan(t *testing.T) {
	uid := uuid.New()
	newID := UUID{}
	err := newID.Scan(uid.String())
	if err != nil {
		t.Error(err)
	}

	if newID.String() != uid.String() {
		t.Errorf("convert value failed")
	}
}

func TestUUID_Value(t *testing.T) {
	uid := uuid.New()
	newID := UUID(uid)

	val, err := newID.Value()
	if err != nil {
		t.Error(err)
	}
	if val.(string) != uid.String() {
		t.Errorf("convert value failed")
	}
}

func TestUUID_JsonMarshal(t *testing.T) {
	type T struct {
		ID UUID
	}

	val := T{ID: NewUUID()}

	marshallBytes, err := json.Marshal(&val)
	if err != nil {
		t.Error(err)
	}

	var val2 T
	err = json.Unmarshal(marshallBytes, &val2)
	if err != nil {
		t.Error(err)
	}

	if val2.ID != val.ID {
		t.Errorf("UUID json marshal fail")
	}
}

func TestUUID_MarshalBinary(t *testing.T) {
	uuid := NewUUID()

	marshallBytes, err := uuid.MarshalBinary()
	if err != nil {
		t.Error(err)
	}

	uuid2 := &UUID{}
	err = uuid2.UnmarshalBinary(marshallBytes)
	if err != nil {
		t.Error(err)
	}

	if uuid != *uuid2 {
		t.Errorf("UUID binary marshal fail")
	}
}
