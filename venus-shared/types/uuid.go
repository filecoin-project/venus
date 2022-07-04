package types

import (
	"bytes"
	"database/sql/driver"
	"fmt"

	"github.com/google/uuid"
)

type UUID uuid.UUID

func NewUUID() UUID {
	return UUID(uuid.New())
}

func ParseUUID(uid string) (UUID, error) {
	id, err := uuid.Parse(uid)
	if err != nil {
		return UUID{}, nil
	}

	return UUID(id), nil
}

// Value implement sql.Scanner
func (uid UUID) IsEmpty() bool {
	return uid == UUID{}
}

// Value implement sql.Scanner
func (uid UUID) String() string {
	return uuid.UUID(uid).String()
}

// Value implement sql.Scanner
func (uid UUID) Value() (driver.Value, error) {
	return uuid.UUID(uid).String(), nil
}

// Scan assigns a value from a database driver.
// An error should be returned if the value cannot be stored
// without loss of information.
//
// Reference types such as []byte are only valid until the next call to Scan
// and should not be retained. Their underlying memory is owned by the driver.
// If retention is necessary, copy their values before the next call to Scan.
func (uid *UUID) Scan(value interface{}) error {
	var id uuid.UUID
	var err error
	switch value := value.(type) {
	case string:
		id, err = uuid.Parse(value)
	case []byte:
		id, err = uuid.ParseBytes(value)
	default:
		return fmt.Errorf("unsupport %t type for uuid", value)
	}
	if err != nil {
		return err
	}
	*uid = (UUID)(id)
	return nil
}

func (uid UUID) MarshalJSON() ([]byte, error) {
	return []byte("\"" + uid.String() + "\""), nil
}

func (uid *UUID) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, "\"")
	id, err := uuid.ParseBytes(b)
	if err != nil {
		return err
	}
	*uid = (UUID)(id)
	return nil
}

func (uid UUID) MarshalBinary() ([]byte, error) {
	return uuid.UUID(uid).MarshalBinary()
}

func (uid *UUID) UnmarshalBinary(b []byte) error {
	return (*uuid.UUID)(uid).UnmarshalBinary(b)
}
