package pbutil

import "log"

type Marshaler interface {
	Marshal() (data []byte, err error)
}

type Unmarshaler interface {
	Unmarshal(data []byte) error
}

func MustMarshal(m Marshaler) []byte {
	d, err := m.Marshal()
	if err != nil {
		log.Panicf("pbutil: %v", err)
	}
	return d
}

func MustUnmarshal(um Unmarshaler, data []byte) {
	if err := um.Unmarshal(data); err != nil {
		log.Panicf("pbutil: %v", err)
	}
}
