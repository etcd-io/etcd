package mgo

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/globalsign/mgo/bson"
)

type FullDocument string

const (
	Default      = "default"
	UpdateLookup = "updateLookup"
)

type ChangeStream struct {
	iter           *Iter
	isClosed       bool
	options        ChangeStreamOptions
	pipeline       interface{}
	resumeToken    *bson.Raw
	collection     *Collection
	readPreference *ReadPreference
	err            error
	m              sync.Mutex
	sessionCopied  bool
}

type ChangeStreamOptions struct {

	// FullDocument controls the amount of data that the server will return when
	// returning a changes document.
	FullDocument FullDocument

	// ResumeAfter specifies the logical starting point for the new change stream.
	ResumeAfter *bson.Raw

	// MaxAwaitTimeMS specifies the maximum amount of time for the server to wait
	// on new documents to satisfy a change stream query.
	MaxAwaitTimeMS time.Duration

	// BatchSize specifies the number of documents to return per batch.
	BatchSize int

	// Collation specifies the way the server should collate returned data.
	//TODO Collation *Collation
}

var errMissingResumeToken = errors.New("resume token missing from result")

// Watch constructs a new ChangeStream capable of receiving continuing data
// from the database.
func (coll *Collection) Watch(pipeline interface{},
	options ChangeStreamOptions) (*ChangeStream, error) {

	if pipeline == nil {
		pipeline = []bson.M{}
	}

	csPipe := constructChangeStreamPipeline(pipeline, options)
	pipe := coll.Pipe(&csPipe)
	if options.MaxAwaitTimeMS > 0 {
		pipe.SetMaxTime(options.MaxAwaitTimeMS)
	}
	if options.BatchSize > 0 {
		pipe.Batch(options.BatchSize)
	}
	pIter := pipe.Iter()

	// check that there was no issue creating the iterator.
	// this will fail immediately with an error from the server if running against
	// a standalone.
	if err := pIter.Err(); err != nil {
		return nil, err
	}

	pIter.isChangeStream = true
	return &ChangeStream{
		iter:        pIter,
		collection:  coll,
		resumeToken: nil,
		options:     options,
		pipeline:    pipeline,
	}, nil
}

// Next retrieves the next document from the change stream, blocking if necessary.
// Next returns true if a document was successfully unmarshalled into result,
// and false if an error occured. When Next returns false, the Err method should
// be called to check what error occurred during iteration. If there were no events
// available (ErrNotFound), the Err method returns nil so the user can retry the invocaton.
//
// For example:
//
//    pipeline := []bson.M{}
//
//    changeStream := collection.Watch(pipeline, ChangeStreamOptions{})
//    for changeStream.Next(&changeDoc) {
//        fmt.Printf("Change: %v\n", changeDoc)
//    }
//
//    if err := changeStream.Close(); err != nil {
//        return err
//    }
//
// If the pipeline used removes the _id field from the result, Next will error
// because the _id field is needed to resume iteration when an error occurs.
//
func (changeStream *ChangeStream) Next(result interface{}) bool {
	// the err field is being constantly overwritten and we don't want the user to
	// attempt to read it at this point so we lock.
	changeStream.m.Lock()

	defer changeStream.m.Unlock()

	// if we are in a state of error, then don't continue.
	if changeStream.err != nil {
		return false
	}

	if changeStream.isClosed {
		changeStream.err = fmt.Errorf("illegal use of a closed ChangeStream")
		return false
	}

	var err error

	// attempt to fetch the change stream result.
	err = changeStream.fetchResultSet(result)
	if err == nil {
		return true
	}

	// if we get no results we return false with no errors so the user can call Next
	// again, resuming is not needed as the iterator is simply timed out as no events happened.
	// The user will call Timeout in order to understand if this was the case.
	if err == ErrNotFound {
		return false
	}

	// check if the error is resumable
	if !isResumableError(err) {
		// error is not resumable, give up and return it to the user.
		changeStream.err = err
		return false
	}

	// try to resume.
	err = changeStream.resume()
	if err != nil {
		// we've not been able to successfully resume and should only try once,
		// so we give up.
		changeStream.err = err
		return false
	}

	// we've successfully resumed the changestream.
	// try to fetch the next result.
	err = changeStream.fetchResultSet(result)
	if err != nil {
		changeStream.err = err
		return false
	}

	return true
}

// Err returns nil if no errors happened during iteration, or the actual
// error otherwise.
func (changeStream *ChangeStream) Err() error {
	changeStream.m.Lock()
	defer changeStream.m.Unlock()
	return changeStream.err
}

// Close kills the server cursor used by the iterator, if any, and returns
// nil if no errors happened during iteration, or the actual error otherwise.
func (changeStream *ChangeStream) Close() error {
	changeStream.m.Lock()
	defer changeStream.m.Unlock()
	changeStream.isClosed = true
	err := changeStream.iter.Close()
	if err != nil {
		changeStream.err = err
	}
	if changeStream.sessionCopied {
		changeStream.iter.session.Close()
		changeStream.sessionCopied = false
	}
	return err
}

// ResumeToken returns a copy of the current resume token held by the change stream.
// This token should be treated as an opaque token that can be provided to instantiate
// a new change stream.
func (changeStream *ChangeStream) ResumeToken() *bson.Raw {
	changeStream.m.Lock()
	defer changeStream.m.Unlock()
	if changeStream.resumeToken == nil {
		return nil
	}
	var tokenCopy = *changeStream.resumeToken
	return &tokenCopy
}

// Timeout returns true if the last call of Next returned false because of an iterator timeout.
func (changeStream *ChangeStream) Timeout() bool {
	return changeStream.iter.Timeout()
}

func constructChangeStreamPipeline(pipeline interface{},
	options ChangeStreamOptions) interface{} {
	pipelinev := reflect.ValueOf(pipeline)

	// ensure that the pipeline passed in is a slice.
	if pipelinev.Kind() != reflect.Slice {
		panic("pipeline argument must be a slice")
	}

	// construct the options to be used by the change notification
	// pipeline stage.
	changeStreamStageOptions := bson.M{}

	if options.FullDocument != "" {
		changeStreamStageOptions["fullDocument"] = options.FullDocument
	}
	if options.ResumeAfter != nil {
		changeStreamStageOptions["resumeAfter"] = options.ResumeAfter
	}

	changeStreamStage := bson.M{"$changeStream": changeStreamStageOptions}

	pipeOfInterfaces := make([]interface{}, pipelinev.Len()+1)

	// insert the change notification pipeline stage at the beginning of the
	// aggregation.
	pipeOfInterfaces[0] = changeStreamStage

	// convert the passed in slice to a slice of interfaces.
	for i := 0; i < pipelinev.Len(); i++ {
		pipeOfInterfaces[1+i] = pipelinev.Index(i).Addr().Interface()
	}
	var pipelineAsInterface interface{} = pipeOfInterfaces
	return pipelineAsInterface
}

func (changeStream *ChangeStream) resume() error {
	// copy the information for the new socket.

	// Thanks to Copy() future uses will acquire a new socket against the newly selected DB.
	newSession := changeStream.iter.session.Copy()

	// fetch the cursor from the iterator and use it to run a killCursors
	// on the connection.
	cursorId := changeStream.iter.op.cursorId
	err := runKillCursorsOnSession(newSession, cursorId)
	if err != nil {
		return err
	}

	// change out the old connection to the database with the new connection.
	if changeStream.sessionCopied {
		changeStream.collection.Database.Session.Close()
	}
	changeStream.collection.Database.Session = newSession
	changeStream.sessionCopied = true

	opts := changeStream.options
	if changeStream.resumeToken != nil {
		opts.ResumeAfter = changeStream.resumeToken
	}
	// make a new pipeline containing the resume token.
	changeStreamPipeline := constructChangeStreamPipeline(changeStream.pipeline, opts)

	// generate the new iterator with the new connection.
	newPipe := changeStream.collection.Pipe(changeStreamPipeline)
	changeStream.iter = newPipe.Iter()
	if err := changeStream.iter.Err(); err != nil {
		return err
	}
	changeStream.iter.isChangeStream = true
	return nil
}

// fetchResumeToken unmarshals the _id field from the document, setting an error
// on the changeStream if it is unable to.
func (changeStream *ChangeStream) fetchResumeToken(rawResult *bson.Raw) error {
	changeStreamResult := struct {
		ResumeToken *bson.Raw `bson:"_id,omitempty"`
	}{}

	err := rawResult.Unmarshal(&changeStreamResult)
	if err != nil {
		return err
	}

	if changeStreamResult.ResumeToken == nil {
		return errMissingResumeToken
	}

	changeStream.resumeToken = changeStreamResult.ResumeToken
	return nil
}

func (changeStream *ChangeStream) fetchResultSet(result interface{}) error {
	rawResult := bson.Raw{}

	// fetch the next set of documents from the cursor.
	gotNext := changeStream.iter.Next(&rawResult)
	err := changeStream.iter.Err()
	if err != nil {
		return err
	}

	if !gotNext && err == nil {
		// If the iter.Err() method returns nil despite us not getting a next batch,
		// it is becuase iter.Err() silences this case.
		return ErrNotFound
	}

	// grab the resumeToken from the results
	if err := changeStream.fetchResumeToken(&rawResult); err != nil {
		return err
	}

	// put the raw results into the data structure the user provided.
	if err := rawResult.Unmarshal(result); err != nil {
		return err
	}
	return nil
}

func isResumableError(err error) bool {
	_, isQueryError := err.(*QueryError)
	// if it is not a database error OR it is a database error,
	// but the error is a notMaster error
	//and is not a missingResumeToken error (caused by the user provided pipeline)
	return (!isQueryError || isNotMasterError(err)) && (err != errMissingResumeToken)
}

func runKillCursorsOnSession(session *Session, cursorId int64) error {
	socket, err := session.acquireSocket(true)
	if err != nil {
		return err
	}
	err = socket.Query(&killCursorsOp{[]int64{cursorId}})
	if err != nil {
		return err
	}
	socket.Release()

	return nil
}
