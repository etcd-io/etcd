package uuid

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash"
	"log"
	mrand "math/rand"
	"net"
	"os"
	"sync"
	"time"
)

var (
	once       = new(sync.Once)
	generator  = newGenerator(nil)
)

func init() {
	seed := time.Now().UTC().UnixNano()
	b := [8]byte{}
	_, err := rand.Read(b[:])
	if err == nil {
		seed += int64(binary.BigEndian.Uint64(b[:]))
	}
	mrand.Seed(seed)

}

// Random provides a random number generator which reads into the given []byte, the package
// uses crypto/rand.Read by default. You can supply your own implementation or downgrade it
// to the match/rand package.
//
// The function is used by V4 UUIDs and for setting up V1 and V2 UUIDs in the
// Generator Init or Register* functions.
type Random func([]byte) (int, error)

// Next provides the next Timestamp value to be used by the next V1 or V2 UUID.
// The default uses the uuid.spinner which spins at a resolution of
// 100ns ticks and provides a spin resolution redundancy of 1024
// cycles. This ensures that the system is not too quick when
// generating V1 or V2 UUIDs. Each system requires a tuned Resolution to
// enhance performance.
type Next func() Timestamp

// Identifier provides the Node to be used during the life of a
// uuid.Generator. If it cannot be determined nil should be returned, the
// package will then provide a node identifier provided by the Random function.
// The default generator gets a MAC address from the first interface that is 'up' checking
// net.FlagUp.
type Identifier func() Node

// HandleRandomError provides the ability to manage a serious error that may be
// caused by accessing the standard crypto/rand library or the supplied uuid/Random
// function. Due to the rarity of this occurrence the error is swallowed by the
// uuid/NewV4 function which relies heavily on random numbers, the package will
// panic instead if an error occurs.
//
// You can change this behaviour by passing in your own uuid/HandleError
// function to a custom Generator. This function can attempt to fix the random
// number generator. If your uuid/HandleError returns true the generator will
// attempt to generate another V4 uuid. If another error occurs the function
// will return a fallback v4 uuid generated from the less random math/rand standard library.
//
// Waiting for system entropy may be all that is required in the initial error.
// If something more serious has occurred, handle appropriately using this function.
type HandleRandomError func([]byte, int, error) error

// Generator is used to create and monitor the running of V1 and V2, and V4
// UUIDs. It can be setup to take custom implementations for Timestamp, Node
// and Random number retrieval by providing those functions as required.
// You can also supply a uuid/Saver implementation for saving the state of the generator
// and you can also provide an error policy for V4 UUIDs and possible errors in the random
// number generator.
type Generator struct {
	// Access to the store needs to be maintained
	sync.Mutex

	// Once ensures that the generator is only setup and initialised once.
	// This will occur either when you explicitly call the
	// uuid.Generator.Init function or when a V1 or V2 id is generated.
	sync.Once

	// Store contains the current values being used by the Generator.
	*Store

	// Identifier as per the type Identifier func() Node
	Identifier

	// HandleRandomError as per the type HandleError func(error) bool
	HandleRandomError

	// Next as per the type Next func() Timestamp
	Next

	// Random as per the type Random func([]byte) (int, error)
	Random

	// Saver provides a non-volatile store to save the state of the
	// generator, the default is nil which will cause the timestamp
	// clock sequence to populate with random data. You can register your
	// own saver by using the uuid.RegisterSaver function or by creating
	// your own uuid.Generator instance.
	// UUIDs.
	Saver

	*log.Logger
}

// GeneratorConfig allows you to setup a new uuid.Generator using uuid.NewGenerator or RegisterGenerator. You can supply your own
// implementations for the random number generator Random, Identifier and Timestamp retrieval. You can also
// adjust the resolution of the default Timestamp spinner and supply your own
// error handler for crypto/rand failures.
type GeneratorConfig struct {
	Saver
	Next
	Resolution uint
	Identifier
	Random
	HandleRandomError
	*log.Logger
}

// NewGenerator will create a new uuid.Generator with the given functions.
func NewGenerator(config *GeneratorConfig) (*Generator, error) {
	return onceDo(newGenerator(config))
}

func onceDo(gen *Generator) (*Generator, error) {
	var err error
	gen.Do(func() {
		err = gen.init()
		if err != nil {
			gen = nil
		}
	})
	return gen, err
}

func newGenerator(config *GeneratorConfig) (gen *Generator) {
	if config == nil {
		config = new(GeneratorConfig)
	}
	gen = new(Generator)
	if config.Next == nil {
		if config.Resolution == 0 {
			config.Resolution = defaultSpinResolution
		}
		gen.Next = (&spinner{
			Resolution: config.Resolution,
			Count:      0,
			Timestamp:  Now(),
			now: Now,
		}).next
	} else {
		gen.Next = config.Next
	}
	if config.Identifier == nil {
		gen.Identifier = findFirstHardwareAddress
	} else {
		gen.Identifier = config.Identifier
	}
	if config.Random == nil {
		gen.Random = rand.Read
	} else {
		gen.Random = config.Random
	}
	if config.HandleRandomError == nil {
		gen.HandleRandomError = gen.runHandleError
	} else {
		gen.HandleRandomError = config.HandleRandomError
	}
	if config.Logger == nil {
		gen.Logger = log.New(os.Stderr, "uuid: ", log.LstdFlags)
	} else {
		gen.Logger = config.Logger
	}
	gen.Saver = config.Saver
	gen.Store = new(Store)
	return
}

// RegisterGenerator will set the package generator with the given configuration
// Like uuid.Init this can only be called once. Any subsequent calls will have no
// effect. If you call this you do not need to call uuid.Init.
func RegisterGenerator(config *GeneratorConfig) (err error) {
	notOnce := true
	once.Do(func() {
		generator, err = NewGenerator(config)
		notOnce = false
		return
	})
	if notOnce {
		panic("uuid: Register* methods cannot be called more than once.")
	}
	return
}

func (o *Generator) read() {

	// Save the state (current timestamp, clock sequence, and node ID)
	// back to the stable store
	if o.Saver != nil {
		defer o.save()
	}

	// Obtain a lock
	o.Lock()
	defer o.Unlock()

	// Get the current time as a 60-bit count of 100-nanosecond intervals
	// since 00:00:00.00, 15 October 1582.
	now := o.Next()

	// If the last timestamp is later than
	// the current timestamp, increment the clock sequence value.
	if now <= o.Timestamp {
		o.Sequence++
	}

	// Update the timestamp
	o.Timestamp = now
}

func (o *Generator) init() error {
	// From a system-wide shared stable store (e.g., a file), read the
	// UUID generator state: the values of the timestamp, clock sequence,
	// and node ID used to generate the last UUID.
	var (
		storage Store
		err     error
	)

	o.Lock()
	defer o.Unlock()

	if o.Saver != nil {
		storage, err = o.Read()
		if err != nil {
			o.Saver = nil
		}
	}

	// Get the current time as a 60-bit count of 100-nanosecond intervals
	// since 00:00:00.00, 15 October 1582.
	now := o.Next()

	//  Get the current node id
	node := o.Identifier()

	if node == nil {
		o.Println("address error generating random node id")

		node = make([]byte, 6)
		n, err := o.Random(node)
		if err != nil {
			o.Printf("could not read random bytes into node - read [%d] %s", n, err)
			return err
		}
		// Mark as randomly generated
		node[0] |= 0x01
	}

	// If the state was unavailable (e.g., non-existent or corrupted), or
	// the saved node ID is different than the current node ID, generate
	// a random clock sequence value.
	if o.Saver == nil || !bytes.Equal(storage.Node, node) {

		// 4.1.5.  Clock Sequence https://www.ietf.org/rfc/rfc4122.txt
		//
		// For UUID version 1, the clock sequence is used to help avoid
		// duplicates that could arise when the clock is set backwards in time
		// or if the node ID changes.
		//
		// If the clock is set backwards, or might have been set backwards
		// (e.g., while the system was powered off), and the UUID generator can
		// not be sure that no UUIDs were generated with timestamps larger than
		// the value to which the clock was set, then the clock sequence has to
		// be changed.  If the previous value of the clock sequence is known, it
		// can just be incremented; otherwise it should be set to a random or
		// high-quality pseudo-random value.

		// The clock sequence MUST be originally (i.e., once in the lifetime of
		// a system) initialized to a random number to minimize the correlation
		// across systems.  This provides maximum protection against node
		// identifiers that may move or switch from system to system rapidly.
		// The initial value MUST NOT be correlated to the node identifier.
		b := make([]byte, 2)
		n, err := o.Random(b)
		if err == nil {
			storage.Sequence = Sequence(binary.BigEndian.Uint16(b))
			o.Printf("initialised random sequence [%d]", storage.Sequence)

		} else {
			o.Printf("could not read random bytes into sequence - read [%d] %s", n, err)
			return err
		}
	} else if now < storage.Timestamp {
		// If the state was available, but the saved timestamp is later than
		// the current timestamp, increment the clock sequence value.
		storage.Sequence++
	}

	storage.Timestamp = now
	storage.Node = node

	o.Store = &storage

	return nil
}

func (o *Generator) save() {
	func(state *Generator) {
		if state.Saver != nil {
			state.Lock()
			defer state.Unlock()
			state.Save(*state.Store)
		}
	}(o)
}

// NewV1 generates a new RFC4122 version 1 UUID based on a 60 bit timestamp and
// node id.
func (o *Generator) NewV1() UUID {
	o.read()
	id := UUID{}

	makeUuid(&id,
		uint32(o.Timestamp),
		uint16(o.Timestamp>>32),
		uint16(o.Timestamp>>48),
		uint16(o.Sequence),
		o.Node)

	id.setRFC4122Version(VersionOne)
	return id
}

// ReadV1 will read a slice of UUIDs. Be careful with the set amount.
func (o *Generator) ReadV1(ids []UUID) {
	for i := range ids {
		ids[i] = o.NewV1()
	}
}

// BulkV1 will return a slice of V1 UUIDs. Be careful with the set amount.
func (o *Generator) BulkV1(amount int) []UUID {
	ids := make([]UUID, amount)
	o.ReadV1(ids)
	return ids
}

// NewV2 generates a new DCE version 2 UUID based on a 60 bit timestamp, node id
// and the id of the given Id type.
func (o *Generator) NewV2(idType SystemId) UUID {
	o.read()

	id := UUID{}

	var osId uint32

	switch idType {
	case SystemIdUser:
		osId = uint32(os.Getuid())
	case SystemIdGroup:
		osId = uint32(os.Getgid())
	case SystemIdEffectiveUser:
		osId = uint32(os.Geteuid())
	case SystemIdEffectiveGroup:
		osId = uint32(os.Getegid())
	case SystemIdCallerProcess:
		osId = uint32(os.Getpid())
	case SystemIdCallerProcessParent:
		osId = uint32(os.Getppid())
	}

	makeUuid(&id,
		osId,
		uint16(o.Timestamp>>32),
		uint16(o.Timestamp>>48),
		uint16(o.Sequence),
		o.Node)

	id[9] = byte(idType)
	id.setRFC4122Version(VersionTwo)
	return id
}

// NewV3 generates a new RFC4122 version 3 UUID based on the MD5 hash of a
// namespace UUID namespace Implementation UUID and one or more unique names.
func (o *Generator) NewV3(namespace Implementation, names ...interface{}) UUID {
	id := UUID{}
	id.unmarshal(digest(md5.New(), namespace.Bytes(), names...))
	id.setRFC4122Version(VersionThree)
	return id
}

func digest(hash hash.Hash, name []byte, names ...interface{}) []byte {
	for _, v := range names {
		switch t := v.(type) {
		case string:
			name = append(name, t...)
			continue
		case []byte:
			name = append(name, t...)
			continue
		case *string:
			name = append(name, (*t)...)
			continue
		}
		if s, ok := v.(fmt.Stringer); ok {
			name = append(name, s.String()...)
			continue
		}
		panic(fmt.Sprintf("uuid: does not support type [%T] as a name for hashed UUIDs.", v))
	}
	hash.Write(name)
	return hash.Sum(nil)
}

// NewV4 generates a cryptographically secure random RFC4122 version 4 UUID. If there is an error with the random
// number generator this will
func (o *Generator) NewV4() (id UUID) {
	o.v4(&id)
	return
}

// ReadV4 will read into a slice of UUIDs. Be careful with the set amount.
// Note: V4 UUIDs require sufficient entropy from the generator.
// If n == len(ids) err will be nil.
func (o *Generator) ReadV4(ids []UUID) {
	for i := range ids {
		id := UUID{}
		o.v4(&id)
		ids[i] = id
		continue
	}
	return
}

// BulkV4 will return a slice of V4 UUIDs. Be careful with the set amount.
// Note: V4 UUIDs require sufficient entropy from the generator.
// If n == len(ids) err will be nil.
func (o *Generator) BulkV4(amount int) []UUID {
	ids := make([]UUID, amount)
	o.ReadV4(ids)
	return ids
}

func (o *Generator) v4(id *UUID) {
	n, err := o.Random(id[:])
	if err != nil {
		o.Printf("there was an error getting random bytes [%s]", err)
		if err = o.HandleRandomError(id[:], n, err); err != nil {
			panic(fmt.Sprintf("random number error - %s", err))
		}
	}
	id.setRFC4122Version(VersionFour)
}


// NewV5 generates an RFC4122 version 5 UUID based on the SHA-1 hash of a
// namespace Implementation UUID and one or more unique names.
func (o *Generator) NewV5(namespace Implementation, names ...interface{}) UUID {
	id := UUID{}
	id.unmarshal(digest(sha1.New(), namespace.Bytes(), names...))
	id.setRFC4122Version(VersionFive)
	return id
}

// NewHash generate a UUID based on the given hash implementation. The hash will
// be of the given names. The version will be set to 0 for Unknown and the
// variant will be set to VariantFuture.
func (o *Generator) NewHash(hash hash.Hash, names ...interface{}) UUID {
	id := UUID{}
	id.unmarshal(digest(hash, []byte{}, names...))
	id[versionIndex] &= 0x0f
	id[versionIndex] |= uint8(0 << 4)
	id[variantIndex] &= variantSet
	id[variantIndex] |= VariantFuture
	return id
}

func makeUuid(id *UUID, low uint32, mid, hiAndV, seq uint16, node Node) {

	id[0] = byte(low >> 24)
	id[1] = byte(low >> 16)
	id[2] = byte(low >> 8)
	id[3] = byte(low)

	id[4] = byte(mid >> 8)
	id[5] = byte(mid)

	id[6] = byte(hiAndV >> 8)
	id[7] = byte(hiAndV)

	id[8] = byte(seq >> 8)
	id[9] = byte(seq)

	copy(id[10:], node)
}

func findFirstHardwareAddress() (node Node) {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, i := range interfaces {
			if i.Flags&net.FlagUp != 0 && bytes.Compare(i.HardwareAddr, nil) != 0 {
				// Don't use random as we have a real address
				node = Node(i.HardwareAddr)
				break
			}
		}
	}
	return
}

func (o *Generator) runHandleError(id []byte, n int, err error) error {
	o.Lock()
	mrand.Read(id)
	o.Unlock()
	return nil
}
