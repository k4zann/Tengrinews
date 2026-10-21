// The fake speaks the driver's handshake (legacy OP_QUERY hello, maxWireVersion 21); it may need updating when the driver is bumped.

package mongostore_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"tengrinews/internal/mongostore"
	"tengrinews/internal/news"
)

// fakeMongo speaks just enough of the MongoDB wire protocol to open a store and answer the
// store's count, list and find commands. It records every command, and a command named in
// hang is read but never answered, like a MongoDB that stopped responding.
type fakeMongo struct {
	uri  string
	hang string
	seen chan string // command names as they arrive; a send never blocks

	mu   sync.Mutex
	cmds []bson.D
}

func newFakeMongo(t *testing.T, hang string) *fakeMongo {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeMongo{uri: "mongodb://" + ln.Addr().String() + "/?directConnection=true", hang: hang, seen: make(chan string, 256)}

	var mu sync.Mutex
	var conns []net.Conn
	t.Cleanup(func() { // closing the connections ends the serve loops
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
			go f.serve(c)
		}
	}()
	return f
}

// commands returns the names of the commands received so far, lowercased, without the
// driver's background hello/isMaster heartbeats.
func (f *fakeMongo) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.cmds {
		if name := strings.ToLower(c[0].Key); name != "hello" && name != "ismaster" {
			out = append(out, name)
		}
	}
	return out
}

// find returns the find commands received so far.
func (f *fakeMongo) find() []bson.D {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []bson.D
	for _, c := range f.cmds {
		if c[0].Key == "find" {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeMongo) serve(c net.Conn) {
	for {
		var hdr [16]byte
		if _, err := io.ReadFull(c, hdr[:]); err != nil {
			return
		}
		reqID := binary.LittleEndian.Uint32(hdr[4:8])
		op := binary.LittleEndian.Uint32(hdr[12:16])
		body := make([]byte, binary.LittleEndian.Uint32(hdr[0:4])-16)
		if _, err := io.ReadFull(c, body); err != nil {
			return
		}

		var cmd bson.D
		var out []byte
		switch op {
		case 2004: // OP_QUERY: the legacy handshake
			rest := body[4:]                           // flags
			rest = rest[bytes.IndexByte(rest, 0)+1+8:] // collection name, skip, return
			if bson.Unmarshal(rest, &cmd) != nil || len(cmd) == 0 {
				return
			}
			reply := f.record(cmd)
			if reply == nil {
				continue
			}
			doc, _ := bson.Marshal(reply)
			head := make([]byte, 20) // OP_REPLY: flags, cursor id, starting from, number returned
			binary.LittleEndian.PutUint32(head[0:], 8)
			binary.LittleEndian.PutUint32(head[16:], 1)
			out = frame(1, reqID, append(head, doc...))
		case 2013: // OP_MSG
			if body[4] != 0 || bson.Unmarshal(body[5:], &cmd) != nil || len(cmd) == 0 {
				return
			}
			reply := f.record(cmd)
			if reply == nil {
				continue
			}
			doc, _ := bson.Marshal(reply)
			out = frame(2013, reqID, append([]byte{0, 0, 0, 0, 0}, doc...))
		default:
			return
		}
		if _, err := c.Write(out); err != nil {
			return
		}
	}
}

func frame(op, respTo uint32, payload []byte) []byte {
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h[0:], uint32(16+len(payload)))
	binary.LittleEndian.PutUint32(h[8:], respTo)
	binary.LittleEndian.PutUint32(h[12:], op)
	return append(h, payload...)
}

// record stores cmd and returns the reply, or nil when the command is the one to hang on.
func (f *fakeMongo) record(cmd bson.D) bson.D {
	name := cmd[0].Key
	f.mu.Lock()
	f.cmds = append(f.cmds, cmd)
	f.mu.Unlock()
	select {
	case f.seen <- name:
	default:
	}
	if name == f.hang {
		return nil
	}

	ok := bson.E{Key: "ok", Value: 1.0}
	batch := func(docs ...bson.D) bson.D {
		arr := make(bson.A, len(docs))
		for i, d := range docs {
			arr[i] = d
		}
		return bson.D{{Key: "cursor", Value: bson.D{{Key: "id", Value: int64(0)}, {Key: "ns", Value: "news.articles"}, {Key: "firstBatch", Value: arr}}}, ok}
	}
	switch strings.ToLower(name) {
	case "ismaster", "hello":
		return bson.D{{Key: "ismaster", Value: true}, {Key: "isWritablePrimary", Value: true}, {Key: "helloOk", Value: true},
			{Key: "maxBsonObjectSize", Value: int32(16777216)}, {Key: "maxMessageSizeBytes", Value: int32(48000000)},
			{Key: "maxWriteBatchSize", Value: int32(100000)}, {Key: "minWireVersion", Value: int32(0)}, {Key: "maxWireVersion", Value: int32(21)}, ok}
	case "aggregate":
		if strings.Contains(fmt.Sprint(cmd), "$group") { // the count pipeline
			return batch(bson.D{{Key: "_id", Value: nil}, {Key: "n", Value: int32(2)}})
		}
		return batch(articleDoc("a01"), articleDoc("a02"))
	case "find":
		for _, e := range cmd {
			if e.Key == "filter" && reflect.DeepEqual(e.Value, bson.D{{Key: "articles.article_id", Value: "a07"}}) {
				return batch(bson.D{{Key: "_id", Value: int32(1)}, {Key: "articles", Value: bson.A{articleDoc("a07")}}})
			}
		}
		return batch()
	}
	return bson.D{ok}
}

func articleDoc(id string) bson.D {
	return bson.D{{Key: "article_id", Value: id}, {Key: "title", Value: "title " + id}}
}

func openFake(t *testing.T, f *fakeMongo) *mongostore.Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := mongostore.Open(ctx, f.uri, "news")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

// The store only reads, and a post is found with one find on articles.article_id that
// projects just the matching array element.
func TestStoreSendsOnlyReadCommands(t *testing.T) {
	f := newFakeMongo(t, "")
	store := openFake(t, f)
	ctx := context.Background()

	if n, err := store.CountArticles(ctx); err != nil || n != 2 {
		t.Fatalf("CountArticles = %d, %v", n, err)
	}
	if got, err := store.ListArticles(ctx, 10, 10); err != nil || len(got) != 2 {
		t.Fatalf("ListArticles = %v, %v", got, err)
	}

	before := len(f.commands())
	got, err := store.ArticleByID(ctx, "a07")
	if err != nil || got.ID != "a07" || got.Title != "title a07" {
		t.Fatalf("ArticleByID(a07) = %+v, %v", got, err)
	}
	if after := f.commands()[before:]; !slices.Equal(after, []string{"find"}) {
		t.Errorf("one lookup sent %v, want a single find", after)
	}
	if _, err := store.ArticleByID(ctx, "nope"); !errors.Is(err, news.ErrNotFound) {
		t.Errorf("ArticleByID(nope) = %v, want news.ErrNotFound", err)
	}

	finds := f.find()
	if len(finds) != 2 {
		t.Fatalf("%d find commands, want 2", len(finds))
	}
	byKey := map[string]any{}
	for _, e := range finds[0] {
		byKey[e.Key] = e.Value
	}
	if byKey["find"] != "articles" || byKey["$db"] != "news" {
		t.Errorf("find targets %v.%v, want news.articles", byKey["$db"], byKey["find"])
	}
	if want := (bson.D{{Key: "articles.article_id", Value: "a07"}}); !reflect.DeepEqual(byKey["filter"], want) {
		t.Errorf("filter = %v, want %v", byKey["filter"], want)
	}
	if want := (bson.D{{Key: "articles.$", Value: int32(1)}}); !reflect.DeepEqual(byKey["projection"], want) {
		t.Errorf("projection = %v, want %v: only the matching element may come back", byKey["projection"], want)
	}

	if err := store.Close(ctx); err != nil {
		t.Errorf("Close: %v", err)
	}
	read := []string{"aggregate", "find", "ping", "endsessions", "killcursors", "getmore"}
	for _, name := range f.commands() {
		if !slices.Contains(read, name) {
			t.Errorf("the store sent %q, which is not a read command", name)
		}
	}
}

// waitFor returns once the fake has received a command called name.
func (f *fakeMongo) waitFor(t *testing.T, name string) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case got := <-f.seen:
			if got == name {
				return
			}
		case <-timeout:
			t.Fatalf("the fake never received %q", name)
		}
	}
}

func TestClientDisconnectCancelsOperation(t *testing.T) {
	f := newFakeMongo(t, "aggregate")
	store := openFake(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := store.CountArticles(ctx)
		done <- err
	}()
	f.waitFor(t, "aggregate") // the query is on the wire and unanswered
	cancel()                  // the client goes away

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("CountArticles = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second): // the 5 s operation timeout would not be soon enough
		t.Fatal("the operation kept running after its context was cancelled")
	}
}

func TestHungOperationEndsAfterAboutFiveSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the 5 s operation timeout")
	}
	f := newFakeMongo(t, "aggregate")
	store := openFake(t, f)

	start := time.Now()
	_, err := store.CountArticles(context.Background()) // no deadline: the client's 5 s timeout applies
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("CountArticles succeeded against a MongoDB that never answers")
	}
	if elapsed < 4*time.Second || elapsed > 9*time.Second {
		t.Errorf("gave up after %v, want about 5s (err: %v)", elapsed, err)
	}
}
