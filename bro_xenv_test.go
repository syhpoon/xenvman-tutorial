// +build xenvman

package main

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"encoding/json"
	"net/http"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/require"
	"github.com/syhpoon/xenvman/pkg/client"
	"github.com/syhpoon/xenvman/pkg/def"
)

const broTestPort = 9988

func TestXenvmanBro(t *testing.T) {
	cl := client.New(client.Params{})

	env := cl.MustCreateEnv(&def.InputEnv{
		Name:        "bro-test",
		Description: "Testing Bro API",
		Templates: []*def.Tpl{
			{
				Tpl: "bro",
				Parameters: def.TplParams{
					"binary": client.FileToBase64("xenvman-tutorial"),
					"port":   broTestPort,
				},
			},
			{
				Tpl: "mongo",
			},
		},

		Options: &def.EnvOptions{
			KeepAlive: def.Duration(2 * time.Minute),
		},
	})

	defer env.Terminate()

	testBroadcast(env, t)
}

// Poll with two clients and send a message from the third one
// Verify both received the message
// Verify the message was saved to mongo db
func testBroadcast(env *client.Env, t *testing.T) {
	broCont, err := env.GetContainer("bro", 0, "bro")
	require.Nil(t, err)

	portStr := fmt.Sprintf("%d", broTestPort)

	mongoCont, err := env.GetContainer("mongo", 0, "mongo")
	require.Nil(t, err)

	pollUrl := fmt.Sprintf("http://%s:%d/v1/poll/",
		env.ExternalAddress, broCont.Ports[portStr])

	postUrl := fmt.Sprintf("http://%s:%d/v1/bro",
		env.ExternalAddress, broCont.Ports[portStr])

	mongoUrl := fmt.Sprintf("%s:%d/bro",
		env.ExternalAddress, mongoCont.Ports["27017"])

	// 1. Start pollers
	msgCh := make(chan *BroMessage, 2)
	errCh := make(chan error, 2)

	go poller(pollUrl+"1", msgCh, errCh)
	go poller(pollUrl+"2", msgCh, errCh)

	msg := &BroMessage{
		From:    "3",
		Message: "wut!?",
		Angry:   true,
	}

	// Post the message
	postBody, err := json.Marshal(msg)
	require.Nil(t, err)

	resp, err := http.Post(postUrl, "test/javascript",
		bytes.NewReader(postBody))

	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Wait for pollers
	got := 0

	for got < 2 {
		select {
		case err := <-errCh:
			panic(fmt.Errorf("Poller error: %+v", err))
		case rmsg := <-msgCh:
			require.Equal(t, msg, rmsg)
			got += 1
		case <-time.After(5 * time.Second):
			panic(fmt.Errorf("Poller timeout"))
		}
	}

	// Make sure the message was saved to mongo
	t.Logf("Connecting to Mongo at %s", mongoUrl)

	db, err := mgo.Dial(mongoUrl)
	require.Nil(t, err)

	db.SetSocketTimeout(0)
	defer db.Close()

	col := db.DB("bro").C("messages")

	mongoMsg := dbMessage{}
	err = col.Find(bson.M{}).One(&mongoMsg)
	require.Nil(t, err)

	require.Equal(t, *msg, *mongoMsg.Msg)
}

func poller(url string, msgCh chan<- *BroMessage, errCh chan<- error) {
	cl := http.Client{Timeout: 10 * time.Second}

	resp, err := cl.Get(url)

	if err != nil {
		errCh <- err

		return
	}

	if resp.StatusCode != http.StatusOK {
		errCh <- fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	msg := BroMessage{}

	if err := dec.Decode(&msg); err != nil {
		errCh <- err

		return
	}

	msgCh <- &msg
}
