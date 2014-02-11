package raft

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

//------------------------------------------------------------------------------
//
// Tests
//
//------------------------------------------------------------------------------

//--------------------------------------
// Append
//--------------------------------------

// Ensure that we can append to a new log.
func TestLogNewLog(t *testing.T) {
	path := getLogPath()
	log := newLog()
	log.ApplyFunc = func(e *LogEntry, c Command) (interface{}, error) {
		return nil, nil
	}
	if err := log.open(path); err != nil {
		t.Fatalf("Unable to open log: %v", err)
	}
	defer log.close()
	defer os.Remove(path)

	e, _ := newLogEntry(log, nil, 1, 1, &testCommand1{Val: "foo", I: 20})
	if err := log.appendEntry(e); err != nil {
		t.Fatalf("Unable to append: %v", err)
	}
	e, _ = newLogEntry(log, nil, 2, 1, &testCommand2{X: 100})
	if err := log.appendEntry(e); err != nil {
		t.Fatalf("Unable to append: %v", err)
	}
	e, _ = newLogEntry(log, nil, 3, 2, &testCommand1{Val: "bar", I: 0})
	if err := log.appendEntry(e); err != nil {
		t.Fatalf("Unable to append: %v", err)
	}

	// Partial commit.
	if err := log.setCommitIndex(2); err != nil {
		t.Fatalf("Unable to partially commit: %v", err)
	}
	if index, term := log.commitInfo(); index != 2 || term != 1 {
		t.Fatalf("Invalid commit info [IDX=%v, TERM=%v]", index, term)
	}

	// Full commit.
	if err := log.setCommitIndex(3); err != nil {
		t.Fatalf("Unable to commit: %v", err)
	}
	if index, term := log.commitInfo(); index != 3 || term != 2 {
		t.Fatalf("Invalid commit info [IDX=%v, TERM=%v]", index, term)
	}
}

// Ensure that we can decode and encode to an existing log.
func TestLogExistingLog(t *testing.T) {
	tmpLog := newLog()
	e0, _ := newLogEntry(tmpLog, nil, 1, 1, &testCommand1{Val: "foo", I: 20})
	e1, _ := newLogEntry(tmpLog, nil, 2, 1, &testCommand2{X: 100})
	e2, _ := newLogEntry(tmpLog, nil, 3, 2, &testCommand1{Val: "bar", I: 0})
	log, path := setupLog([]*LogEntry{e0, e1, e2})
	defer log.close()
	defer os.Remove(path)

	// Validate existing log entries.
	if len(log.entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(log.entries))
	}
	if log.entries[0].Index() != 1 || log.entries[0].Term() != 1 {
		t.Fatalf("Unexpected entry[0]: %v", log.entries[0])
	}
	if log.entries[1].Index() != 2 || log.entries[1].Term() != 1 {
		t.Fatalf("Unexpected entry[1]: %v", log.entries[1])
	}
	if log.entries[2].Index() != 3 || log.entries[2].Term() != 2 {
		t.Fatalf("Unexpected entry[2]: %v", log.entries[2])
	}
}

// Ensure that we can check the contents of the log by index/term.
func TestLogContainsEntries(t *testing.T) {
	tmpLog := newLog()
	e0, _ := newLogEntry(tmpLog, nil, 1, 1, &testCommand1{Val: "foo", I: 20})
	e1, _ := newLogEntry(tmpLog, nil, 2, 1, &testCommand2{X: 100})
	e2, _ := newLogEntry(tmpLog, nil, 3, 2, &testCommand1{Val: "bar", I: 0})
	log, path := setupLog([]*LogEntry{e0, e1, e2})
	defer log.close()
	defer os.Remove(path)

	if log.containsEntry(0, 0) {
		t.Fatalf("Zero-index entry should not exist in log.")
	}
	if log.containsEntry(1, 0) {
		t.Fatalf("Entry with mismatched term should not exist")
	}
	if log.containsEntry(4, 0) {
		t.Fatalf("Out-of-range entry should not exist")
	}
	if !log.containsEntry(2, 1) {
		t.Fatalf("Entry 2/1 should exist")
	}
	if !log.containsEntry(3, 2) {
		t.Fatalf("Entry 2/1 should exist")
	}
}

// Ensure that we can recover from an incomplete/corrupt log and continue logging.
func TestLogRecovery(t *testing.T) {
	tmpLog := newLog()
	e0, _ := newLogEntry(tmpLog, nil, 1, 1, &testCommand1{Val: "foo", I: 20})
	e1, _ := newLogEntry(tmpLog, nil, 2, 1, &testCommand2{X: 100})
	f, _ := ioutil.TempFile("", "raft-log-")

	e0.Encode(f)
	e1.Encode(f)
	f.WriteString("CORRUPT!")
	f.Close()

	log := newLog()
	log.ApplyFunc = func(e *LogEntry, c Command) (interface{}, error) {
		return nil, nil
	}
	if err := log.open(f.Name()); err != nil {
		t.Fatalf("Unable to open log: %v", err)
	}
	defer log.close()
	defer os.Remove(f.Name())

	e, _ := newLogEntry(log, nil, 3, 2, &testCommand1{Val: "bat", I: -5})
	if err := log.appendEntry(e); err != nil {
		t.Fatalf("Unable to append: %v", err)
	}

	// Validate existing log entries.
	if len(log.entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(log.entries))
	}
	if log.entries[0].Index() != 1 || log.entries[0].Term() != 1 {
		t.Fatalf("Unexpected entry[0]: %v", log.entries[0])
	}
	if log.entries[1].Index() != 2 || log.entries[1].Term() != 1 {
		t.Fatalf("Unexpected entry[1]: %v", log.entries[1])
	}
	if log.entries[2].Index() != 3 || log.entries[2].Term() != 2 {
		t.Fatalf("Unexpected entry[2]: %v", log.entries[2])
	}
}

//--------------------------------------
// Append
//--------------------------------------

// Ensure that we can truncate uncommitted entries in the log.
func TestLogTruncate(t *testing.T) {
	log, path := setupLog(nil)
	if err := log.open(path); err != nil {
		t.Fatalf("Unable to open log: %v", err)
	}

	defer os.Remove(path)

	entry1, _ := newLogEntry(log, nil, 1, 1, &testCommand1{Val: "foo", I: 20})
	if err := log.appendEntry(entry1); err != nil {
		t.Fatalf("Unable to append: %v", err)
	}
	entry2, _ := newLogEntry(log, nil, 2, 1, &testCommand2{X: 100})
	if err := log.appendEntry(entry2); err != nil {
		t.Fatalf("Unable to append: %v", err)
	}
	entry3, _ := newLogEntry(log, nil, 3, 2, &testCommand1{Val: "bar", I: 0})
	if err := log.appendEntry(entry3); err != nil {
		t.Fatalf("Unable to append: %v", err)
	}
	if err := log.setCommitIndex(2); err != nil {
		t.Fatalf("Unable to partially commit: %v", err)
	}

	// Truncate committed entry.
	if err := log.truncate(1, 1); err == nil || err.Error() != "raft.Log: Index is already committed (2): (IDX=1, TERM=1)" {
		t.Fatalf("Truncating committed entries shouldn't work: %v", err)
	}
	// Truncate past end of log.
	if err := log.truncate(4, 2); err == nil || err.Error() != "raft.Log: Entry index does not exist (MAX=3): (IDX=4, TERM=2)" {
		t.Fatalf("Truncating past end-of-log shouldn't work: %v", err)
	}
	// Truncate entry with mismatched term.
	if err := log.truncate(2, 2); err == nil || err.Error() != "raft.Log: Entry at index does not have matching term (1): (IDX=2, TERM=2)" {
		t.Fatalf("Truncating mismatched entries shouldn't work: %v", err)
	}
	// Truncate end of log.
	if err := log.truncate(3, 2); !(err == nil && reflect.DeepEqual(log.entries, []*LogEntry{entry1, entry2, entry3})) {
		t.Fatalf("Truncating end of log should work: %v\n\nEntries:\nActual: %v\nExpected: %v", err, log.entries, []*LogEntry{entry1, entry2, entry3})
	}
	// Truncate at last commit.
	if err := log.truncate(2, 1); !(err == nil && reflect.DeepEqual(log.entries, []*LogEntry{entry1, entry2})) {
		t.Fatalf("Truncating at last commit should work: %v\n\nEntries:\nActual: %v\nExpected: %v", err, log.entries, []*LogEntry{entry1, entry2})
	}

	// Append after truncate
	if err := log.appendEntry(entry3); err != nil {
		t.Fatalf("Unable to append after truncate: %v", err)
	}

	log.close()

	// Recovery the truncated log
	log = newLog()
	if err := log.open(path); err != nil {
		t.Fatalf("Unable to open log: %v", err)
	}
	// Validate existing log entries.
	if len(log.entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(log.entries))
	}
	if log.entries[0].Index() != 1 || log.entries[0].Term() != 1 {
		t.Fatalf("Unexpected entry[0]: %v", log.entries[0])
	}
	if log.entries[1].Index() != 2 || log.entries[1].Term() != 1 {
		t.Fatalf("Unexpected entry[1]: %v", log.entries[1])
	}
	if log.entries[2].Index() != 3 || log.entries[2].Term() != 2 {
		t.Fatalf("Unexpected entry[2]: %v", log.entries[2])
	}
}
