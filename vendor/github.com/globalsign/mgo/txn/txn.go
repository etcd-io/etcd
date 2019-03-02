// Package txn implements support for multi-document transactions.
//
// For details check the following blog post:
//
//     http://blog.labix.org/2012/08/22/multi-doc-transactions-for-mongodb
//
package txn

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	mgo "github.com/globalsign/mgo"

	"github.com/globalsign/mgo/bson"

	crand "crypto/rand"
	mrand "math/rand"
)

type state int

const (
	tpreparing state = 1 // One or more documents not prepared
	tprepared  state = 2 // Prepared but not yet ready to run
	taborting  state = 3 // Assertions failed, cleaning up
	tapplying  state = 4 // Changes are in progress
	taborted   state = 5 // Pre-conditions failed, nothing done
	tapplied   state = 6 // All changes applied
)

func (s state) String() string {
	switch s {
	case tpreparing:
		return "preparing"
	case tprepared:
		return "prepared"
	case taborting:
		return "aborting"
	case tapplying:
		return "applying"
	case taborted:
		return "aborted"
	case tapplied:
		return "applied"
	}
	panic(fmt.Errorf("unknown state: %d", s))
}

var rand *mrand.Rand
var randmu sync.Mutex

func init() {
	var seed int64
	err := binary.Read(crand.Reader, binary.BigEndian, &seed)
	if err != nil {
		panic(err)
	}
	rand = mrand.New(mrand.NewSource(seed))
}

type transaction struct {
	Id     bson.ObjectId `bson:"_id"`
	State  state         `bson:"s"`
	Info   interface{}   `bson:"i,omitempty"`
	Ops    []Op          `bson:"o"`
	Nonce  string        `bson:"n,omitempty"`
	Revnos []int64       `bson:"r,omitempty"`

	docKeysCached docKeys
}

func (t *transaction) String() string {
	if t.Nonce == "" {
		return t.Id.Hex()
	}
	return string(t.token())
}

func (t *transaction) done() bool {
	return t.State == tapplied || t.State == taborted
}

func (t *transaction) token() token {
	if t.Nonce == "" {
		panic("transaction has no nonce")
	}
	return tokenFor(t)
}

func (t *transaction) docKeys() docKeys {
	if t.docKeysCached != nil {
		return t.docKeysCached
	}
	dkeys := make(docKeys, 0, len(t.Ops))
NextOp:
	for _, op := range t.Ops {
		dkey := op.docKey()
		for i := range dkeys {
			if dkey == dkeys[i] {
				continue NextOp
			}
		}
		dkeys = append(dkeys, dkey)
	}
	sort.Sort(dkeys)
	t.docKeysCached = dkeys
	return dkeys
}

// tokenFor returns a unique transaction token that
// is composed by t's Id and a nonce. If t already has
// a nonce assigned to it, it will be used, otherwise
// a new nonce will be generated.
func tokenFor(t *transaction) token {
	nonce := t.Nonce
	if nonce == "" {
		nonce = newNonce()
	}
	return token(t.Id.Hex() + "_" + nonce)
}

func newNonce() string {
	randmu.Lock()
	r := rand.Uint32()
	randmu.Unlock()
	n := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		n[i] = "0123456789abcdef"[(r>>(4*i))&0xf]
	}
	return string(n)
}

type token string

func (tt token) id() bson.ObjectId { return bson.ObjectIdHex(string(tt[:24])) }
func (tt token) nonce() string     { return string(tt[25:]) }

// Op represents an operation to a single document that may be
// applied as part of a transaction with other operations.
type Op struct {
	// C and Id identify the collection and document this operation
	// refers to. Id is matched against the "_id" document field.
	C  string      `bson:"c"`
	Id interface{} `bson:"d"`

	// Assert optionally holds a query document that is used to
	// test the operation document at the time the transaction is
	// going to be applied. The assertions for all operations in
	// a transaction are tested before any changes take place,
	// and the transaction is entirely aborted if any of them
	// fails. This is also the only way to prevent a transaction
	// from being being applied (the transaction continues despite
	// the outcome of Insert, Update, and Remove).
	Assert interface{} `bson:"a,omitempty"`

	// The Insert, Update and Remove fields describe the mutation
	// intended by the operation. At most one of them may be set
	// per operation. If none are set, Assert must be set and the
	// operation becomes a read-only test.
	//
	// Insert holds the document to be inserted at the time the
	// transaction is applied. The Id field will be inserted
	// into the document automatically as its _id field. The
	// transaction will continue even if the document already
	// exists. Use Assert with txn.DocMissing if the insertion is
	// required.
	//
	// Update holds the update document to be applied at the time
	// the transaction is applied. The transaction will continue
	// even if a document with Id is missing. Use Assert to
	// test for the document presence or its contents.
	//
	// Remove indicates whether to remove the document with Id.
	// The transaction continues even if the document doesn't yet
	// exist at the time the transaction is applied. Use Assert
	// with txn.DocExists to make sure it will be removed.
	Insert interface{} `bson:"i,omitempty"`
	Update interface{} `bson:"u,omitempty"`
	Remove bool        `bson:"r,omitempty"`
}

func (op *Op) isChange() bool {
	return op.Update != nil || op.Insert != nil || op.Remove
}

func (op *Op) docKey() docKey {
	return docKey{op.C, op.Id}
}

func (op *Op) name() string {
	switch {
	case op.Update != nil:
		return "update"
	case op.Insert != nil:
		return "insert"
	case op.Remove:
		return "remove"
	case op.Assert != nil:
		return "assert"
	}
	return "none"
}

const (
	// DocExists may be used on an operation's
	// Assert value to assert that the document with the given
	// ID exists.
	DocExists = "d+"
	// DocMissing may be used on an operation's
	// Assert value to assert that the document with the given
	// ID does not exist.
	DocMissing = "d-"
)

// A Runner applies operations as part of a transaction onto any number
// of collections within a database. See the Run method for details.
type Runner struct {
	tc   *mgo.Collection // txns
	sc   *mgo.Collection // stash
	lc   *mgo.Collection // log
	opts RunnerOptions   // runtime options
}

const defaultMaxTxnQueueLength = 1000

// NewRunner returns a new transaction runner that uses tc to hold its
// transactions.
//
// Multiple transaction collections may exist in a single database, but
// all collections that are touched by operations in a given transaction
// collection must be handled exclusively by it.
//
// A second collection with the same name of tc but suffixed by ".stash"
// will be used for implementing the transactional behavior of insert
// and remove operations.
func NewRunner(tc *mgo.Collection) *Runner {
	return &Runner{
		tc:   tc,
		sc:   tc.Database.C(tc.Name + ".stash"),
		lc:   nil,
		opts: DefaultRunnerOptions(),
	}
}

// RunnerOptions encapsulates ways you can tweak transaction Runner behavior.
type RunnerOptions struct {
	// MaxTxnQueueLength is a way to limit bad behavior. Many operations on
	// transaction queues are O(N^2), and transaction queues growing too large
	// are usually indicative of a bug in behavior. This should be larger
	// than the maximum number of concurrent operations to a single document.
	// Normal operations are likely to only ever hit 10 or so, we use a default
	// maximum length of 1000.
	MaxTxnQueueLength int
}

// SetOptions allows people to change some of the internal behavior of a Runner.
func (r *Runner) SetOptions(opts RunnerOptions) {
	r.opts = opts
}

// DefaultRunnerOptions defines default behavior for a Runner.
// Users can use the DefaultRunnerOptions to only override specific behavior.
func DefaultRunnerOptions() RunnerOptions {
	return RunnerOptions{
		MaxTxnQueueLength: defaultMaxTxnQueueLength,
	}
}

// ErrAborted error returned if one or more operations
// can't be applied.
var ErrAborted = fmt.Errorf("transaction aborted")

// Run creates a new transaction with ops and runs it immediately.
// The id parameter specifies the transaction Id, and may be written
// down ahead of time to later verify the success of the change and
// resume it, when the procedure is interrupted for any reason. If
// empty, a random Id will be generated.
// The info parameter, if not nil, is included under the "i"
// field of the transaction document.
//
// Operations across documents are not atomically applied, but are
// guaranteed to be eventually all applied in the order provided or
// all aborted, as long as the affected documents are only modified
// through transactions. If documents are simultaneously modified
// by transactions and out of transactions the behavior is undefined.
//
// If Run returns no errors, all operations were applied successfully.
// If it returns ErrAborted, one or more operations can't be applied
// and the transaction was entirely aborted with no changes performed.
// Otherwise, if the transaction is interrupted while running for any
// reason, it may be resumed explicitly or by attempting to apply
// another transaction on any of the documents targeted by ops, as
// long as the interruption was made after the transaction document
// itself was inserted. Run Resume with the obtained transaction Id
// to confirm whether the transaction was applied or not.
//
// Any number of transactions may be run concurrently, with one
// runner or many.
func (r *Runner) Run(ops []Op, id bson.ObjectId, info interface{}) (err error) {
	const efmt = "error in transaction op %d: %s"
	for i := range ops {
		op := &ops[i]
		if op.C == "" || op.Id == nil {
			return fmt.Errorf(efmt, i, "C or Id missing")
		}
		changes := 0
		if op.Insert != nil {
			changes++
		}
		if op.Update != nil {
			changes++
		}
		if op.Remove {
			changes++
		}
		if changes > 1 {
			return fmt.Errorf(efmt, i, "more than one of Insert/Update/Remove set")
		}
		if changes == 0 && op.Assert == nil {
			return fmt.Errorf(efmt, i, "none of Assert/Insert/Update/Remove set")
		}
	}
	if id == "" {
		id = bson.NewObjectId()
	}

	// Insert transaction sooner rather than later, to stay on the safer side.
	t := transaction{
		Id:    id,
		Ops:   ops,
		State: tpreparing,
		Info:  info,
	}
	if err = r.tc.Insert(&t); err != nil {
		return err
	}
	if err = flush(r, &t); err != nil {
		return err
	}
	if t.State == taborted {
		return ErrAborted
	} else if t.State != tapplied {
		panic(fmt.Errorf("invalid state for %s after flush: %q", &t, t.State))
	}
	return nil
}

// ResumeAll resumes all pending transactions. All ErrAborted errors
// from individual transactions are ignored.
func (r *Runner) ResumeAll() (err error) {
	debugf("Resuming all unfinished transactions")
	iter := r.tc.Find(bson.D{{Name: "s", Value: bson.D{{Name: "$in", Value: []state{tpreparing, tprepared, tapplying}}}}}).Iter()
	var t transaction
	for iter.Next(&t) {
		if t.State == tapplied || t.State == taborted {
			continue
		}
		debugf("Resuming %s from %q", t.Id, t.State)
		if err := flush(r, &t); err != nil {
			return err
		}
		if !t.done() {
			panic(fmt.Errorf("invalid state for %s after flush: %q", &t, t.State))
		}
	}
	return nil
}

// Resume resumes the transaction with Id. It returns mgo.ErrNotFound
// if the transaction is not found. Otherwise, it has the same semantics
// of the Run method after the transaction is inserted.
func (r *Runner) Resume(id bson.ObjectId) (err error) {
	t, err := r.load(id)
	if err != nil {
		return err
	}
	if !t.done() {
		debugf("Resuming %s from %q", t, t.State)
		if err := flush(r, t); err != nil {
			return err
		}
	}
	if t.State == taborted {
		return ErrAborted
	} else if t.State != tapplied {
		panic(fmt.Errorf("invalid state for %s after flush: %q", t, t.State))
	}
	return nil
}

// ChangeLog enables logging of changes to the given collection
// every time a transaction that modifies content is done being
// applied.
//
// Saved documents are in the format:
//
//     {"_id": <txn id>, <collection>: {"d": [<doc id>, ...], "r": [<doc revno>, ...]}}
//
// The document revision is the value of the txn-revno field after
// the change has been applied. Negative values indicate the document
// was not present in the collection. Revisions will not change when
// updates or removes are applied to missing documents or inserts are
// attempted when the document isn't present.
func (r *Runner) ChangeLog(logc *mgo.Collection) {
	r.lc = logc
}

// PurgeMissing removes from collections any state that refers to transaction
// documents that for whatever reason have been lost from the system (removed
// by accident or lost in a hard crash, for example).
//
// This method should very rarely be needed, if at all, and should never be
// used during the normal operation of an application. Its purpose is to put
// a system that has seen unavoidable corruption back in a working state.
func (r *Runner) PurgeMissing(collections ...string) error {
	type M map[string]interface{}
	type S []interface{}

	type TDoc struct {
		Id       interface{} `bson:"_id"`
		TxnQueue []string    `bson:"txn-queue"`
	}

	found := make(map[bson.ObjectId]bool)

	sort.Strings(collections)
	for _, collection := range collections {
		c := r.tc.Database.C(collection)
		iter := c.Find(nil).Select(bson.M{"_id": 1, "txn-queue": 1}).Iter()
		var tdoc TDoc
		for iter.Next(&tdoc) {
			for _, txnToken := range tdoc.TxnQueue {
				txnId := bson.ObjectIdHex(txnToken[:24])
				if found[txnId] {
					continue
				}
				if r.tc.FindId(txnId).One(nil) == nil {
					found[txnId] = true
					continue
				}
				logf("WARNING: purging from document %s/%v the missing transaction id %s", collection, tdoc.Id, txnId)
				err := c.UpdateId(tdoc.Id, M{"$pull": M{"txn-queue": M{"$regex": "^" + txnId.Hex() + "_*"}}})
				if err != nil {
					return fmt.Errorf("error purging missing transaction %s: %v", txnId.Hex(), err)
				}
			}
		}
		if err := iter.Close(); err != nil {
			return fmt.Errorf("transaction queue iteration error for %s: %v", collection, err)
		}
	}

	type StashTDoc struct {
		Id       docKey   `bson:"_id"`
		TxnQueue []string `bson:"txn-queue"`
	}

	iter := r.sc.Find(nil).Select(bson.M{"_id": 1, "txn-queue": 1}).Iter()
	var stdoc StashTDoc
	for iter.Next(&stdoc) {
		for _, txnToken := range stdoc.TxnQueue {
			txnId := bson.ObjectIdHex(txnToken[:24])
			if found[txnId] {
				continue
			}
			if r.tc.FindId(txnId).One(nil) == nil {
				found[txnId] = true
				continue
			}
			logf("WARNING: purging from stash document %s/%v the missing transaction id %s", stdoc.Id.C, stdoc.Id.Id, txnId)
			err := r.sc.UpdateId(stdoc.Id, M{"$pull": M{"txn-queue": M{"$regex": "^" + txnId.Hex() + "_*"}}})
			if err != nil {
				return fmt.Errorf("error purging missing transaction %s: %v", txnId.Hex(), err)
			}
		}
	}
	if err := iter.Close(); err != nil {
		return fmt.Errorf("transaction stash iteration error: %v", err)
	}

	return nil
}

func (r *Runner) load(id bson.ObjectId) (*transaction, error) {
	var t transaction
	err := r.tc.FindId(id).One(&t)
	if err == mgo.ErrNotFound {
		return nil, fmt.Errorf("cannot find transaction %s", id)
	} else if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Runner) loadMulti(ids []bson.ObjectId, preloaded map[bson.ObjectId]*transaction) error {
	txns := make([]transaction, 0, len(ids))

	query := r.tc.Find(bson.M{"_id": bson.M{"$in": ids}})
	// Not sure that this actually has much of an effect when using All()
	query.Batch(len(ids))
	err := query.All(&txns)
	if err == mgo.ErrNotFound {
		return fmt.Errorf("could not find a transaction in batch: %v", ids)
	} else if err != nil {
		return err
	}
	for i := range txns {
		t := &txns[i]
		preloaded[t.Id] = t
	}
	return nil
}

type typeNature int

const (
	// The order of these values matters. Transactions
	// from applications using different ordering will
	// be incompatible with each other.
	_ typeNature = iota
	natureString
	natureInt
	natureFloat
	natureBool
	natureStruct
)

func valueNature(v interface{}) (value interface{}, nature typeNature) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return rv.String(), natureString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), natureInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(rv.Uint()), natureInt
	case reflect.Float32, reflect.Float64:
		return rv.Float(), natureFloat
	case reflect.Bool:
		return rv.Bool(), natureBool
	case reflect.Struct:
		return v, natureStruct
	}
	panic("document id type unsupported by txn: " + rv.Kind().String())
}

type docKey struct {
	C  string
	Id interface{}
}

type docKeys []docKey

func (ks docKeys) Len() int      { return len(ks) }
func (ks docKeys) Swap(i, j int) { ks[i], ks[j] = ks[j], ks[i] }
func (ks docKeys) Less(i, j int) bool {
	a, b := ks[i], ks[j]
	if a.C != b.C {
		return a.C < b.C
	}
	return valuecmp(a.Id, b.Id) == -1
}

func valuecmp(a, b interface{}) int {
	av, an := valueNature(a)
	bv, bn := valueNature(b)
	if an < bn {
		return -1
	}
	if an > bn {
		return 1
	}

	if av == bv {
		return 0
	}
	var less bool
	switch an {
	case natureString:
		less = av.(string) < bv.(string)
	case natureInt:
		less = av.(int64) < bv.(int64)
	case natureFloat:
		less = av.(float64) < bv.(float64)
	case natureBool:
		less = !av.(bool) && bv.(bool)
	case natureStruct:
		less = structcmp(av, bv) == -1
	default:
		panic("unreachable")
	}
	if less {
		return -1
	}
	return 1
}

func structcmp(a, b interface{}) int {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	var ai, bi = 0, 0
	var an, bn = av.NumField(), bv.NumField()
	var avi, bvi interface{}
	var af, bf reflect.StructField
	for {
		for ai < an {
			af = av.Type().Field(ai)
			if isExported(af.Name) {
				avi = av.Field(ai).Interface()
				ai++
				break
			}
			ai++
		}
		for bi < bn {
			bf = bv.Type().Field(bi)
			if isExported(bf.Name) {
				bvi = bv.Field(bi).Interface()
				bi++
				break
			}
			bi++
		}
		if n := valuecmp(avi, bvi); n != 0 {
			return n
		}
		nameA := getFieldName(af)
		nameB := getFieldName(bf)
		if nameA < nameB {
			return -1
		}
		if nameA > nameB {
			return 1
		}
		if ai == an && bi == bn {
			return 0
		}
		if ai == an || bi == bn {
			if ai == bn {
				return -1
			}
			return 1
		}
	}
}

func isExported(name string) bool {
	a := name[0]
	return a >= 'A' && a <= 'Z'
}

func getFieldName(f reflect.StructField) string {
	name := f.Tag.Get("bson")
	if i := strings.Index(name, ","); i >= 0 {
		name = name[:i]
	}
	if name == "" {
		name = strings.ToLower(f.Name)
	}
	return name
}
