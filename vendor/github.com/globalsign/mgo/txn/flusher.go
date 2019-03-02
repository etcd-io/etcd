package txn

import (
	"fmt"

	mgo "github.com/globalsign/mgo"

	"github.com/globalsign/mgo/bson"
)

func flush(r *Runner, t *transaction) error {
	f := &flusher{
		Runner:   r,
		goal:     t,
		goalKeys: make(map[docKey]bool),
		queue:    make(map[docKey][]tokenAndId),
		debugId:  debugPrefix(),
	}
	for _, dkey := range f.goal.docKeys() {
		f.goalKeys[dkey] = true
	}
	return f.run()
}

type flusher struct {
	*Runner
	goal     *transaction
	goalKeys map[docKey]bool
	queue    map[docKey][]tokenAndId
	debugId  string
}

type tokenAndId struct {
	tt  token
	bid bson.ObjectId
}

func (ti tokenAndId) Id() bson.ObjectId {
	return ti.bid
}

func (ti tokenAndId) nonce() string {
	return ti.tt.nonce()
}

func (ti tokenAndId) String() string {
	return string(ti.tt)
}

func tokensWithIds(q []token) []tokenAndId {
	out := make([]tokenAndId, len(q))
	for i, tt := range q {
		out[i].tt = tt
		out[i].bid = tt.id()
	}
	return out
}

func (f *flusher) run() (err error) {
	if chaosEnabled {
		defer f.handleChaos(&err)
	}

	f.debugf("Processing %s", f.goal)
	seen := make(map[bson.ObjectId]*transaction)
	preloaded := make(map[bson.ObjectId]*transaction)
	if err := f.recurse(f.goal, seen, preloaded); err != nil {
		return err
	}
	if f.goal.done() {
		return nil
	}

	// Sparse workloads will generally be managed entirely by recurse.
	// Getting here means one or more transactions have dependencies
	// and perhaps cycles.

	// Build successors data for Tarjan's sort. Must consider
	// that entries in txn-queue are not necessarily valid.
	successors := make(map[bson.ObjectId][]bson.ObjectId)
	ready := true
	for _, dqueue := range f.queue {
	NextPair:
		for i := 0; i < len(dqueue); i++ {
			pred := dqueue[i]
			predid := pred.Id()
			predt := seen[predid]
			if predt == nil || predt.Nonce != pred.nonce() {
				continue
			}
			predsuccids, ok := successors[predid]
			if !ok {
				successors[predid] = nil
			}

			for j := i + 1; j < len(dqueue); j++ {
				succ := dqueue[j]
				succid := succ.Id()
				succt := seen[succid]
				if succt == nil || succt.Nonce != succ.nonce() {
					continue
				}
				if _, ok := successors[succid]; !ok {
					successors[succid] = nil
				}

				// Found a valid pred/succ pair.
				i = j - 1
				for _, predsuccid := range predsuccids {
					if predsuccid == succid {
						continue NextPair
					}
				}
				successors[predid] = append(predsuccids, succid)
				if succid == f.goal.Id {
					// There are still pre-requisites to handle.
					ready = false
				}
				continue NextPair
			}
		}
	}
	f.debugf("Queues: %v", f.queue)
	f.debugf("Successors: %v", successors)
	if ready {
		f.debugf("Goal %s has no real pre-requisites", f.goal)
		return f.advance(f.goal, nil, true)
	}

	// Robert Tarjan's algorithm for detecting strongly-connected
	// components is used for topological sorting and detecting
	// cycles at once. The order in which transactions are applied
	// in commonly affected documents must be a global agreement.
	sorted := tarjanSort(successors)
	if debugEnabled {
		f.debugf("Tarjan output: %v", sorted)
	}
	pull := make(map[bson.ObjectId]*transaction)
	for i := len(sorted) - 1; i >= 0; i-- {
		scc := sorted[i]
		f.debugf("Flushing %v", scc)
		if len(scc) == 1 {
			pull[scc[0]] = seen[scc[0]]
		}
		for _, Id := range scc {
			if err := f.advance(seen[Id], pull, true); err != nil {
				return err
			}
		}
		if len(scc) > 1 {
			for _, Id := range scc {
				pull[Id] = seen[Id]
			}
		}
	}
	return nil
}

const preloadBatchSize = 100

func (f *flusher) recurse(t *transaction, seen map[bson.ObjectId]*transaction, preloaded map[bson.ObjectId]*transaction) error {
	seen[t.Id] = t
	delete(preloaded, t.Id)
	err := f.advance(t, nil, false)
	if err != errPreReqs {
		return err
	}
	for _, dkey := range t.docKeys() {
		remaining := make([]bson.ObjectId, 0, len(f.queue[dkey]))
		toPreload := make(map[bson.ObjectId]struct{}, len(f.queue[dkey]))
		for _, dtt := range f.queue[dkey] {
			Id := dtt.Id()
			if _, scheduled := toPreload[Id]; seen[Id] != nil || scheduled || preloaded[Id] != nil {
				continue
			}
			toPreload[Id] = struct{}{}
			remaining = append(remaining, Id)
		}
		for len(remaining) > 0 {
			batch := remaining
			if len(batch) > preloadBatchSize {
				batch = remaining[:preloadBatchSize]
			}
			remaining = remaining[len(batch):]
			err := f.loadMulti(batch, preloaded)
			if err != nil {
				return err
			}
			for _, Id := range batch {
				if seen[Id] != nil {
					continue
				}
				qt, ok := preloaded[Id]
				if !ok {
					qt, err = f.load(Id)
					if err != nil {
						return err
					}
				}
				err = f.recurse(qt, seen, preloaded)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *flusher) advance(t *transaction, pull map[bson.ObjectId]*transaction, force bool) error {
	for {
		switch t.State {
		case tpreparing, tprepared:
			revnos, err := f.prepare(t, force)
			if err != nil {
				return err
			}
			if t.State != tprepared {
				continue
			}
			if err = f.assert(t, revnos, pull); err != nil {
				return err
			}
			if t.State != tprepared {
				continue
			}
			if err = f.checkpoint(t, revnos); err != nil {
				return err
			}
		case tapplying:
			return f.apply(t, pull)
		case taborting:
			return f.abortOrReload(t, nil, pull)
		case tapplied, taborted:
			return nil
		default:
			panic(fmt.Errorf("transaction in unknown state: %q", t.State))
		}
	}
}

type stash string

const (
	stashStable stash = ""
	stashInsert stash = "insert"
	stashRemove stash = "remove"
)

type txnInfo struct {
	Queue  []token       `bson:"txn-queue"`
	Revno  int64         `bson:"txn-revno,omitempty"`
	Insert bson.ObjectId `bson:"txn-insert,omitempty"`
	Remove bson.ObjectId `bson:"txn-remove,omitempty"`
}

type stashState string

const (
	stashNew       stashState = ""
	stashInserting stashState = "inserting"
)

var txnFields = bson.D{
	{Name: "txn-queue", Value: 1},
	{Name: "txn-revno", Value: 1},
	{Name: "txn-remove", Value: 1},
	{Name: "txn-insert", Value: 1}}

var errPreReqs = fmt.Errorf("transaction has pre-requisites and force is false")

// prepare injects t's Id onto txn-queue for all affected documents
// and collects the current txn-queue and txn-revno values during
// the process. If the prepared txn-queue indicates that there are
// pre-requisite transactions to be applied and the force parameter
// is false, errPreReqs will be returned. Otherwise, the current
// tip revision numbers for all the documents are returned.
func (f *flusher) prepare(t *transaction, force bool) (revnos []int64, err error) {
	if t.State != tpreparing {
		return f.rescan(t, force)
	}
	f.debugf("Preparing %s", t)

	// dkeys being sorted means stable iteration across all runners. This
	// isn't strictly required, but reduces the chances of cycles.
	dkeys := t.docKeys()

	revno := make(map[docKey]int64)
	info := txnInfo{}
	tt := tokenFor(t)
NextDoc:
	for _, dkey := range dkeys {
		change := mgo.Change{
			Update:    bson.D{{Name: "$addToSet", Value: bson.D{{Name: "txn-queue", Value: tt}}}},
			ReturnNew: true,
		}
		c := f.tc.Database.C(dkey.C)
		cquery := c.FindId(dkey.Id).Select(txnFields)

	RetryDoc:
		change.Upsert = false
		chaos("")
		if _, err := cquery.Apply(change, &info); err == nil {
			if f.opts.MaxTxnQueueLength > 0 && len(info.Queue) > f.opts.MaxTxnQueueLength {
				// abort with TXN Queue too long, but remove the entry we just added
				innerErr := c.UpdateId(dkey.Id,
					bson.D{{Name: "$pullAll", Value: bson.D{{Name: "txn-queue", Value: []token{tt}}}}})
				if innerErr != nil {
					f.debugf("error while backing out of queue-too-long: %v", innerErr)
				}
				return nil, fmt.Errorf("txn-queue for %v in %q has too many transactions (%d)",
					dkey.Id, dkey.C, len(info.Queue))
			}
			if info.Remove == "" {
				// Fast path, unless workload is insert/remove heavy.
				revno[dkey] = info.Revno
				f.queue[dkey] = tokensWithIds(info.Queue)
				f.debugf("[A] Prepared document %v with revno %d and queue: %v", dkey, info.Revno, info.Queue)
				continue NextDoc
			} else {
				// Handle remove in progress before preparing it.
				if err := f.loadAndApply(info.Remove); err != nil {
					return nil, err
				}
				goto RetryDoc
			}
		} else if err != mgo.ErrNotFound {
			return nil, err
		}

		// Document missing. Use stash collection.
		change.Upsert = true
		chaos("")
		_, err := f.sc.FindId(dkey).Apply(change, &info)
		if err != nil {
			return nil, err
		}
		if info.Insert != "" {
			// Handle insert in progress before preparing it.
			if err := f.loadAndApply(info.Insert); err != nil {
				return nil, err
			}
			goto RetryDoc
		}

		// Must confirm stash is still in use and is the same one
		// prepared, since applying a remove overwrites the stash.
		docFound := false
		stashFound := false
		if err = c.FindId(dkey.Id).Select(txnFields).One(&info); err == nil {
			docFound = true
		} else if err != mgo.ErrNotFound {
			return nil, err
		} else if err = f.sc.FindId(dkey).One(&info); err == nil {
			stashFound = true
			if info.Revno == 0 {
				// Missing revno in the stash only happens when it
				// has been upserted, in which case it defaults to -1.
				// Txn-inserted documents get revno -1 while in the stash
				// for the first time, and -revno-1 == 2 when they go live.
				info.Revno = -1
			}
		} else if err != mgo.ErrNotFound {
			return nil, err
		}

		if docFound && info.Remove == "" || stashFound && info.Insert == "" {
			for _, dtt := range info.Queue {
				if dtt != tt {
					continue
				}
				// Found tt properly prepared.
				if stashFound {
					f.debugf("[B] Prepared document %v on stash with revno %d and queue: %v", dkey, info.Revno, info.Queue)
				} else {
					f.debugf("[B] Prepared document %v with revno %d and queue: %v", dkey, info.Revno, info.Queue)
				}
				revno[dkey] = info.Revno
				f.queue[dkey] = tokensWithIds(info.Queue)
				continue NextDoc
			}
		}

		// The stash wasn't valid and tt got overwritten. Try again.
		f.unstashToken(tt, dkey)
		goto RetryDoc
	}

	// Save the prepared nonce onto t.
	nonce := tt.nonce()
	qdoc := bson.D{{Name: "_id", Value: t.Id}, {Name: "s", Value: tpreparing}}
	udoc := bson.D{{Name: "$set", Value: bson.D{{Name: "s", Value: tprepared}, {Name: "n", Value: nonce}}}}
	chaos("set-prepared")
	err = f.tc.Update(qdoc, udoc)
	if err == nil {
		t.State = tprepared
		t.Nonce = nonce
	} else if err == mgo.ErrNotFound {
		f.debugf("Can't save nonce of %s: LOST RACE", tt)
		if err := f.reload(t); err != nil {
			return nil, err
		} else if t.State == tpreparing {
			panic("can't save nonce yet transaction is still preparing")
		} else if t.State != tprepared {
			return t.Revnos, nil
		}
		tt = t.token()
	} else if err != nil {
		return nil, err
	}

	prereqs, found := f.hasPreReqs(tt, dkeys)
	if !found {
		// Must only happen when reloading above.
		return f.rescan(t, force)
	} else if prereqs && !force {
		f.debugf("Prepared queue with %s [has prereqs & not forced].", tt)
		return nil, errPreReqs
	}
	revnos = assembledRevnos(t.Ops, revno)
	if !prereqs {
		f.debugf("Prepared queue with %s [no prereqs]. Revnos: %v", tt, revnos)
	} else {
		f.debugf("Prepared queue with %s [forced] Revnos: %v", tt, revnos)
	}
	return revnos, nil
}

func (f *flusher) unstashToken(tt token, dkey docKey) error {
	qdoc := bson.D{{Name: "_id", Value: dkey}, {Name: "txn-queue", Value: tt}}
	udoc := bson.D{{Name: "$pull", Value: bson.D{{Name: "txn-queue", Value: tt}}}}
	chaos("")
	if err := f.sc.Update(qdoc, udoc); err == nil {
		chaos("")
		err = f.sc.Remove(bson.D{{Name: "_id", Value: dkey}, {Name: "txn-queue", Value: bson.D{}}})
	} else if err != mgo.ErrNotFound {
		return err
	}
	return nil
}

func (f *flusher) rescan(t *transaction, force bool) (revnos []int64, err error) {
	f.debugf("Rescanning %s", t)
	if t.State != tprepared {
		panic(fmt.Errorf("rescanning transaction in invalid state: %q", t.State))
	}

	// dkeys being sorted means stable iteration across all
	// runners. This isn't strictly required, but reduces the chances
	// of cycles.
	dkeys := t.docKeys()

	tt := t.token()
	if !force {
		prereqs, found := f.hasPreReqs(tt, dkeys)
		if found && prereqs {
			// Its state is already known.
			return nil, errPreReqs
		}
	}

	revno := make(map[docKey]int64)
	info := txnInfo{}
	for _, dkey := range dkeys {
		const retries = 3
		retry := -1

	RetryDoc:
		retry++
		c := f.tc.Database.C(dkey.C)
		if err := c.FindId(dkey.Id).Select(txnFields).One(&info); err == mgo.ErrNotFound {
			// Document is missing. Look in stash.
			chaos("")
			if err := f.sc.FindId(dkey).One(&info); err == mgo.ErrNotFound {
				// Stash also doesn't exist. Maybe someone applied it.
				if err := f.reload(t); err != nil {
					return nil, err
				} else if t.State != tprepared {
					return t.Revnos, err
				}
				// Not applying either.
				if retry < retries {
					// Retry since there might be an insert/remove race.
					goto RetryDoc
				}
				// Neither the doc nor the stash seem to exist.
				return nil, fmt.Errorf("cannot find document %v for applying transaction %s", dkey, t)
			} else if err != nil {
				return nil, err
			}
			// Stash found.
			if info.Insert != "" {
				// Handle insert in progress before assuming ordering is good.
				if err := f.loadAndApply(info.Insert); err != nil {
					return nil, err
				}
				goto RetryDoc
			}
			if info.Revno == 0 {
				// Missing revno in the stash means -1.
				info.Revno = -1
			}
		} else if err != nil {
			return nil, err
		} else if info.Remove != "" {
			// Handle remove in progress before assuming ordering is good.
			if err := f.loadAndApply(info.Remove); err != nil {
				return nil, err
			}
			goto RetryDoc
		}
		revno[dkey] = info.Revno

		found := false
		for _, Id := range info.Queue {
			if Id == tt {
				found = true
				break
			}
		}
		f.queue[dkey] = tokensWithIds(info.Queue)
		if !found {
			// Rescanned transaction Id was not in the queue. This could mean one
			// of three things:
			//  1) The transaction was applied and popped by someone else. This is
			//     the common case.
			//  2) We've read an out-of-date queue from the stash. This can happen
			//     when someone else was paused for a long while preparing another
			//     transaction for this document, and improperly upserted to the
			//     stash when unpaused (after someone else inserted the document).
			//     This is rare but possible.
			//  3) There's an actual bug somewhere, or outside interference. Worst
			//     possible case.
			f.debugf("Rescanned document %v misses %s in queue: %v", dkey, tt, info.Queue)
			err := f.reload(t)
			if t.State == tpreparing || t.State == tprepared {
				if retry < retries {
					// Case 2.
					goto RetryDoc
				}
				// Case 3.
				return nil, fmt.Errorf("cannot find transaction %s in queue for document %v", t, dkey)
			}
			// Case 1.
			return t.Revnos, err
		}
	}

	prereqs, found := f.hasPreReqs(tt, dkeys)
	if !found {
		panic("rescanning loop guarantees that this can't happen")
	} else if prereqs && !force {
		f.debugf("Rescanned queue with %s: has prereqs, not forced", tt)
		return nil, errPreReqs
	}
	revnos = assembledRevnos(t.Ops, revno)
	if !prereqs {
		f.debugf("Rescanned queue with %s: no prereqs, revnos: %v", tt, revnos)
	} else {
		f.debugf("Rescanned queue with %s: has prereqs, forced, revnos: %v", tt, revnos)
	}
	return revnos, nil
}

func assembledRevnos(ops []Op, revno map[docKey]int64) []int64 {
	revnos := make([]int64, len(ops))
	for i, op := range ops {
		dkey := op.docKey()
		revnos[i] = revno[dkey]
		drevno := revno[dkey]
		switch {
		case op.Insert != nil && drevno < 0:
			revno[dkey] = -drevno + 1
		case op.Update != nil && drevno >= 0:
			revno[dkey] = drevno + 1
		case op.Remove && drevno >= 0:
			revno[dkey] = -drevno - 1
		}
	}
	return revnos
}

func (f *flusher) hasPreReqs(tt token, dkeys docKeys) (prereqs, found bool) {
	found = true
	ttId := tt.id()
NextDoc:
	for _, dkey := range dkeys {
		for _, dtt := range f.queue[dkey] {
			if dtt.tt == tt {
				continue NextDoc
			} else if dtt.Id() != ttId {
				prereqs = true
			}
		}
		found = false
	}
	return
}

func (f *flusher) reload(t *transaction) error {
	var newt transaction
	query := f.tc.FindId(t.Id)
	query.Select(bson.D{{Name: "s", Value: 1}, {Name: "n", Value: 1}, {Name: "r", Value: 1}})
	if err := query.One(&newt); err != nil {
		return fmt.Errorf("failed to reload transaction: %v", err)
	}
	t.State = newt.State
	t.Nonce = newt.Nonce
	t.Revnos = newt.Revnos
	f.debugf("Reloaded %s: %q", t, t.State)
	return nil
}

func (f *flusher) loadAndApply(Id bson.ObjectId) error {
	t, err := f.load(Id)
	if err != nil {
		return err
	}
	return f.advance(t, nil, true)
}

// assert verifies that all assertions in t match the content that t
// will be applied upon. If an assertion fails, the transaction state
// is changed to aborted.
func (f *flusher) assert(t *transaction, revnos []int64, pull map[bson.ObjectId]*transaction) error {
	f.debugf("Asserting %s with revnos %v", t, revnos)
	if t.State != tprepared {
		panic(fmt.Errorf("asserting transaction in invalid state: %q", t.State))
	}
	qdoc := make(bson.D, 3)
	revno := make(map[docKey]int64)
	for i, op := range t.Ops {
		dkey := op.docKey()
		if _, ok := revno[dkey]; !ok {
			revno[dkey] = revnos[i]
		}
		if op.Assert == nil {
			continue
		}
		if op.Assert == DocMissing {
			if revnos[i] >= 0 {
				return f.abortOrReload(t, revnos, pull)
			}
			continue
		}
		if op.Insert != nil {
			return fmt.Errorf("Insert can only Assert txn.DocMissing, was %v", op.Assert)
		}
		// if revnos[i] < 0 { abort }?

		qdoc = append(qdoc[:0], bson.DocElem{Name: "_id", Value: op.Id})
		if op.Assert != DocMissing {
			var revnoq interface{}
			if n := revno[dkey]; n == 0 {
				revnoq = bson.D{{Name: "$exists", Value: false}}
			} else {
				revnoq = n
			}
			// XXX Add tt to the query here, once we're sure it's all working.
			//     Not having it increases the chances of breaking on bad logic.
			qdoc = append(qdoc, bson.DocElem{Name: "txn-revno", Value: revnoq})
			if op.Assert != DocExists {
				qdoc = append(qdoc, bson.DocElem{Name: "$or", Value: []interface{}{op.Assert}})
			}
		}

		c := f.tc.Database.C(op.C)
		if err := c.Find(qdoc).Select(bson.D{{Name: "_id", Value: 1}}).One(nil); err == mgo.ErrNotFound {
			// Assertion failed or someone else started applying.
			return f.abortOrReload(t, revnos, pull)
		} else if err != nil {
			return err
		}
	}
	f.debugf("Asserting %s succeeded", t)
	return nil
}

func (f *flusher) abortOrReload(t *transaction, revnos []int64, pull map[bson.ObjectId]*transaction) (err error) {
	f.debugf("Aborting or reloading %s (was %q)", t, t.State)
	if t.State == tprepared {
		qdoc := bson.D{{Name: "_id", Value: t.Id}, {Name: "s", Value: tprepared}}
		udoc := bson.D{{Name: "$set", Value: bson.D{{Name: "s", Value: taborting}}}}
		chaos("set-aborting")
		if err = f.tc.Update(qdoc, udoc); err == nil {
			t.State = taborting
		} else if err == mgo.ErrNotFound {
			if err = f.reload(t); err != nil || t.State != taborting {
				f.debugf("Won't abort %s. Reloaded state: %q", t, t.State)
				return err
			}
		} else {
			return err
		}
	} else if t.State != taborting {
		panic(fmt.Errorf("aborting transaction in invalid state: %q", t.State))
	}

	if len(revnos) > 0 {
		if pull == nil {
			pull = map[bson.ObjectId]*transaction{t.Id: t}
		}
		seen := make(map[docKey]bool)
		for i, op := range t.Ops {
			dkey := op.docKey()
			if seen[op.docKey()] {
				continue
			}
			seen[dkey] = true

			pullAll := tokensToPull(f.queue[dkey], pull, "")
			if len(pullAll) == 0 {
				continue
			}
			udoc := bson.D{{Name: "$pullAll", Value: bson.D{{Name: "txn-queue", Value: pullAll}}}}
			chaos("")
			if revnos[i] < 0 {
				err = f.sc.UpdateId(dkey, udoc)
			} else {
				c := f.tc.Database.C(dkey.C)
				err = c.UpdateId(dkey.Id, udoc)
			}
			if err != nil && err != mgo.ErrNotFound {
				return err
			}
		}
	}
	udoc := bson.D{{Name: "$set", Value: bson.D{{Name: "s", Value: taborted}}}}
	chaos("set-aborted")
	if err := f.tc.UpdateId(t.Id, udoc); err != nil && err != mgo.ErrNotFound {
		return err
	}
	t.State = taborted
	f.debugf("Aborted %s", t)
	return nil
}

func (f *flusher) checkpoint(t *transaction, revnos []int64) error {
	var debugRevnos map[docKey][]int64
	if debugEnabled {
		debugRevnos = make(map[docKey][]int64)
		for i, op := range t.Ops {
			dkey := op.docKey()
			debugRevnos[dkey] = append(debugRevnos[dkey], revnos[i])
		}
		f.debugf("Ready to apply %s. Saving revnos %v", t, debugRevnos)
	}

	// Save in t the txn-revno values the transaction must run on.
	qdoc := bson.D{{Name: "_id", Value: t.Id}, {Name: "s", Value: tprepared}}
	udoc := bson.D{{Name: "$set", Value: bson.D{{Name: "s", Value: tapplying}, {Name: "r", Value: revnos}}}}
	chaos("set-applying")
	err := f.tc.Update(qdoc, udoc)
	if err == nil {
		t.State = tapplying
		t.Revnos = revnos
		f.debugf("Ready to apply %s. Saving revnos %v: DONE", t, debugRevnos)
	} else if err == mgo.ErrNotFound {
		f.debugf("Ready to apply %s. Saving revnos %v: LOST RACE", t, debugRevnos)
		return f.reload(t)
	}
	return err
}

func (f *flusher) apply(t *transaction, pull map[bson.ObjectId]*transaction) error {
	f.debugf("Applying transaction %s", t)
	if t.State != tapplying {
		panic(fmt.Errorf("applying transaction in invalid state: %q", t.State))
	}
	if pull == nil {
		pull = map[bson.ObjectId]*transaction{t.Id: t}
	}

	logRevnos := append([]int64(nil), t.Revnos...)
	logDoc := bson.D{{Name: "_id", Value: t.Id}}

	tt := tokenFor(t)
	for i := range t.Ops {
		op := &t.Ops[i]
		dkey := op.docKey()
		dqueue := f.queue[dkey]
		revno := t.Revnos[i]

		var opName string
		if debugEnabled {
			opName = op.name()
			f.debugf("Applying %s op %d (%s) on %v with txn-revno %d", t, i, opName, dkey, revno)
		}

		c := f.tc.Database.C(op.C)

		qdoc := bson.D{{Name: "_id", Value: dkey.Id}, {Name: "txn-revno", Value: revno}, {Name: "txn-queue", Value: tt}}
		if op.Insert != nil {
			qdoc[0].Value = dkey
			if revno == -1 {
				qdoc[1].Value = bson.D{{Name: "$exists", Value: false}}
			}
		} else if revno == 0 {
			// There's no document with revno 0. The only way to see it is
			// when an existent document participates in a transaction the
			// first time. Txn-inserted documents get revno -1 while in the
			// stash for the first time, and -revno-1 == 2 when they go live.
			qdoc[1].Value = bson.D{{Name: "$exists", Value: false}}
		}

		pullAll := tokensToPull(dqueue, pull, tt)

		var d bson.D
		var outcome string
		var err error
		switch {
		case op.Update != nil:
			if revno < 0 {
				err = mgo.ErrNotFound
				f.debugf("Won't try to apply update op; negative revision means the document is missing or stashed")
			} else {
				newRevno := revno + 1
				logRevnos[i] = newRevno
				if d, err = objToDoc(op.Update); err != nil {
					return err
				}
				if d, err = addToDoc(d, "$pullAll", bson.D{{Name: "txn-queue", Value: pullAll}}); err != nil {
					return err
				}
				if d, err = addToDoc(d, "$set", bson.D{{Name: "txn-revno", Value: newRevno}}); err != nil {
					return err
				}
				chaos("")
				err = c.Update(qdoc, d)
			}
		case op.Remove:
			if revno < 0 {
				err = mgo.ErrNotFound
			} else {
				newRevno := -revno - 1
				logRevnos[i] = newRevno
				nonce := newNonce()
				stash := txnInfo{}
				change := mgo.Change{
					Update:    bson.D{{Name: "$push", Value: bson.D{{Name: "n", Value: nonce}}}},
					Upsert:    true,
					ReturnNew: true,
				}
				if _, err = f.sc.FindId(dkey).Apply(change, &stash); err != nil {
					return err
				}
				change = mgo.Change{
					Update:    bson.D{{Name: "$set", Value: bson.D{{Name: "txn-remove", Value: t.Id}}}},
					ReturnNew: true,
				}
				var info txnInfo
				if _, err = c.Find(qdoc).Apply(change, &info); err == nil {
					// The document still exists so the stash previously
					// observed was either out of date or necessarily
					// contained the token being applied.
					f.debugf("Marked document %v to be removed on revno %d with queue: %v", dkey, info.Revno, info.Queue)
					updated := false
					if !hasToken(stash.Queue, tt) {
						var set, unset bson.D
						if revno == 0 {
							// Missing revno in stash means -1.
							set = bson.D{{Name: "txn-queue", Value: info.Queue}}
							unset = bson.D{{Name: "n", Value: 1}, {Name: "txn-revno", Value: 1}}
						} else {
							set = bson.D{{Name: "txn-queue", Value: info.Queue}, {Name: "txn-revno", Value: newRevno}}
							unset = bson.D{{Name: "n", Value: 1}}
						}
						qdoc := bson.D{{Name: "_id", Value: dkey}, {Name: "n", Value: nonce}}
						udoc := bson.D{{Name: "$set", Value: set}, {Name: "$unset", Value: unset}}
						if err = f.sc.Update(qdoc, udoc); err == nil {
							updated = true
						} else if err != mgo.ErrNotFound {
							return err
						}
					}
					if updated {
						f.debugf("Updated stash for document %v with revno %d and queue: %v", dkey, newRevno, info.Queue)
					} else {
						f.debugf("Stash for document %v was up-to-date", dkey)
					}
					err = c.Remove(qdoc)
				}
			}
		case op.Insert != nil:
			if revno >= 0 {
				err = mgo.ErrNotFound
			} else {
				newRevno := -revno + 1
				logRevnos[i] = newRevno
				if d, err = objToDoc(op.Insert); err != nil {
					return err
				}
				change := mgo.Change{
					Update:    bson.D{{Name: "$set", Value: bson.D{{Name: "txn-insert", Value: t.Id}}}},
					ReturnNew: true,
				}
				chaos("")
				var info txnInfo
				if _, err = f.sc.Find(qdoc).Apply(change, &info); err == nil {
					f.debugf("Stash for document %v has revno %d and queue: %v", dkey, info.Revno, info.Queue)
					d = setInDoc(d, bson.D{{Name: "_id", Value: op.Id}, {Name: "txn-revno", Value: newRevno}, {Name: "txn-queue", Value: info.Queue}})
					// Unlikely yet unfortunate race in here if this gets seriously
					// delayed. If someone inserts+removes meanwhile, this will
					// reinsert, and there's no way to avoid that while keeping the
					// collection clean or compromising sharding. applyOps can solve
					// the former, but it can't shard (SERVER-1439).
					chaos("insert")
					err = c.Insert(d)
					if err == nil || mgo.IsDup(err) {
						if err == nil {
							f.debugf("New document %v inserted with revno %d and queue: %v", dkey, info.Revno, info.Queue)
						} else {
							f.debugf("Document %v already existed", dkey)
						}
						chaos("")
						if err = f.sc.Remove(qdoc); err == nil {
							f.debugf("Stash for document %v removed", dkey)
						}
					}
				}
			}
		case op.Assert != nil:
			// Pure assertion. No changes to apply.
		}
		if err == nil {
			outcome = "DONE"
		} else if err == mgo.ErrNotFound || mgo.IsDup(err) {
			outcome = "MISS"
			err = nil
		} else {
			outcome = err.Error()
		}
		if debugEnabled {
			f.debugf("Applying %s op %d (%s) on %v with txn-revno %d: %s", t, i, opName, dkey, revno, outcome)
		}
		if err != nil {
			return err
		}

		if f.lc != nil && op.isChange() {
			// Add change to the log document.
			var dr bson.D
			for li := range logDoc {
				elem := &logDoc[li]
				if elem.Name == op.C {
					dr = elem.Value.(bson.D)
					break
				}
			}
			if dr == nil {
				logDoc = append(logDoc, bson.DocElem{Name: op.C, Value: bson.D{{Name: "d", Value: []interface{}{}}, {Name: "r", Value: []int64{}}}})
				dr = logDoc[len(logDoc)-1].Value.(bson.D)
			}
			dr[0].Value = append(dr[0].Value.([]interface{}), op.Id)
			dr[1].Value = append(dr[1].Value.([]int64), logRevnos[i])
		}
	}
	t.State = tapplied

	if f.lc != nil {
		// Insert log document into the changelog collection.
		f.debugf("Inserting %s into change log", t)
		err := f.lc.Insert(logDoc)
		if err != nil && !mgo.IsDup(err) {
			return err
		}
	}

	// It's been applied, so errors are ignored here. It's fine for someone
	// else to win the race and mark it as applied, and it's also fine for
	// it to remain pending until a later point when someone will perceive
	// it has been applied and mark it at such.
	f.debugf("Marking %s as applied", t)
	chaos("set-applied")
	f.tc.Update(
		bson.D{{Name: "_id", Value: t.Id}, {Name: "s", Value: tapplying}},
		bson.D{{Name: "$set", Value: bson.D{{Name: "s", Value: tapplied}}}})
	return nil
}

func tokensToPull(dqueue []tokenAndId, pull map[bson.ObjectId]*transaction, dontPull token) []token {
	var result []token
	for j := len(dqueue) - 1; j >= 0; j-- {
		dtt := dqueue[j]
		if dtt.tt == dontPull {
			continue
		}
		if _, ok := pull[dtt.Id()]; ok {
			// It was handled before and this is a leftover invalid
			// nonce in the queue. Cherry-pick it out.
			result = append(result, dtt.tt)
		}
	}
	return result
}

func objToDoc(obj interface{}) (d bson.D, err error) {
	data, err := bson.Marshal(obj)
	if err != nil {
		return nil, err
	}
	err = bson.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}
	return d, err
}

func addToDoc(doc bson.D, key string, add bson.D) (bson.D, error) {
	for i := range doc {
		elem := &doc[i]
		if elem.Name != key {
			continue
		}
		old, ok := elem.Value.(bson.D)
		if !ok {
			return nil, fmt.Errorf("invalid %q value in change document: %#v", key, elem.Value)
		}
		elem.Value = append(old, add...)
		return doc, nil
	}
	return append(doc, bson.DocElem{Name: key, Value: add}), nil
}

func setInDoc(doc bson.D, set bson.D) bson.D {
	dlen := len(doc)
NextS:
	for s := range set {
		sname := set[s].Name
		for d := 0; d < dlen; d++ {
			if doc[d].Name == sname {
				doc[d].Value = set[s].Value
				continue NextS
			}
		}
		doc = append(doc, set[s])
	}
	return doc
}

func hasToken(tokens []token, tt token) bool {
	for _, ttt := range tokens {
		if ttt == tt {
			return true
		}
	}
	return false
}

func (f *flusher) debugf(format string, args ...interface{}) {
	if !debugEnabled {
		return
	}
	debugf(f.debugId+format, args...)
}
