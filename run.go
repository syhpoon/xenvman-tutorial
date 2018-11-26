/*
 MIT License

 Copyright (c) 2018 Max Kuznetsov <syhpoon@syhpoon.ca>

 Permission is hereby granted, free of charge, to any person obtaining a copy
 of this software and associated documentation files (the "Software"), to deal
 in the Software without restriction, including without limitation the rights
 to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 copies of the Software, and to permit persons to whom the Software is
 furnished to do so, subject to the following conditions:

 The above copyright notice and this permission notice shall be included in all
 copies or substantial portions of the Software.

 THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 SOFTWARE.
*/

package main

import (
	"context"
	"log"
	"net"
	"os"
	"sync"
	"syscall"

	"github.com/globalsign/mgo"
	"github.com/spf13/viper"

	"os/signal"

	"github.com/spf13/cobra"
)

var configFlag string
var listenFlag string
var mongoFlag string

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run bro API server",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())

		listenCfg := listenFlag
		mongoCfg := mongoFlag

		if configFlag != "" {
			initConfig()

			listenCfg = viper.GetString("listen")
			mongoCfg = viper.GetString("mongo")
		}

		listener, err := net.Listen("tcp", listenCfg)

		if err != nil {
			log.Fatalf("Unable to start listener: %s", err)
		}

		// DB
		info, err := mgo.ParseURL(mongoCfg)

		if err != nil {
			log.Fatalf("Invalid mongo url %s: %s", mongoCfg, err)
		}

		log.Printf("Connecting to mongo at %s", mongoCfg)

		db, err := mgo.Dial(mongoCfg)

		if err != nil {
			log.Fatalf("Error connecting to Mongo: %s", err)
		}

		srv := NewServer(listener, db, info.Database, ctx)

		wg := &sync.WaitGroup{}
		wg.Add(1)

		go srv.Run(wg)

		wait(ctx, cancel, wg)
	},
}

func initConfig() {
	file, err := os.Open(configFlag)

	if err != nil {
		log.Fatalf("Error reading config: %s", err)
	}

	log.Printf("Using configuration file: %s", configFlag)

	viper.SetConfigType("toml")

	err = viper.ReadConfig(file)

	if err != nil {
		log.Fatalf("Error parsing config: %s", err)
	}
}

func wait(ctx context.Context, cancel func(), wg *sync.WaitGroup) {
	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt)
	signal.Notify(c,
		syscall.SIGTERM,
		syscall.SIGABRT,
		syscall.SIGPIPE,
		syscall.SIGBUS,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGQUIT)

LOOP:
	for {
		select {
		case <-ctx.Done():
			break
		case sig := <-c:
			switch sig {
			case os.Interrupt, syscall.SIGTERM, syscall.SIGABRT,
				syscall.SIGPIPE, syscall.SIGBUS, syscall.SIGUSR1, syscall.SIGUSR2,
				syscall.SIGQUIT:
				cancel()

				break LOOP
			}
		}
	}

	wg.Wait()
}

func init() {
	runCmd.Flags().StringVarP(&configFlag, "config", "c", "",
		"Path to configuration file")
	runCmd.Flags().StringVarP(&listenFlag, "listen", "l", ":9999",
		"Listen address")
	runCmd.Flags().StringVarP(&mongoFlag, "db", "d", "localhost/bro",
		"Mongo host/database")
}
