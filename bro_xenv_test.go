// +build xenvman

package main

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/stretchr/testify/require"
	"github.com/syhpoon/xenvman/pkg/client"
	"github.com/syhpoon/xenvman/pkg/def"
)

const broTestPort = 9988

func TestXenvmanBro(t *testing.T) {
	cl := client.New(client.Params{})

	env := cl.MustCreateEnv(&def.Env{
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

	mongoCont, err := env.GetContainer("mongo", 0, "mongo")
	require.Nil(t, err)

	msg := &BroMessage{
		From:    "3",
		Message: "wut!?",
		Angry:   true,
	}

	pollUrl := fmt.Sprintf("http://%s/v1/poll/", broCont.Ports[broTestPort])
	postUrl := fmt.Sprintf("http://%s/v1/bro", broCont.Ports[broTestPort])
	mongoUrl := fmt.Sprintf("%s/bro", mongoCont.Ports[27017])

	// 1. Start pollers
	msgCh := make(chan *BroMessage, 2)
	errCh := make(chan error, 2)

	go poller(pollUrl+"1", msgCh, errCh)
	go poller(pollUrl+"2", msgCh, errCh)

	// Post the message
	postBody, err := json.Marshal(msg)
	require.Nil(t, err)

	resp, err := http.Post(postUrl, "test/javascript", bytes.NewReader(postBody))
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

	// TODO: Mongo
	_ = mongoUrl
}

func poller(url string, msgCh chan<- *BroMessage, errCh chan<- error) {
	cl := http.Client{Timeout: 10 * time.Second}

	resp, err := cl.Get(url)

	if err != nil {
		errCh <- err

		return
	}

	defer resp.Body.Close()

	rb, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		errCh <- err

		return
	}

	msg := BroMessage{}

	if err := json.Unmarshal(rb, &msg); err != nil {
		errCh <- err

		return
	}

	fmt.Printf(">>>> %+v\n", msg)
	msgCh <- &msg
}
